package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/mapi"
	"storyblok-go-website/internal/schema"
)

func registeredComponents(t *testing.T) int {
	t.Helper()
	all, err := components.Schemas()
	if err != nil {
		t.Fatal(err)
	}
	return len(all)
}

func TestValidateNeedsNoCredentialsAndEmitsDeterministicJSON(t *testing.T) {
	var first bytes.Buffer
	for i := 0; i < 2; i++ {
		var out, log bytes.Buffer
		if err := run(t.Context(), []string{"validate"}, func(string) string { return "" }, &out, &log); err != nil {
			t.Fatal(err)
		}
		var validated []schema.Component
		if err := json.Unmarshal(out.Bytes(), &validated); err != nil {
			t.Fatal(err)
		}
		if want := registeredComponents(t); len(validated) != want {
			t.Fatalf("got %d, want %d", len(validated), want)
		}
		if i == 0 {
			first.Write(out.Bytes())
		} else if !bytes.Equal(first.Bytes(), out.Bytes()) {
			t.Fatal("nondeterministic schema output")
		}
	}
}

func TestPlanIsReadOnlyAndShowsMigrationWarnings(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("plan performed a write")
			w.WriteHeader(405)
			return
		}
		_, _ = w.Write([]byte(`{"components":[{"id":1,"name":"enterprise_page","schema":{}}]}`))
	}))
	defer ts.Close()
	var out, log bytes.Buffer
	path := filepath.Join(t.TempDir(), "plan.json")
	err := run(t.Context(), []string{"plan", "--space", "123", "--api-url", ts.URL, "--out", path}, func(key string) string {
		if key == "STORYBLOK_TOKEN" {
			return "token"
		}
		return ""
	}, &out, &log)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p mapi.Plan
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != registeredComponents(t) || !strings.Contains(log.String(), "MIGRATION CHECK: enterprise_page") {
		t.Fatalf("plan=%#v log=%s", p, log.String())
	}
}

func TestCLIRejectsUnknownCommandsAndArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"delete"}, {"validate", "surprise"}, {"validate", "--plan", "bad"}} {
		var out bytes.Buffer
		if err := run(t.Context(), args, func(string) string { return "" }, &out, &out); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestPlanDiffShowsOnlyMeaningfulChanges(t *testing.T) {
	before := &schema.Component{Name: "block-section-x", Schema: map[string]map[string]any{
		"heading": {"type": "text", "display_name": "Old", "description": nil, "required": nil, "id": "server-id"},
	}}
	after := schema.Component{Name: "block-section-x", Schema: map[string]map[string]any{
		"heading": {"type": "text", "display_name": "New", "description": "", "required": false},
	}}
	var out bytes.Buffer
	printPlan(&out, mapi.Plan{Changes: []mapi.Change{{Action: "update", Before: before, After: after}}})
	if got := out.String(); !strings.Contains(got, `schema.heading.display_name: "Old" -> "New"`) ||
		strings.Contains(got, "description") || strings.Contains(got, "required") || strings.Contains(got, "id:") {
		t.Errorf("diff:\n%s", got)
	}
}
