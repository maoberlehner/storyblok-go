package metrics_test

import (
	"context"
	"encoding/json/jsontext"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storyblok-go-website/internal/metrics"
	"storyblok-go-website/internal/storyblok"
)

type content struct{ err error }

func (c content) Story(context.Context, string, storyblok.StoryOptions) (jsontext.Value, error) {
	return jsontext.Value(`{}`), c.err
}
func (c content) Stories(context.Context, storyblok.StoriesOptions) (storyblok.StoryList, error) {
	return storyblok.StoryList{}, c.err
}
func (c content) CacheVersion() (int64, bool) { return 1, true }

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	return string(body)
}

func TestMetrics(t *testing.T) {
	m := metrics.New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{slug...}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := m.Middleware(mux)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/some/page", nil))

	_, _ = m.Content(content{}).Story(t.Context(), "home", storyblok.StoryOptions{})
	_, _ = m.Content(content{err: storyblok.ErrNotFound}).Story(t.Context(), "x", storyblok.StoryOptions{})
	_, _ = m.Content(content{err: storyblok.ErrRateLimited}).Stories(t.Context(), storyblok.StoriesOptions{})
	m.RecordVital("LCP", 1800)
	m.RecordVital("CLS", 0.05)

	out := scrape(t, m)
	for _, want := range []string{
		`http_requests_total{pattern="GET /{slug...}",status="418"} 1`,
		`http_request_duration_seconds_count{pattern="GET /{slug...}"} 1`,
		`storyblok_requests_total{operation="story",outcome="ok"} 1`,
		`storyblok_requests_total{operation="story",outcome="not_found"} 1`,
		`storyblok_requests_total{operation="stories",outcome="rate_limited"} 1`,
		`web_vitals_milliseconds_count{name="LCP"} 1`,
		`web_vitals_cls_count 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
}
