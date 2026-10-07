// Command storyblok-seed creates or updates the demo content: the stories in
// the seed directory and generated articles. Folders are created as needed.
// Seeding again overwrites these stories and leaves all others untouched.
//
// Apply the component schemas first, so the content validates.
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
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"storyblok-go-website/internal/mapi"
)

type seedStory struct {
	FullSlug string         `json:"full_slug"`
	Name     string         `json:"name"`
	Content  map[string]any `json:"content"`
	// AlternateOf names the story this one translates at folder level.
	AlternateOf string `json:"alternate_of"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, env func(string) string, stdout io.Writer) error {
	flags := flag.NewFlagSet("storyblok-seed", flag.ContinueOnError)
	space := flags.String("space", env("STORYBLOK_SPACE"), "target space ID")
	apiURL := flags.String("api-url", cmp.Or(env("STORYBLOK_MAPI_URL"), mapi.DefaultURL), "regional Management API base URL")
	dir := flags.String("dir", "seed", "directory with story JSON files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	files, err := readStories(*dir)
	if err != nil {
		return err
	}
	client, err := mapi.NewClient(*apiURL, *space, env("STORYBLOK_TOKEN"))
	if err != nil {
		return err
	}
	if err := ensureLanguage(ctx, client, "de", "German", stdout); err != nil {
		return err
	}
	// Articles go first, so the landing pages list them right away.
	// Alternates go last, after the stories they translate.
	stories := append(articles(), files...)
	slices.SortStableFunc(stories, func(a, b seedStory) int {
		return cmp.Compare(boolInt(a.AlternateOf != ""), boolInt(b.AlternateOf != ""))
	})
	folders := map[string]int64{"": 0}
	uploaded := map[string]mapi.Asset{}
	for _, story := range stories {
		parentID, err := ensureFolder(ctx, client, folders, path.Dir(story.FullSlug))
		if err != nil {
			return err
		}
		content, err := resolveSeedImages(ctx, client, uploaded, story.Content)
		if err != nil {
			return err
		}
		groupID := ""
		if story.AlternateOf != "" {
			original, err := client.FindStory(ctx, story.AlternateOf)
			if err != nil {
				return err
			}
			if original == nil {
				return fmt.Errorf("%s: alternate_of %s does not exist", story.FullSlug, story.AlternateOf)
			}
			groupID = original.GroupID
		}
		if err := saveStory(ctx, client, mapi.Story{
			Name:     story.Name,
			Slug:     path.Base(story.FullSlug),
			FullSlug: story.FullSlug,
			ParentID: parentID,
			Content:  content.(map[string]any),
			GroupID:  groupID,
		}, stdout); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "Seeded %d stories.\n", len(stories))
	return nil
}

func readStories(dir string) ([]seedStory, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no story files in %s", dir)
	}
	slices.Sort(paths)
	var stories []seedStory
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var story seedStory
		if err := json.Unmarshal(data, &story); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if story.FullSlug == "" || story.Name == "" || story.Content["component"] == nil {
			return nil, fmt.Errorf("%s: full_slug, name, and content.component are required", p)
		}
		stories = append(stories, story)
	}
	return stories, nil
}

// ensureFolder returns the ID of the folder at fullSlug, creating it and its
// parents if needed. "." is the space root.
func ensureFolder(ctx context.Context, client *mapi.Client, known map[string]int64, fullSlug string) (int64, error) {
	if fullSlug == "." {
		fullSlug = ""
	}
	if id, ok := known[fullSlug]; ok {
		return id, nil
	}
	parentID, err := ensureFolder(ctx, client, known, path.Dir(fullSlug))
	if err != nil {
		return 0, err
	}
	folder, err := client.FindStory(ctx, fullSlug)
	if err != nil {
		return 0, err
	}
	if folder == nil {
		name := path.Base(fullSlug)
		created, err := client.SaveStory(ctx, mapi.Story{Name: capitalize(name), Slug: name, ParentID: parentID, IsFolder: true})
		if err != nil {
			return 0, fmt.Errorf("creating folder %s: %w", fullSlug, err)
		}
		folder = &created
	} else if !folder.IsFolder {
		return 0, fmt.Errorf("%s is a story, not a folder", fullSlug)
	}
	known[fullSlug] = folder.ID
	return folder.ID, nil
}

func saveStory(ctx context.Context, client *mapi.Client, story mapi.Story, stdout io.Writer) error {
	existing, err := client.FindStory(ctx, story.FullSlug)
	if err != nil {
		return err
	}
	action := "created"
	if existing != nil {
		if existing.IsFolder {
			return errors.New(story.FullSlug + " is a folder, not a story")
		}
		story.ID, action = existing.ID, "updated"
	}
	if _, err := client.SaveStory(ctx, story); err != nil {
		return fmt.Errorf("saving %s: %w", story.FullSlug, err)
	}
	fmt.Fprintf(stdout, "%s %s\n", action, story.FullSlug)
	return nil
}

// ensureLanguage adds the language to the space unless it has it, keeping
// the others.
func ensureLanguage(ctx context.Context, client *mapi.Client, code, name string, stdout io.Writer) error {
	languages, err := client.Languages(ctx)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(languages, func(l mapi.Language) bool { return l.Code == code }) {
		return nil
	}
	if err := client.SetLanguages(ctx, append(languages, mapi.Language{Code: code, Name: name})); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "added language %s\n", code)
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
