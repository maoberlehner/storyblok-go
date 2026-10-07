package server_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"storyblok-go-website/internal/server"
)

func TestHealthz(t *testing.T) {
	ts, content := newServerWithContent(t, &recordingInbox{}, nil)
	before := content.fetches.Load()
	status, body := do(t, http.MethodGet, ts.URL+"/healthz", "")
	if status != http.StatusOK || body != "ok\n" {
		t.Fatalf("status %d, body %q", status, body)
	}
	if content.fetches.Load() != before {
		t.Error("health check fetched content")
	}
}

type vitalsRecorder struct {
	mu     sync.Mutex
	values map[string]float64
}

func (v *vitalsRecorder) record(name string, value float64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.values[name] = value
}

func TestVitals(t *testing.T) {
	rec := &vitalsRecorder{values: map[string]float64{}}
	ts, _ := newServerWithContent(t, &recordingInbox{}, []server.Option{server.WithVitals(rec.record)})

	status, _ := do(t, http.MethodPost, ts.URL+"/vitals", `{"name":"LCP","value":1234.5,"rating":"good","path":"/landing"}`)
	if status != http.StatusNoContent || rec.values["LCP"] != 1234.5 {
		t.Fatalf("status %d, recorded %v", status, rec.values)
	}
	for _, body := range []string{
		`{"name":"FID","value":1,"rating":"good","path":"/"}`,
		`{"name":"CLS","value":-1,"rating":"good","path":"/"}`,
		`{"name":"INP","value":1e9,"rating":"poor","path":"/"}`,
		`{"name":"TTFB","value":1,"rating":"great","path":"/"}`,
		`{"name":"FCP","value":1,"rating":"good","path":"https://evil.example/"}`,
		`not json`,
		`{"name":"LCP","value":1,"rating":"good","path":"/` + strings.Repeat("a", 2000) + `"}`,
	} {
		if status, _ := do(t, http.MethodPost, ts.URL+"/vitals", body); status != http.StatusBadRequest {
			t.Errorf("%.40s: status %d, want 400", body, status)
		}
	}
}

func TestPublishedPagesReportVitals(t *testing.T) {
	ts := newServer(t)
	_, published := do(t, http.MethodGet, ts.URL+"/", "")
	if !strings.Contains(published, `src="/assets/vendor/web-vitals-6.2.3.iife.js"`) || !strings.Contains(published, `src="/assets/vitals.js"`) {
		t.Error("published page does not load the vitals scripts")
	}
	_, preview := do(t, http.MethodGet, ts.URL+"/"+previewQuery(), "")
	if strings.Contains(preview, "vitals.js") {
		t.Error("preview reports vitals")
	}
}
