package storyblok

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func storyResponse(cv int64, header http.Header) *http.Response {
	return apiResponse(200, header, io.NopCloser(strings.NewReader(fmt.Sprintf(`{"story":{"name":"Home"},"cv":%d}`, cv))))
}

func TestClientLearnsAndUpdatesCacheVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		responses := []int64{100, 200, 150, 0}
		queries := []string{"", "100", "200", "200"}
		calls := 0
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.Query().Get("cv"); got != queries[calls] {
				t.Errorf("request %d cv = %q, want %q", calls, got, queries[calls])
			}
			calls++
			return storyResponse(responses[calls-1], nil), nil
		})
		for range responses {
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestClientRefreshesCacheVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		queries := []string{"", "100", "", "200"}
		calls := 0
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.Query().Get("cv"); got != queries[calls] {
				t.Errorf("request %d cv = %q, want %q", calls, got, queries[calls])
			}
			cv := int64(100)
			if calls >= 2 {
				cv = 200
			}
			calls++
			return storyResponse(cv, nil), nil
		})
		for i := range queries {
			if i == 2 {
				time.Sleep(cacheVersionRefreshInterval)
			}
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestClientDiscoversCacheVersionOnceForConcurrentRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		var withoutCV, withStaleCV atomic.Int32
		cv := atomic.Int64{}
		cv.Store(100)
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Query().Get("cv") {
			case "":
				withoutCV.Add(1)
			case "100":
				if cv.Load() == 200 {
					withStaleCV.Add(1)
				}
			}
			time.Sleep(100 * time.Millisecond)
			return storyResponse(cv.Load(), nil), nil
		})
		getConcurrently := func() {
			var wg sync.WaitGroup
			for range 40 {
				wg.Go(func() {
					if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
						t.Error(err)
					}
				})
			}
			wg.Wait()
		}

		getConcurrently()
		if got := withoutCV.Load(); got != 1 {
			t.Errorf("cold start: %d requests without cv, want 1", got)
		}

		time.Sleep(cacheVersionRefreshInterval)
		cv.Store(200)
		withoutCV.Store(0)
		getConcurrently()
		if got := withoutCV.Load(); got != 1 {
			t.Errorf("refresh: %d requests without cv, want 1", got)
		}
		if withStaleCV.Load() == 0 {
			t.Error("requests during the refresh waited instead of using the known cv")
		}
	})
}

func TestClientRediscoversCacheVersionAfterFailedDiscovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		var queries []string
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			queries = append(queries, r.URL.Query().Get("cv"))
			if strings.HasSuffix(r.URL.Path, "/missing") {
				return apiResponse(404, nil, io.NopCloser(strings.NewReader(`{}`))), nil
			}
			return storyResponse(100, nil), nil
		})
		if _, err := client.Story(t.Context(), "missing", StoryOptions{Version: Published}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		for range 2 {
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
				t.Fatal(err)
			}
		}
		if want := []string{"", "", "100"}; fmt.Sprint(queries) != fmt.Sprint(want) {
			t.Errorf("cv per request = %q, want %q", queries, want)
		}
	})
}

func TestClientKeepsRefreshScheduleWhenDiscoveryReportsOlderCacheVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		responses := []int64{100, 50, 100}
		var queries []string
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			queries = append(queries, r.URL.Query().Get("cv"))
			return storyResponse(responses[len(queries)-1], nil), nil
		})
		for i := range responses {
			if i == 1 {
				time.Sleep(cacheVersionRefreshInterval)
			}
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
				t.Fatal(err)
			}
		}
		if want := []string{"", "", "100"}; fmt.Sprint(queries) != fmt.Sprint(want) {
			t.Errorf("cv per request = %q, want %q", queries, want)
		}
	})
}

func TestClientFollowsCacheVersionRedirect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		var queries []string
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			queries = append(queries, r.URL.Query().Get("cv"))
			if !r.URL.Query().Has("cv") {
				location := *r.URL
				query := location.Query()
				query.Set("cv", "100")
				location.RawQuery = query.Encode()
				return apiResponse(301, http.Header{"Location": {location.String()}}, http.NoBody), nil
			}
			return storyResponse(100, http.Header{"X-Cache": {cloudFrontCacheHit}}), nil
		})
		for range 2 {
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
				t.Fatal(err)
			}
		}
		if want := []string{"", "100", "100"}; fmt.Sprint(queries) != fmt.Sprint(want) {
			t.Errorf("cv per request = %q, want %q", queries, want)
		}
	})
}

func TestClientLearnsCacheVersionFromRedirectToMissingStory(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		var queries []string
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			queries = append(queries, r.URL.Query().Get("cv"))
			if !r.URL.Query().Has("cv") {
				location := *r.URL
				query := location.Query()
				query.Set("cv", "100")
				location.RawQuery = query.Encode()
				return apiResponse(301, http.Header{"Location": {location.String()}}, http.NoBody), nil
			}
			if strings.HasSuffix(r.URL.Path, "/missing") {
				return apiResponse(404, nil, io.NopCloser(strings.NewReader(`{}`))), nil
			}
			return storyResponse(100, nil), nil
		})
		if _, err := client.Story(t.Context(), "missing", StoryOptions{Version: Published}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
			t.Fatal(err)
		}
		if want := []string{"", "100", "100"}; fmt.Sprint(queries) != fmt.Sprint(want) {
			t.Errorf("cv per request = %q, want %q", queries, want)
		}
	})
}

func TestDraftAndSpaceRequestsDoNotChangeCacheVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		calls := 0
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			want := ""
			if calls == 3 {
				want = "100"
			}
			if got := r.URL.Query().Get("cv"); got != want {
				t.Errorf("request %d cv = %q, want %q", calls, got, want)
			}
			cv := 100
			if r.URL.Query().Get("version") == "draft" {
				cv = 900
			}
			calls++
			return apiResponse(200, nil, io.NopCloser(strings.NewReader(fmt.Sprintf(`{"story":{},"cv":%d,"space":{"id":123}}`, cv)))), nil
		})
		for _, version := range []Version{Published, Draft} {
			if _, err := client.Story(t.Context(), "home", StoryOptions{Version: version}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := client.SpaceID(t.Context()); err != nil {
			t.Fatal(err)
		}
		if _, err := client.Story(t.Context(), "home", StoryOptions{Version: Published}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestClientFetchesPublishedVersionByDefault(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		var queries []string
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			queries = append(queries, r.URL.Query().Get("version")+":"+r.URL.Query().Get("cv"))
			return storyResponse(100, nil), nil
		})
		for range 2 {
			if _, err := client.Story(t.Context(), "home", StoryOptions{}); err != nil {
				t.Fatal(err)
			}
		}
		if want := []string{"published:", "published:100"}; fmt.Sprint(queries) != fmt.Sprint(want) {
			t.Errorf("version:cv per request = %q, want %q", queries, want)
		}
	})
}

func TestConcurrentResponsesCannotRegressCacheVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := NewClient(DefaultBaseURL, "secret")
		started := make(chan struct{})
		release := make(chan struct{})
		client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			cv := int64(100)
			switch {
			case strings.HasSuffix(r.URL.Path, "/slow"):
				close(started)
				<-release
				cv = 200
			case strings.HasSuffix(r.URL.Path, "/fast"):
				cv = 300
			case strings.HasSuffix(r.URL.Path, "/last"):
				if got := r.URL.Query().Get("cv"); got != "300" {
					t.Errorf("last request cv = %q, want 300", got)
				}
				cv = 300
			}
			return storyResponse(cv, nil), nil
		})
		get := func(slug string) {
			t.Helper()
			if _, err := client.Story(t.Context(), slug, StoryOptions{Version: Published}); err != nil {
				t.Error(err)
			}
		}
		get("home")
		var wg sync.WaitGroup
		wg.Go(func() { get("slow") })
		<-started
		get("fast")
		close(release)
		wg.Wait()
		get("last")
	})
}
