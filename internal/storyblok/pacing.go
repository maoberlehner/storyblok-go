package storyblok

import (
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"storyblok-go-website/internal/apihttp"
)

// Storyblok allows 50 uncached single-story requests per second and up to
// 1000/s served from the CDN cache. Starting at half the uncached limit with
// a burst of the same size stays below it in any one-second window.
const (
	baseRequestRate  = 25
	maxCachedRate    = 1000
	maxPermitWait    = 2 * time.Second
	operationTimeout = 10 * time.Second
	// attemptTimeout leaves room for retries when a single attempt stalls.
	attemptTimeout = 4 * time.Second
)

const (
	cloudFrontCacheHit = "Hit from cloudfront"
	genericCacheHit    = "HIT"
)

// ErrRateLimited reports that a request could not get a permit from the
// client's rate limiter in time.
var ErrRateLimited = apihttp.ErrRateLimited

// cdnPacer paces requests in two tiers. Published story requests with a cv
// can be served from the CDN cache, so their tier grows past the base rate
// while responses are cache hits. Drafts, space metadata, and cv discovery
// stay at or below the base rate in a tier of their own.
type cdnPacer struct {
	cached   *apihttp.Tier
	uncached *apihttp.Tier
}

func newCDNPacer() *cdnPacer {
	tier := func(ceiling float64) *apihttp.Tier {
		return apihttp.NewTier(apihttp.TierConfig{
			Base:     baseRequestRate,
			Ceiling:  rate.Limit(ceiling),
			MaxBurst: baseRequestRate,
			MaxWait:  maxPermitWait,
		})
	}
	return &cdnPacer{cached: tier(maxCachedRate), uncached: tier(baseRequestRate)}
}

func isPublishedStory(request *http.Request) bool {
	return strings.Contains(request.URL.Path, "/stories/") && request.URL.Query().Get("version") == string(Published)
}

func isCacheEligible(request *http.Request) bool {
	return isPublishedStory(request) && request.URL.Query().Get("cv") != ""
}

func isCacheHit(response *http.Response) bool {
	cache := strings.TrimSpace(response.Header.Get("X-Cache"))
	return strings.EqualFold(cache, cloudFrontCacheHit) || strings.EqualFold(cache, genericCacheHit)
}

func (p *cdnPacer) tierFor(request *http.Request) *apihttp.Tier {
	if isCacheEligible(request) {
		return p.cached
	}
	return p.uncached
}

func (p *cdnPacer) Wait(request *http.Request) error {
	return p.tierFor(request).Acquire(request.Context())
}

func (p *cdnPacer) Observe(response *http.Response) {
	tier := p.tierFor(response.Request)
	tier.Record(response, tier == p.cached && isCacheHit(response))
}
