package mapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	"storyblok-go-website/internal/schema"
)

type Change struct {
	Action string            `json:"action"`
	ID     int64             `json:"id,omitempty"`
	Before *schema.Component `json:"before,omitempty"`
	After  schema.Component  `json:"after"`
}

type Plan struct {
	Version     int      `json:"version"`
	BaseURL     string   `json:"base_url"`
	Space       string   `json:"space"`
	DesiredHash string   `json:"desired_hash"`
	RemoteHash  string   `json:"remote_hash"`
	Changes     []Change `json:"changes"`
	Warnings    []string `json:"warnings"`
}

func component(r Remote) schema.Component {
	return schema.Component{Name: r.Name, DisplayName: r.DisplayName, IsRoot: r.IsRoot, IsNestable: r.IsNestable, Schema: r.Schema}
}

// normalized removes API-supplied field IDs and empty defaults for comparison.
// Nonempty unknown field properties remain visible in the diff: schemas are
// fully code-owned, while top-level editor metadata is never sent in updates.
func normalized(c schema.Component) schema.Component {
	data, _ := json.Marshal(c)
	var result schema.Component
	_ = json.Unmarshal(data, &result)
	if result.Schema == nil {
		result.Schema = map[string]map[string]any{}
	}
	for _, field := range result.Schema {
		delete(field, "id")
		for key, value := range field {
			if value == nil || reflect.DeepEqual(value, false) || reflect.DeepEqual(value, "") {
				delete(field, key)
				continue
			}
			if array, ok := value.([]any); ok && len(array) == 0 {
				delete(field, key)
			}
		}
		if field["type"] == "bloks" {
			if _, ok := field["component_whitelist"].([]any); ok {
				values := field["component_whitelist"].([]any)
				slices.SortFunc(values, func(a, b any) int { return compare(fmt.Sprint(a), fmt.Sprint(b)) })
			}
		}
	}
	return result
}

func compare(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func hash(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func BuildPlan(baseURL, space string, desired []schema.Component, remote []Remote) (Plan, error) {
	desired = slices.Clone(desired)
	slices.SortFunc(desired, func(a, b schema.Component) int { return compare(a.Name, b.Name) })
	remote = slices.Clone(remote)
	slices.SortFunc(remote, func(a, b Remote) int { return compare(a.Name, b.Name) })
	p := Plan{Version: 1, BaseURL: baseURL, Space: space, DesiredHash: hash(desired), Changes: []Change{}, Warnings: []string{}}
	byName := map[string]Remote{}
	for _, r := range remote {
		if _, exists := byName[r.Name]; exists {
			return Plan{}, fmt.Errorf("duplicate remote component %s", r.Name)
		}
		byName[r.Name] = r
	}
	state := []Remote{}
	for _, r := range remote {
		n := normalized(component(r))
		r.Schema = n.Schema
		state = append(state, r)
	}
	p.RemoteHash = hash(state)
	seen := map[string]bool{}
	for _, d := range desired {
		if seen[d.Name] {
			return Plan{}, fmt.Errorf("duplicate desired component %s", d.Name)
		}
		seen[d.Name] = true
		r, exists := byName[d.Name]
		if !exists {
			p.Changes = append(p.Changes, Change{Action: "create", After: d})
			continue
		}
		before, after := normalized(component(r)), normalized(d)
		if hash(before) == hash(after) {
			continue
		}
		p.Changes = append(p.Changes, Change{Action: "update", ID: r.ID, Before: &before, After: d})
		p.Warnings = append(p.Warnings, migrationWarnings(before, after)...)
	}
	for _, r := range remote {
		if !seen[r.Name] {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s exists only remotely and will be left untouched; stories using it may need a one-off migration to a supported component", r.Name))
		}
	}
	slices.Sort(p.Warnings)
	return p, nil
}

func migrationWarnings(before, after schema.Component) []string {
	var warnings []string
	add := func(field, reason string) {
		warnings = append(warnings, fmt.Sprintf("%s.%s: %s; a one-off content migration may be necessary", after.Name, field, reason))
	}
	if before.IsRoot != after.IsRoot || before.IsNestable != after.IsNestable {
		add("(component)", "root/nestable role changed")
	}
	for name, old := range before.Schema {
		new, exists := after.Schema[name]
		if !exists {
			add(name, "field removed (renames also require migrating stored keys)")
			continue
		}
		if old["type"] != new["type"] {
			add(name, "field type changed")
			continue
		}
		// Any non-editor-only change is conservatively reported, including
		// newly required fields, allowlists, option values, and validation.
		semantic := func(field map[string]any) map[string]any {
			out := map[string]any{}
			for key, value := range field {
				if key != "pos" && key != "display_name" && key != "description" {
					out[key] = value
				}
			}
			return out
		}
		if hash(semantic(old)) != hash(semantic(new)) {
			add(name, "validation, allowed values, or content settings changed")
		}
	}
	for name, field := range after.Schema {
		if _, exists := before.Schema[name]; !exists && field["required"] == true {
			add(name, "required field added; existing stories are not backfilled")
		}
	}
	return warnings
}

// Apply re-plans against live state and the current code before writing. The
// plan is an artifact, not an arbitrary source of API operations to execute.
func (c *Client) Apply(ctx context.Context, p Plan, desired []schema.Component) error {
	if p.Version != 1 || p.BaseURL != c.BaseURL || p.Space != c.Space {
		return fmt.Errorf("plan version or target mismatch; generate a new plan")
	}
	remote, err := c.List(ctx)
	if err != nil {
		return err
	}
	fresh, err := BuildPlan(c.BaseURL, c.Space, desired, remote)
	if err != nil {
		return err
	}
	if hash(fresh) != hash(p) {
		return fmt.Errorf("plan is stale or modified (code or remote schema changed); generate a new plan")
	}
	byName := map[string]Remote{}
	for _, r := range remote {
		byName[r.Name] = r
	}
	// Create shells first so references work even for mutually nested blocks.
	// Partial failures are recoverable by generating and applying a new plan.
	for _, change := range p.Changes {
		if change.Action != "create" {
			continue
		}
		shell := change.After
		shell.Schema = map[string]map[string]any{}
		created, err := c.Write(ctx, 0, shell)
		if err != nil {
			return fmt.Errorf("create %s: %w", shell.Name, err)
		}
		byName[shell.Name] = created
	}
	for _, change := range p.Changes {
		current := byName[change.After.Name]
		// Deep copy before restoring field IDs so plan data stays immutable.
		data, _ := json.Marshal(change.After)
		var payload schema.Component
		_ = json.Unmarshal(data, &payload)
		for name, field := range payload.Schema {
			if id, ok := current.Schema[name]["id"]; ok {
				field["id"] = id
			}
		}
		if _, err := c.Write(ctx, current.ID, payload); err != nil {
			return fmt.Errorf("update %s: %w", payload.Name, err)
		}
	}
	remote, err = c.List(ctx)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	remaining, err := BuildPlan(c.BaseURL, c.Space, desired, remote)
	if err != nil {
		return err
	}
	if len(remaining.Changes) != 0 {
		return fmt.Errorf("verification found schema drift after applying; generate a new plan")
	}
	return nil
}
