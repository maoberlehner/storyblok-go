package mapi

import (
	"context"
	"net/http"
)

type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Languages lists the space's languages besides the default one.
func (c *Client) Languages(ctx context.Context) ([]Language, error) {
	var envelope struct {
		Space struct {
			Languages []Language `json:"languages"`
		} `json:"space"`
	}
	_, err := c.request(ctx, http.MethodGet, "", nil, &envelope)
	return envelope.Space.Languages, err
}

// SetLanguages replaces the space's languages besides the default one.
func (c *Client) SetLanguages(ctx context.Context, languages []Language) error {
	var envelope map[string]any
	_, err := c.request(ctx, http.MethodPut, "", map[string]any{"space": map[string]any{"languages": languages}}, &envelope)
	return err
}
