package storyblok_test

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"strconv"
	"testing"
	"time"

	"storyblok-go-website/internal/storyblok"
)

const previewToken = "preview-token"

func previewQuery(spaceID, token string, issued time.Time) url.Values {
	ts := strconv.FormatInt(issued.Unix(), 10)
	sum := sha1.Sum([]byte(spaceID + ":" + token + ":" + ts))
	return url.Values{
		"_storyblok_tk[space_id]":  {spaceID},
		"_storyblok_tk[timestamp]": {ts},
		"_storyblok_tk[token]":     {hex.EncodeToString(sum[:])},
	}
}

func TestIsValidPreview(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		query url.Values
		want  bool
	}{
		{"signed by the editor", previewQuery("42", previewToken, now.Add(-time.Minute)), true},
		{"signed with another token", previewQuery("42", "other-token", now), false},
		{"expired", previewQuery("42", previewToken, now.Add(-storyblok.PreviewTokenMaxAge-time.Second)), false},
		{"missing", url.Values{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := storyblok.IsValidPreview(tt.query, previewToken, now); got != tt.want {
				t.Errorf("IsValidPreview() = %v, want %v", got, tt.want)
			}
		})
	}
}
