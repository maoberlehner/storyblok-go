package storyblok

import (
	"cmp"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// versionedCDN answers with the story name current for the cv it reports and
// counts the requests that reach it.
type versionedCDN struct {
	mu       sync.Mutex
	cv       int64
	requests int
	// discoveries counts requests without cv, which discoveryDelay slows down
	// and which report discoveryCV instead of cv if set.
	discoveries    int
	discoveryDelay time.Duration
	discoveryCV    int64
}

func (f *versionedCDN) roundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.requests++
	cv := f.cv
	discovery := !r.URL.Query().Has("cv")
	if discovery {
		f.discoveries++
		cv = cmp.Or(f.discoveryCV, cv)
	}
	f.mu.Unlock()
	if discovery {
		time.Sleep(f.discoveryDelay)
	}
	body := fmt.Sprintf(`{"story":{"name":"Version %d"},"stories":[],"cv":%d}`, cv, cv)
	return apiResponse(200, http.Header{"Total": {"42"}}, io.NopCloser(strings.NewReader(body))), nil
}

func newVersionedCDNClient(cv int64) (*Client, *versionedCDN) {
	cdn := &versionedCDN{cv: cv}
	client := NewClient(DefaultBaseURL, "secret")
	client.api.HTTPClient.Transport = roundTripFunc(cdn.roundTrip)
	return client, cdn
}

func storyName(t *testing.T, client *Client, slug string, version Version) string {
	t.Helper()
	story, err := client.Story(t.Context(), slug, StoryOptions{Version: version})
	if err != nil {
		t.Fatal(err)
	}
	return string(story)
}

func TestClientServesPublishedResponsesFromMemory(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, cdn := newVersionedCDNClient(100)
		// The first request discovers the cv, the second fetches with it.
		for range 5 {
			storyName(t, client, "home", Published)
		}
		if cdn.requests != 2 {
			t.Errorf("%d API requests, want 2", cdn.requests)
		}
		storyName(t, client, "about", Published)
		if cdn.requests != 3 {
			t.Errorf("%d API requests after another slug, want 3", cdn.requests)
		}
	})
}

func TestClientFetchesPublishedContentAgainForNewCacheVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, cdn := newVersionedCDNClient(100)
		storyName(t, client, "home", Published)
		storyName(t, client, "home", Published)
		cdn.cv = 200
		time.Sleep(cacheVersionRefreshInterval)
		storyName(t, client, "home", Published)
		synctest.Wait()
		if got := storyName(t, client, "home", Published); !strings.Contains(got, "Version 200") {
			t.Errorf("story after publishing = %s, want version 200", got)
		}
	})
}

func TestClientFetchesEveryDraft(t *testing.T) {
	client, cdn := newVersionedCDNClient(100)
	storyName(t, client, "home", Draft)
	storyName(t, client, "home", Draft)
	if cdn.requests != 2 {
		t.Errorf("%d API requests, want 2", cdn.requests)
	}
}

func TestClientKeepsTotalOfCachedStoryLists(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, cdn := newVersionedCDNClient(100)
		for range 3 {
			list, err := client.Stories(t.Context(), StoriesOptions{StartsWith: "news/"})
			if err != nil {
				t.Fatal(err)
			}
			if list.Total != 42 {
				t.Errorf("total = %d, want 42", list.Total)
			}
		}
		if cdn.requests != 2 {
			t.Errorf("%d API requests, want 2", cdn.requests)
		}
	})
}
