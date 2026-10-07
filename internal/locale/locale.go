// Package locale defines the site's languages, how URLs carry them, and the
// UI strings in each.
package locale

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

type Locale struct {
	// Code is the Storyblok language code, the URL prefix, and the HTML lang.
	Code string
	// Name is the language's name in itself, for the language switcher.
	Name string
	// OG is the Open Graph locale.
	OG        string
	messages  map[string]string
	thousands string
}

var (
	English = Locale{Code: "en", Name: "English", OG: "en_US", messages: english, thousands: ","}
	German  = Locale{Code: "de", Name: "Deutsch", OG: "de_DE", messages: german, thousands: "."}
	// Default is the space's default language, served without a URL prefix.
	Default = English
	All     = []Locale{English, German}
)

func (l Locale) IsDefault() bool { return l.Code == Default.Code }

// Prefix is the URL path prefix: empty for the default locale, else "/<code>".
func (l Locale) Prefix() string {
	if l.IsDefault() {
		return ""
	}
	return "/" + l.Code
}

// HomePath is the URL of the locale's home page: "/" or "/<code>".
func (l Locale) HomePath() string { return cmp.Or(l.Prefix(), "/") }

// StoryLanguage is the Content Delivery API language parameter.
func (l Locale) StoryLanguage() string {
	if l.IsDefault() {
		return ""
	}
	return l.Code
}

// FromPath splits a URL path into its locale and the rest, without slashes.
func FromPath(path string) (Locale, string) {
	rest := strings.Trim(path, "/")
	for _, l := range All {
		if l.IsDefault() {
			continue
		}
		if rest == l.Code {
			return l, ""
		}
		if after, ok := strings.CutPrefix(rest, l.Code+"/"); ok {
			return l, after
		}
	}
	return Default, rest
}

func ByCode(code string) (Locale, bool) {
	i := slices.IndexFunc(All, func(l Locale) bool { return l.Code == code })
	if i < 0 {
		return Locale{}, false
	}
	return All[i], true
}

// T returns the UI string for key, formatted with args. Missing translations
// fall back to English, then to the key.
func (l Locale) T(key string, args ...any) string {
	msg, ok := l.messages[key]
	if !ok {
		msg, ok = english[key]
	}
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

func (l Locale) FormatNumber(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + l.thousands + s[i:]
	}
	return s
}

// Keys lists the UI string keys.
func Keys() []string { return slices.Sorted(maps.Keys(english)) }
