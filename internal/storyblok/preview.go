package storyblok

import (
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"net/url"
	"strconv"
	"time"
)

// PreviewTokenMaxAge bounds how long a Visual Editor preview URL stays valid.
// The editor only refreshes the token when it reloads the preview iframe, so
// it must outlast a typical editing session.
const PreviewTokenMaxAge = 12 * time.Hour

// IsValidPreview reports whether query carries the signed _storyblok_tk
// parameters the Visual Editor appends to preview URLs.
func IsValidPreview(query url.Values, previewToken string, now time.Time) bool {
	spaceID := query.Get("_storyblok_tk[space_id]")
	timestamp := query.Get("_storyblok_tk[timestamp]")
	token := query.Get("_storyblok_tk[token]")
	if spaceID == "" || timestamp == "" || token == "" {
		return false
	}

	sum := sha1.Sum([]byte(spaceID + ":" + previewToken + ":" + timestamp))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(token)) != 1 {
		return false
	}

	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	age := now.Sub(time.Unix(seconds, 0))
	return age >= -time.Minute && age <= PreviewTokenMaxAge
}
