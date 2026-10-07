package main

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

// Articles combine every subject with every angle, so the listing has enough
// pages to exercise "load more".
var (
	articleSubjects = []string{
		"design tokens", "content modeling", "edge caching", "accessible forms",
		"image delivery", "localization", "preview workflows", "site search",
		"performance budgets", "structured content", "component libraries",
	}
	articleAngles = []struct{ title, description, body string }{
		{
			"A practical guide to %s",
			"Where to start with %s, which mistakes to avoid, and how to tell whether it works.",
			"Most teams adopt %s after a painful launch. This guide covers the decisions that matter in the first weeks, the trade-offs you can postpone, and the signals that tell you it pays off.",
		},
		{
			"What we learned scaling %s",
			"Three years of %s across dozens of sites, condensed into the lessons we would apply again.",
			"What worked for one site broke at twenty. We share how our approach to %s changed as the number of teams, locales, and pages grew, and what we would do differently today.",
		},
	}
	nonSlug = regexp.MustCompile(`[^a-z0-9]+`)
)

const articlesFolder = "articles"

// articles returns the generated articles, oldest first.
func articles() []seedStory {
	var stories []seedStory
	for _, angle := range articleAngles {
		for _, subject := range articleSubjects {
			title := capitalize(fmt.Sprintf(angle.title, subject))
			slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(title), "-"), "-")
			fullSlug := articlesFolder + "/" + slug
			stories = append(stories, seedStory{
				FullSlug: fullSlug,
				Name:     title,
				Content: map[string]any{
					"component":   "page-article",
					"_uid":        uid(fullSlug, 0),
					"title":       title,
					"description": fmt.Sprintf(angle.description, subject),
					"sections": []any{map[string]any{
						"component": "block-section-intro",
						"_uid":      uid(fullSlug, 1),
						"heading":   "Overview",
						"text":      capitalize(fmt.Sprintf(angle.body, subject)),
					}},
				},
			})
		}
	}
	return stories
}

func capitalize(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

// uid derives a stable block UID, so seeding again keeps UIDs unchanged.
func uid(fullSlug string, n int) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s/%d", fullSlug, n))
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}
