// Command storyblok-schema validates, plans, and applies component schemas.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/mapi"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, env func(string) string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: storyblok-schema validate | plan --space ID [--out schema-plan.json] | apply --space ID --plan schema-plan.json")
	}
	command := args[0]
	if command != "validate" && command != "plan" && command != "apply" {
		return fmt.Errorf("unknown command %q", command)
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	space := flags.String("space", env("STORYBLOK_SPACE"), "target space ID")
	apiURL := flags.String("api-url", cmp.Or(env("STORYBLOK_MAPI_URL"), mapi.DefaultURL), "regional Management API base URL")
	out := flags.String("out", "", "output file (validate: stdout; plan: schema-plan.json)")
	planFile := flags.String("plan", "", "reviewed plan to apply")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if command != "apply" && *planFile != "" {
		return errors.New("--plan is only valid for apply")
	}
	if command == "apply" && *out != "" {
		return errors.New("--out is not valid for apply")
	}
	desired, err := components.Schemas()
	if err != nil {
		return err
	}
	// Constructing the renderer also verifies template parsing offline.
	if _, err := components.NewRenderer(); err != nil {
		return err
	}
	if command == "validate" {
		fmt.Fprintf(stderr, "Validated %d CMS component schemas.\n", len(desired))
		return writeJSON(*out, desired, stdout)
	}
	client, err := mapi.NewClient(*apiURL, *space, env("STORYBLOK_TOKEN"))
	if err != nil {
		return err
	}
	if command == "plan" {
		remote, err := client.List(ctx)
		if err != nil {
			return err
		}
		plan, err := mapi.BuildPlan(client.BaseURL, client.Space, desired, remote)
		if err != nil {
			return err
		}
		printPlan(stderr, plan)
		path := cmp.Or(*out, "schema-plan.json")
		if err := writeJSON(path, plan, stdout); err != nil {
			return err
		}
		fmt.Fprintln(stderr, "Plan written to", path)
		return nil
	}
	if *planFile == "" {
		return errors.New("apply requires --plan; run plan first")
	}
	data, err := os.ReadFile(*planFile)
	if err != nil {
		return err
	}
	var plan mapi.Plan
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("plan must contain exactly one JSON object")
	}
	printPlan(stdout, plan)
	if err := client.Apply(ctx, plan, desired); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Applied and verified. Story content was not migrated; remote-only components were not deleted.")
	return nil
}

func writeJSON(path string, value any, stdout io.Writer) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "" || path == "-" {
		_, err = stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func printPlan(w io.Writer, p mapi.Plan) {
	fmt.Fprintf(w, "Space %s (%s): %d component changes\n", p.Space, p.BaseURL, len(p.Changes))
	for _, change := range p.Changes {
		fmt.Fprintf(w, "%s %s\n", strings.ToUpper(change.Action), change.After.Name)
		if change.Before != nil {
			before, _ := json.Marshal(change.Before)
			after, _ := json.Marshal(change.After)
			var a, b map[string]any
			_ = json.Unmarshal(before, &a)
			_ = json.Unmarshal(after, &b)
			printDiff(w, "", a, b)
		} else {
			names := make([]string, 0, len(change.After.Schema))
			for name := range change.After.Schema {
				names = append(names, name)
			}
			slices.Sort(names)
			for _, name := range names {
				fmt.Fprintf(w, "  + %s (%s)\n", name, change.After.Schema[name]["type"])
			}
		}
	}
	for _, warning := range p.Warnings {
		fmt.Fprintln(w, "MIGRATION CHECK:", warning)
	}
}

func printDiff(w io.Writer, path string, a, b map[string]any) {
	keys := []string{}
	for key := range a {
		keys = append(keys, key)
	}
	for key := range b {
		if _, ok := a[key]; !ok {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		name := path + key
		x, y := a[key], b[key]
		if am, ok := x.(map[string]any); ok {
			if bm, ok := y.(map[string]any); ok {
				printDiff(w, name+".", am, bm)
				continue
			}
		}
		xj, _ := json.Marshal(x)
		yj, _ := json.Marshal(y)
		if string(xj) != string(yj) {
			fmt.Fprintf(w, "  %s: %s -> %s\n", name, xj, yj)
		}
	}
}
