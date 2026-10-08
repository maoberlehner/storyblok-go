// Package metrics exposes Prometheus metrics for HTTP requests, Storyblok
// requests, and web vitals reported by browsers.
package metrics

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"storyblok-go-website/internal/server"
	"storyblok-go-website/internal/storyblok"
)

type Metrics struct {
	registry  *prometheus.Registry
	requests  *prometheus.CounterVec
	duration  *prometheus.HistogramVec
	storyblok *prometheus.CounterVec
	vitals    *prometheus.HistogramVec
	cls       prometheus.Histogram
}

func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total", Help: "HTTP responses by route pattern and status.",
		}, []string{"pattern", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "http_request_duration_seconds", Help: "HTTP response time by route pattern.",
			Buckets: prometheus.DefBuckets,
		}, []string{"pattern"}),
		storyblok: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "storyblok_requests_total", Help: "Content requests by operation and outcome, including those served from memory.",
		}, []string{"operation", "outcome"}),
		vitals: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "web_vitals_milliseconds", Help: "Timing web vitals reported by browsers.",
			// Spans the "good" and "poor" thresholds of LCP, INP, FCP, and TTFB.
			Buckets: []float64{100, 200, 500, 800, 1000, 1800, 2500, 3000, 4000, 6000, 10000},
		}, []string{"name"}),
		cls: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "web_vitals_cls", Help: "Cumulative layout shift reported by browsers.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.15, 0.25, 0.5, 1},
		}),
	}
	m.registry.MustRegister(m.requests, m.duration, m.storyblok, m.vitals, m.cls,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Middleware counts responses by the ServeMux pattern that handled them,
// which keeps label values bounded no matter which URLs are requested.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		pattern := r.Pattern
		if pattern == "" {
			pattern = "unmatched"
		}
		m.requests.WithLabelValues(pattern, strconv.Itoa(max(rec.status, http.StatusOK))).Inc()
		m.duration.WithLabelValues(pattern).Observe(time.Since(start).Seconds())
	})
}

// RecordVital records a web vital that the server already validated.
func (m *Metrics) RecordVital(name string, value float64) {
	if name == "CLS" {
		m.cls.Observe(value)
		return
	}
	m.vitals.WithLabelValues(name).Observe(value)
}

// Content counts the Content Delivery API calls of content.
func (m *Metrics) Content(content server.ContentSource) server.ContentSource {
	return instrumentedContent{ContentSource: content, counter: m.storyblok}
}

type instrumentedContent struct {
	server.ContentSource
	counter *prometheus.CounterVec
}

func (c instrumentedContent) Story(ctx context.Context, slug string, opts storyblok.StoryOptions) (jsontext.Value, error) {
	story, err := c.ContentSource.Story(ctx, slug, opts)
	c.counter.WithLabelValues("story", outcome(err)).Inc()
	return story, err
}

func (c instrumentedContent) Stories(ctx context.Context, opts storyblok.StoriesOptions) (storyblok.StoryList, error) {
	list, err := c.ContentSource.Stories(ctx, opts)
	c.counter.WithLabelValues("stories", outcome(err)).Inc()
	return list, err
}

func outcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, storyblok.ErrNotFound):
		return "not_found"
	case errors.Is(err, storyblok.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "error"
	}
}
