package server

import (
	"encoding/json/v2"
	"net/http"
	"strings"
)

const (
	maxVitalBodyBytes = 1 << 10
	maxVitalPathBytes = 512
)

// vitalLimits bounds plausible values per metric: milliseconds for timings,
// a unitless score for CLS. Larger values are bogus or malicious.
var vitalLimits = map[string]float64{
	"LCP": 120_000, "INP": 120_000, "FCP": 120_000, "TTFB": 120_000, "CLS": 100,
}

var vitalRatings = map[string]bool{"good": true, "needs-improvement": true, "poor": true}

// WithVitals receives the web vitals that browsers report.
func WithVitals(record func(name string, value float64)) Option {
	return func(s *Server) { s.recordVital = record }
}

func (s *Server) serveHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) receiveVital(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Name   string  `json:"name"`
		Value  float64 `json:"value"`
		Rating string  `json:"rating"`
		Path   string  `json:"path"`
	}
	if err := json.UnmarshalRead(http.MaxBytesReader(w, r.Body, maxVitalBodyBytes), &v); err != nil {
		http.Error(w, "invalid vital", http.StatusBadRequest)
		return
	}
	limit, known := vitalLimits[v.Name]
	if !known || v.Value < 0 || v.Value > limit || !vitalRatings[v.Rating] ||
		!strings.HasPrefix(v.Path, "/") || strings.HasPrefix(v.Path, "//") || len(v.Path) > maxVitalPathBytes {
		http.Error(w, "invalid vital", http.StatusBadRequest)
		return
	}
	if s.recordVital != nil {
		s.recordVital(v.Name, v.Value)
	}
	s.logger.InfoContext(r.Context(), "web vital", "name", v.Name, "value", v.Value, "rating", v.Rating, "path", v.Path)
	w.WriteHeader(http.StatusNoContent)
}
