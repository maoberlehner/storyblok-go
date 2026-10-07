package components

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"storyblok-go-website/internal/storyblok"
)

const (
	articlesPerPage = 6
	// Without JavaScript, every loaded page is rendered again from a single
	// request, and the Content Delivery API returns at most 100 stories.
	maxArticlePages = 100 / articlesPerPage
)

// BlockSectionArticles lists the articles in a folder, newest first, with a
// "load more" button.
type BlockSectionArticles struct {
	storyblok.Blok
	Heading string `json:"heading"`
	Folder  string `json:"folder"`

	Listing ArticleListing `json:"-"`
}

type ArticleListing struct {
	// Cards holds the articles to render: all loaded pages for a full page,
	// only the requested page for an enhanced request.
	Cards []BaseCard
	Page  int
	Total int
	Path  string
	// Param holds the number of pages to show.
	Param string
	// State carries the other blocks' state through the form.
	State []FormValue
}

func (l ArticleListing) Shown() int    { return min(l.Page*articlesPerPage, l.Total) }
func (l ArticleListing) HasMore() bool { return l.Shown() < l.Total }
func (l ArticleListing) NextPage() int { return l.Page + 1 }

type articleSummary struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// StateParams names the parameter for the number of pages, which is unique
// per listing, so several listings on a page keep their state.
func (b *BlockSectionArticles) StateParams() []string { return []string{"page-" + b.UID} }

func (b *BlockSectionArticles) Load(ctx context.Context, content Content, req Request) error {
	param := b.StateParams()[0]
	page := 1
	if n, err := strconv.Atoi(req.Query.Get(param)); err == nil {
		page = min(max(n, 1), maxArticlePages)
	}
	opts := storyblok.StoriesOptions{
		Version:         req.Version,
		StartsWith:      strings.Trim(b.Folder, "/") + "/",
		ContentType:     "page-article",
		SortBy:          "first_published_at:desc",
		ExcludingFields: []string{"sections"},
		Page:            1,
		PerPage:         page * articlesPerPage,
	}
	firstPosition := 1
	if req.Enhanced && req.Targets(b) {
		opts.Page, opts.PerPage = page, articlesPerPage
		firstPosition = (page-1)*articlesPerPage + 1
	}
	list, err := content.Stories(ctx, opts)
	if err != nil {
		return err
	}
	var stories []storyblok.Story[articleSummary]
	if err := json.Unmarshal(list.Stories, &stories); err != nil {
		return fmt.Errorf("decoding articles: %w", err)
	}

	b.Listing = ArticleListing{Page: page, Total: list.Total, Path: req.Path, Param: param}
	state := req.StateQuery()
	for _, key := range slices.Sorted(maps.Keys(state)) {
		for _, value := range state[key] {
			if key != param {
				b.Listing.State = append(b.Listing.State, FormValue{Name: key, Value: value})
			}
		}
	}
	firstNew := (page-1)*articlesPerPage + 1
	for i, story := range stories {
		position := firstPosition + i
		b.Listing.Cards = append(b.Listing.Cards, BaseCard{
			ID:        fmt.Sprintf("%s-item-%d", ElementID(b), position),
			Title:     story.Content.Title,
			Text:      story.Content.Description,
			Href:      "/" + story.FullSlug,
			Autofocus: page > 1 && position == firstNew,
		})
	}
	return nil
}
