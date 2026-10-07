package main

import "testing"

func TestParseSiteURL(t *testing.T) {
	for raw, want := range map[string]string{
		"https://www.example.com":  "https://www.example.com",
		"https://www.example.com/": "https://www.example.com",
		"http://localhost:8080":    "http://localhost:8080",
	} {
		if got, err := parseSiteURL(raw); err != nil || got != want {
			t.Errorf("parseSiteURL(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "www.example.com", "ftp://example.com", "https://example.com/blog", "https://example.com/?a=1"} {
		if _, err := parseSiteURL(raw); err == nil {
			t.Errorf("parseSiteURL(%q) accepted an invalid origin", raw)
		}
	}
}
