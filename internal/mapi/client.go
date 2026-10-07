// Package mapi implements the schema CLI's small Management API surface.
package mapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"storyblok-go-website/internal/apihttp"
)

const DefaultURL = "https://mapi.storyblok.com/v1"

// Paid plans allow 6 Management API requests per second. On plans with a
// lower limit, the first 429 halves the rate.
const (
	requestRate      = 6
	attemptTimeout   = 30 * time.Second
	maxResponseBytes = 16 << 20
)

type Remote struct {
	ID          int64                     `json:"id"`
	Name        string                    `json:"name"`
	DisplayName string                    `json:"display_name"`
	IsRoot      bool                      `json:"is_root"`
	IsNestable  bool                      `json:"is_nestable"`
	Schema      map[string]map[string]any `json:"schema"`
}

type Client struct {
	BaseURL string
	Space   string
	token   string
	api     *apihttp.Client
}

func NewClient(baseURL, space, token string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return nil, fmt.Errorf("API URL must be HTTPS (HTTP allowed only for localhost tests)")
	}
	id, err := strconv.ParseInt(space, 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("space must be a positive numeric ID")
	}
	if token == "" {
		return nil, fmt.Errorf("STORYBLOK_TOKEN is required")
	}
	// A burst of one keeps the client below the limit in any one-second window.
	pacer := apihttp.NewTier(apihttp.TierConfig{Base: requestRate, MaxBurst: 1})
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Space: strconv.FormatInt(id, 10), token: token,
		// Redirects are not followed, so the token never reaches another host.
		api: apihttp.NewClient(apihttp.Config{Pacer: pacer, AttemptTimeout: attemptTimeout}),
	}, nil
}

func (c *Client) List(ctx context.Context) ([]Remote, error) {
	result := []Remote{}
	seen := map[int64]bool{}
	for page := 1; ; page++ {
		var envelope struct {
			Components []Remote `json:"components"`
		}
		headers, err := c.request(ctx, http.MethodGet, fmt.Sprintf("?per_page=1000&page=%d", page), nil, &envelope)
		if err != nil {
			return nil, err
		}
		if envelope.Components == nil {
			return nil, fmt.Errorf("API response omitted the components array")
		}
		for _, component := range envelope.Components {
			if component.ID <= 0 || seen[component.ID] {
				return nil, fmt.Errorf("invalid or repeated component ID in API response")
			}
			seen[component.ID] = true
			result = append(result, component)
		}
		// This endpoint may return all components without pagination headers.
		total := headers.Get("Total")
		if total == "" {
			break
		}
		n, err := strconv.Atoi(total)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid Total header")
		}
		if len(result) >= n {
			break
		}
		if len(envelope.Components) == 0 {
			return nil, fmt.Errorf("incomplete paginated component response")
		}
	}
	return result, nil
}

func (c *Client) Write(ctx context.Context, id int64, component any) (Remote, error) {
	method, suffix := http.MethodPost, ""
	if id != 0 {
		method, suffix = http.MethodPut, "/"+strconv.FormatInt(id, 10)
	}
	var envelope struct {
		Component Remote `json:"component"`
	}
	_, err := c.request(ctx, method, suffix, map[string]any{"component": component}, &envelope)
	if err == nil && envelope.Component.ID <= 0 {
		err = fmt.Errorf("API response omitted the component ID; re-plan before retrying")
	}
	return envelope.Component, err
}

func (c *Client) request(ctx context.Context, method, suffix string, body, out any) (http.Header, error) {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(data)
	}
	endpoint := c.BaseURL + "/spaces/" + c.Space + "/components" + suffix
	req, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.api.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s components failed: %w; re-plan before retrying a write", method, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Don't print response bodies: they can contain reflected credentials.
		return nil, fmt.Errorf("%s components returned HTTP %d; re-plan after a failed write", method, response.StatusCode)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("invalid Management API response: %w", err)
	}
	return response.Header, nil
}
