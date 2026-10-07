package mapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"storyblok-go-website/internal/schema"
)

func desiredComponents() []schema.Component {
	return []schema.Component{
		{Name: "block-section-intro", DisplayName: "Intro", IsNestable: true, Schema: map[string]map[string]any{"heading": {"type": "text", "pos": 0, "required": true}}},
		{Name: "page-landing-page", DisplayName: "Landing", IsRoot: true, Schema: map[string]map[string]any{"sections": {"type": "bloks", "pos": 0, "restrict_components": true, "component_whitelist": []string{"block-section-intro"}}}},
	}
}

func asRemote(c schema.Component, id int64) Remote {
	data, _ := json.Marshal(c)
	var r Remote
	_ = json.Unmarshal(data, &r)
	r.ID = id
	return r
}

func TestPlanIgnoresServerDefaultsAndFieldIDs(t *testing.T) {
	desired := desiredComponents()
	remote := []Remote{asRemote(desired[1], 2), asRemote(desired[0], 1)}
	for _, r := range remote {
		for _, f := range r.Schema {
			f["id"] = "server-id"
			f["translatable"] = false
			f["description"] = nil
			f["display_name"] = ""
		}
	}
	p, err := BuildPlan(DefaultURL, "123", desired, remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 0 || len(p.Warnings) != 0 {
		t.Fatalf("not idempotent: %#v", p)
	}
}

func TestMigrationWarnings(t *testing.T) {
	before := schema.Component{Name: "page-test", IsRoot: true, Schema: map[string]map[string]any{
		"removed": {"type": "text"}, "type": {"type": "text"}, "required": {"type": "text"}, "children": {"type": "bloks", "component_whitelist": []string{"old"}}, "label": {"type": "text", "display_name": "Old"},
	}}
	after := schema.Component{Name: "page-test", IsRoot: true, Schema: map[string]map[string]any{
		"type": {"type": "textarea"}, "required": {"type": "text", "required": true}, "new": {"type": "text", "required": true}, "children": {"type": "bloks", "component_whitelist": []string{"new"}}, "label": {"type": "text", "display_name": "New"},
	}}
	p, err := BuildPlan(DefaultURL, "123", []schema.Component{after}, []Remote{asRemote(before, 1), {ID: 2, Name: "legacy"}})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(p.Warnings, "\n")
	for _, want := range []string{"field removed", "type changed", "required field added", "page-test.required", "page-test.children", "legacy exists only remotely"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %s in %s", want, joined)
		}
	}
	if strings.Contains(joined, "page-test.label") {
		t.Fatal("label-only change flagged as migration")
	}
}

// apiFixture validates the actual wire contract, retaining remote editor metadata.
type apiFixture struct {
	items     map[int64]Remote
	writes    int
	failWrite int
	nextID    int64
}

func newAPI(t *testing.T) (*Client, *apiFixture) {
	t.Helper()
	f := &apiFixture{items: map[int64]Remote{}, nextID: 100}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "test-token" {
			t.Error("missing auth")
			w.WriteHeader(401)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/spaces/123/components") {
			t.Errorf("wrong path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			items := []Remote{}
			for _, item := range f.items {
				items = append(items, item)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"components": items})
			return
		}
		f.writes++
		if f.failWrite == f.writes {
			w.WriteHeader(500)
			return
		}
		var body struct {
			Component map[string]json.RawMessage `json:"component"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body.Component["icon"]; ok {
			t.Error("overwriting unmanaged editor metadata")
		}
		data, _ := json.Marshal(body.Component)
		var item Remote
		_ = json.Unmarshal(data, &item)
		switch r.Method {
		case "POST":
			f.nextID++
			item.ID = f.nextID
		case "PUT":
			id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/v1/spaces/123/components/"), 10, 64)
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			item.ID = id
			if _, ok := f.items[id]; !ok {
				w.WriteHeader(404)
				return
			}
		default:
			t.Errorf("unexpected mutation %s", r.Method)
			w.WriteHeader(405)
			return
		}
		for name, field := range item.Schema {
			if field["id"] == nil {
				field["id"] = fmt.Sprintf("%d-%s", item.ID, name)
			}
			if allowed, ok := field["component_whitelist"].([]any); ok {
				for _, target := range allowed {
					found := false
					for _, existing := range f.items {
						if existing.Name == target {
							found = true
						}
					}
					if !found {
						t.Errorf("unresolved reference %s", target)
						w.WriteHeader(422)
						return
					}
				}
			}
		}
		f.items[item.ID] = item
		_ = json.NewEncoder(w).Encode(map[string]any{"component": item})
	}))
	t.Cleanup(ts.Close)
	c, err := NewClient(ts.URL+"/v1", "123", "test-token")
	if err != nil {
		t.Fatal(err)
	}
	return c, f
}

func planFor(t *testing.T, c *Client, desired []schema.Component) Plan {
	t.Helper()
	remote, err := c.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	p, err := BuildPlan(c.BaseURL, c.Space, desired, remote)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestApplyCreatesDependenciesAndConverges(t *testing.T) {
	c, f := newAPI(t)
	desired := desiredComponents()
	f.items[9] = Remote{ID: 9, Name: "legacy", Schema: map[string]map[string]any{}}
	p := planFor(t, c, desired)
	if err := c.Apply(t.Context(), p, desired); err != nil {
		t.Fatal(err)
	}
	if f.writes != 4 || len(f.items) != 3 {
		t.Fatalf("writes=%d items=%d", f.writes, len(f.items))
	}
	p = planFor(t, c, desired)
	if len(p.Changes) != 0 {
		t.Fatal("second plan not empty")
	}
	if err := c.Apply(t.Context(), p, desired); err != nil {
		t.Fatal(err)
	}
	if f.writes != 4 {
		t.Fatal("no-op apply wrote components")
	}
}

func TestApplyPreservesFieldIDs(t *testing.T) {
	c, f := newAPI(t)
	desired := desiredComponents()[:1]
	r := asRemote(desired[0], 10)
	r.Schema["heading"]["id"] = "stable-field"
	r.Schema["heading"]["display_name"] = "Old"
	f.items[10] = r
	p := planFor(t, c, desired)
	if err := c.Apply(t.Context(), p, desired); err != nil {
		t.Fatal(err)
	}
	if f.items[10].Schema["heading"]["id"] != "stable-field" {
		t.Fatal("field ID lost")
	}
}

func TestApplyRejectsStaleAndModifiedPlansBeforeWrites(t *testing.T) {
	for _, kind := range []string{"remote", "code", "plan", "target"} {
		t.Run(kind, func(t *testing.T) {
			c, f := newAPI(t)
			desired := desiredComponents()
			p := planFor(t, c, desired)
			switch kind {
			case "remote":
				f.items[10] = Remote{ID: 10, Name: "external"}
			case "code":
				desired[0].DisplayName = "Changed"
			case "plan":
				p.Changes = nil
			case "target":
				p.Space = "999"
			}
			if err := c.Apply(t.Context(), p, desired); err == nil {
				t.Fatal("stale plan accepted")
			}
			if f.writes != 0 {
				t.Fatal("wrote before validating plan")
			}
		})
	}
}

func TestPartialApplyCanBeReplanned(t *testing.T) {
	c, f := newAPI(t)
	desired := desiredComponents()
	p := planFor(t, c, desired)
	f.failWrite = 3
	if err := c.Apply(t.Context(), p, desired); err == nil {
		t.Fatal("expected failed update")
	}
	if f.writes != 3 {
		t.Fatal("unexpected retry")
	}
	p = planFor(t, c, desired)
	if err := c.Apply(t.Context(), p, desired); err != nil {
		t.Fatal(err)
	}
	if len(f.items) != 2 {
		t.Fatal("duplicated components during recovery")
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c, _ := newAPI(t)
	if _, err := c.List(ctx); err == nil {
		t.Fatal("ignored cancellation")
	}
}
