// Package storyblok fetches stories from the Storyblok Content Delivery API and
// provides the field types used to decode story content.
package storyblok

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/failsafe-go/failsafe-go"
)

const DefaultBaseURL = "https://api.storyblok.com/v2/cdn"

var ErrNotFound = errors.New("storyblok: story not found")

type Version string

const (
	Draft     Version = "draft"
	Published Version = "published"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	executor   failsafe.Executor[*http.Response]
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: requestTimeout},
		executor:   newHTTPExecutor(),
	}
}

type StoryOptions struct {
	Version Version
	// ResolveRelations lists relation fields as "component.field". Resolved
	// stories replace the UUIDs in the returned content.
	ResolveRelations []string
}

// Story returns the raw JSON of the story at slug.
func (c *Client) Story(ctx context.Context, slug string, opts StoryOptions) (jsontext.Value, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	query := url.Values{"token": {c.token}, "version": {string(opts.Version)}}
	if len(opts.ResolveRelations) > 0 {
		query.Set("resolve_relations", strings.Join(opts.ResolveRelations, ","))
	}
	endpoint := c.baseURL + "/stories/" + url.PathEscape(slug) + "?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.do(req)
	if err != nil {
		// The request URL carries the access token, so it must not end up in logs.
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("storyblok: fetching story %q: %w", slug, err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("storyblok: fetching story %q: unexpected status %s", slug, res.Status)
	}

	var payload struct {
		Story jsontext.Value   `json:"story"`
		Rels  []map[string]any `json:"rels"`
	}
	if err := json.UnmarshalRead(res.Body, &payload); err != nil {
		return nil, fmt.Errorf("storyblok: decoding story %q: %w", slug, err)
	}
	if len(payload.Rels) == 0 {
		return payload.Story, nil
	}
	return inlineRelations(payload.Story, payload.Rels, opts.ResolveRelations)
}

// inlineRelations replaces relation UUIDs with the stories the API returned
// separately in "rels", matching the shape the Visual Editor bridge sends.
func inlineRelations(story jsontext.Value, rels []map[string]any, fields []string) (jsontext.Value, error) {
	byUUID := make(map[string]any, len(rels))
	for _, rel := range rels {
		if uuid, ok := rel["uuid"].(string); ok {
			byUUID[uuid] = rel
		}
	}
	fieldsByComponent := map[string][]string{}
	for _, f := range fields {
		component, field, ok := strings.Cut(f, ".")
		if ok {
			fieldsByComponent[component] = append(fieldsByComponent[component], field)
		}
	}

	var tree any
	if err := json.Unmarshal(story, &tree); err != nil {
		return nil, err
	}
	walkBloks(tree, func(blok map[string]any) {
		component, _ := blok["component"].(string)
		for _, field := range fieldsByComponent[component] {
			switch v := blok[field].(type) {
			case string:
				if rel, ok := byUUID[v]; ok {
					blok[field] = rel
				}
			case []any:
				for i, item := range v {
					if uuid, ok := item.(string); ok {
						if rel, ok := byUUID[uuid]; ok {
							v[i] = rel
						}
					}
				}
			}
		}
	})
	return json.Marshal(tree)
}

func walkBloks(node any, visit func(map[string]any)) {
	switch n := node.(type) {
	case map[string]any:
		if _, ok := n["component"]; ok {
			visit(n)
		}
		for _, child := range n {
			walkBloks(child, visit)
		}
	case []any:
		for _, child := range n {
			walkBloks(child, visit)
		}
	}
}

// SpaceID returns the ID of the space the access token belongs to.
func (c *Client) SpaceID(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := c.baseURL + "/spaces/me?" + url.Values{"token": {c.token}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	res, err := c.do(req)
	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			err = urlErr.Err
		}
		return 0, fmt.Errorf("storyblok: fetching space: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("storyblok: fetching space: unexpected status %s", res.Status)
	}
	var payload struct {
		Space struct {
			ID int64 `json:"id"`
		} `json:"space"`
	}
	if err := json.UnmarshalRead(res.Body, &payload); err != nil {
		return 0, fmt.Errorf("storyblok: decoding space: %w", err)
	}
	return payload.Space.ID, nil
}
