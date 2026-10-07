package mapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Story is the writable part of a Management API story.
type Story struct {
	ID       int64          `json:"id,omitempty"`
	Name     string         `json:"name"`
	Slug     string         `json:"slug"`
	FullSlug string         `json:"full_slug,omitempty"`
	ParentID int64          `json:"parent_id"`
	IsFolder bool           `json:"is_folder,omitempty"`
	Content  map[string]any `json:"content,omitempty"`
}

// FindStory returns the story or folder at fullSlug, or nil if there is none.
func (c *Client) FindStory(ctx context.Context, fullSlug string) (*Story, error) {
	var envelope struct {
		Stories []Story `json:"stories"`
	}
	query := url.Values{"with_slug": {fullSlug}}
	if _, err := c.request(ctx, http.MethodGet, "/stories?"+query.Encode(), nil, &envelope); err != nil {
		return nil, err
	}
	for _, story := range envelope.Stories {
		if story.FullSlug == fullSlug {
			return &story, nil
		}
	}
	return nil, nil
}

// SaveStory creates the story, or replaces it if it has an ID. Stories are
// published; folders have nothing to publish.
func (c *Client) SaveStory(ctx context.Context, story Story) (Story, error) {
	method, path := http.MethodPost, "/stories"
	if story.ID != 0 {
		method, path = http.MethodPut, "/stories/"+strconv.FormatInt(story.ID, 10)
	}
	body := map[string]any{"story": story}
	if !story.IsFolder {
		body["publish"] = 1
	}
	var envelope struct {
		Story json.RawMessage `json:"story"`
	}
	if _, err := c.request(ctx, method, path, body, &envelope); err != nil {
		return Story{}, err
	}
	var saved Story
	if err := json.Unmarshal(envelope.Story, &saved); err != nil || saved.ID <= 0 {
		return Story{}, fmt.Errorf("API response omitted the story ID")
	}
	return saved, nil
}
