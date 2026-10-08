package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newCachingServer(t *testing.T) (*httptest.Server, *fakeContent) {
	t.Helper()
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	content.cv.Store(1)
	content.confirmed.Store(true)
	return ts, content
}

func TestPublishedStoriesAreDecodedOncePerCacheVersion(t *testing.T) {
	ts, content := newCachingServer(t)
	for range 2 {
		if res := request(t, http.MethodGet, ts.URL+"/landing", nil, nil); res.status != http.StatusOK {
			t.Fatalf("status = %d", res.status)
		}
	}
	if n := content.fetchesOf("landing"); n != 1 {
		t.Errorf("landing fetched %d times, want 1", n)
	}

	content.cv.Store(2)
	request(t, http.MethodGet, ts.URL+"/landing", nil, nil)
	if n := content.fetchesOf("landing"); n != 2 {
		t.Errorf("landing fetched %d times after a new cv, want 2", n)
	}
}

func TestDraftsAreFetchedForEveryPreview(t *testing.T) {
	ts, content := newCachingServer(t)
	for range 2 {
		request(t, http.MethodGet, ts.URL+"/landing"+previewQuery(), nil, nil)
	}
	if n := content.fetchesOf("landing"); n != 2 {
		t.Errorf("landing fetched %d times, want 2", n)
	}
}

// Requests render one cached story concurrently; each must only show its own
// state.
func TestCachedStoryKeepsRequestsApart(t *testing.T) {
	ts, _ := newCachingServer(t)
	invalid := map[string][]string{"_block": {"contact"}, "email": {"not-an-email"}}

	t.Run("group", func(t *testing.T) {
		for page := 1; page <= 3; page++ {
			for i := range 4 {
				t.Run(fmt.Sprintf("page %d #%d", page, i), func(t *testing.T) {
					t.Parallel()
					full := request(t, http.MethodGet, fmt.Sprintf("%s/landing?page-arts=%d", ts.URL, page), nil, nil)
					if got, want := len(articleTitles(sectionHTML(t, full.body, "b-arts"))), min(page*6, totalArticles); got != want {
						t.Errorf("listing has %d articles, want %d", got, want)
					}
					if got := len(articleTitles(sectionHTML(t, full.body, "b-more"))); got != 6 {
						t.Errorf("other listing has %d articles, want 6", got)
					}
					if strings.Contains(full.body, `value="not-an-email"`) {
						t.Error("page shows another request's form errors")
					}

					fragment := request(t, http.MethodGet, fmt.Sprintf("%s/landing?page-arts=%d&_block=arts", ts.URL, page), nil, enhanced)
					if got, want := len(articleTitles(fragment.body)), min(6, totalArticles-(page-1)*6); got != want {
						t.Errorf("fragment has %d articles, want %d", got, want)
					}

					form := request(t, http.MethodPost, ts.URL+"/landing", invalid, enhanced)
					if form.status != http.StatusUnprocessableEntity || !strings.Contains(form.body, `value="not-an-email"`) {
						t.Errorf("form fragment: status = %d", form.status)
					}
				})
			}
		}
	})
}
