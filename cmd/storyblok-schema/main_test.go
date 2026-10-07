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

	"storyblok-go-website/internal/mapi"
	"storyblok-go-website/internal/schema"
)

func TestValidateNeedsNoCredentialsAndEmitsDeterministicJSON(t *testing.T) {
	var first bytes.Buffer
	for i := 0; i < 2; i++ {
		var out, log bytes.Buffer
		if err := run(t.Context(), []string{"validate"}, func(string) string { return "" }, &out, &log); err != nil {
			t.Fatal(err)
		}
		var components []schema.Component
		if err := json.Unmarshal(out.Bytes(), &components); err != nil {
			t.Fatal(err)
		}
		if len(components) != 2 {
			t.Fatalf("got %d", len(components))
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
	if len(p.Changes) != 2 || !strings.Contains(log.String(), "MIGRATION CHECK: enterprise_page") {
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
