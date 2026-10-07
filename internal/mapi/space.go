package mapi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type SpaceSettings struct {
	// Languages are the space's languages besides the default one.
	Languages []Language `json:"languages"`
	// PublishesLanguages means each field-level translation is published on
	// its own; publishing a story only publishes the default language.
	PublishesLanguages bool `json:"use_translated_stories"`
}

func (c *Client) Settings(ctx context.Context) (SpaceSettings, error) {
	var envelope struct {
		Space SpaceSettings `json:"space"`
	}
	_, err := c.request(ctx, http.MethodGet, "", nil, &envelope)
	return envelope.Space, err
}

// SetLanguages replaces the space's languages besides the default one.
func (c *Client) SetLanguages(ctx context.Context, languages []Language) error {
	var envelope map[string]any
	_, err := c.request(ctx, http.MethodPut, "", map[string]any{"space": map[string]any{"languages": languages}}, &envelope)
	return err
}

// PublishLanguage publishes one field-level translation of a story, for
// spaces that publish languages separately.
func (c *Client) PublishLanguage(ctx context.Context, storyID int64, lang string) error {
	var envelope map[string]any
	query := url.Values{"lang": {lang}}
	_, err := c.request(ctx, http.MethodGet, "/stories/"+strconv.FormatInt(storyID, 10)+"/publish?"+query.Encode(), nil, &envelope)
	return err
}
