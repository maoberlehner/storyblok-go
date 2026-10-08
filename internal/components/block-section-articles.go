package components

import (
	"context"
	"encoding/json/v2"
	"fmt"
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
// "load more" link.
type BlockSectionArticles struct {
	storyblok.Blok
	SectionStyle
	Heading string `json:"heading"`
	Folder  string `json:"folder"`
}

// articlesView is the listing as one request shows it.
type articlesView struct {
	*BlockSectionArticles
	Listing ArticleListing
}

type ArticleListing struct {
	// Cards holds the articles to render: all loaded pages for a full page,
	// only the requested page for an enhanced request.
	Cards []BaseCard
	Page  int
	Total int
	// More shows one more page and keeps the other blocks' state; nil after
	// the last page.
	More *BaseButton
}

func (l ArticleListing) Shown() int { return min(l.Page*articlesPerPage, l.Total) }

type articleSummary struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// StateParams names the parameter for the number of pages, which is unique
// per listing, so several listings on a page keep their state.
func (b *BlockSectionArticles) StateParams() []string { return []string{"page-" + ShortID(b)} }

func (b *BlockSectionArticles) Load(ctx context.Context, content Content, req Request) (Block, error) {
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
	// Stories in a language's folder are folder-level translations: the API
	// would prefix their slugs with the language a second time.
	if !strings.HasPrefix(opts.StartsWith, req.Locale.Code+"/") {
		opts.Language = req.Locale.StoryLanguage()
	}
	firstPosition := 1
	if req.Enhanced && req.Targets(b) {
		opts.Page, opts.PerPage = page, articlesPerPage
		firstPosition = (page-1)*articlesPerPage + 1
	}
	list, err := content.Stories(ctx, opts)
	if err != nil {
		return nil, err
	}
	var stories []storyblok.Story[articleSummary]
	if err := json.Unmarshal(list.Stories, &stories); err != nil {
		return nil, fmt.Errorf("decoding articles: %w", err)
	}

	view := &articlesView{BlockSectionArticles: b, Listing: ArticleListing{Page: page, Total: list.Total}}
	if view.Listing.Shown() < view.Listing.Total {
		next := req.StateQuery()
		next.Set(param, strconv.Itoa(page+1))
		view.Listing.More = &BaseButton{
			Href:    req.Path + "?" + next.Encode(),
			Label:   req.Locale.T("articles.load_more"),
			Enhance: &ButtonEnhancement{Block: ShortID(b), Target: "#" + ElementID(b) + "-list", Swap: "beforeend"},
		}
	}
	firstNew := (page-1)*articlesPerPage + 1
	for i, story := range stories {
		position := firstPosition + i
		view.Listing.Cards = append(view.Listing.Cards, BaseCard{
			ID:        fmt.Sprintf("%s-item-%d", ElementID(b), position),
			Title:     story.Content.Title,
			Text:      story.Content.Description,
			Href:      "/" + story.FullSlug,
			Autofocus: page > 1 && position == firstNew,
		})
	}
	return view, nil
}
