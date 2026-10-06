# storyblok-go

A marketing website rendered in Go from Storyblok content, with live preview in the Visual Editor.

## Requirements

- Go 1.27+
- [mkcert](https://github.com/FiloSottile/mkcert) (the Visual Editor needs an HTTPS preview URL)

## Run

```sh
echo 'STORYBLOK_PREVIEW_TOKEN=<preview token of your space>' > .env
make run
```

`make run` downloads the fonts and creates a local certificate on first run, then serves https://localhost:8080 with the dev toolbar enabled (press `S` and click a block to open it in Storyblok).

For live preview, set `https://localhost:8080/` as the preview URL in your space settings.

## Storyblok requests

The shared Storyblok client uses [Failsafe-go](https://failsafe-go.dev/) for retries and [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) for adaptive pacing:

- The rate starts at 25 requests/second. After ten consecutive successful `X-Cache: Hit from cloudfront` responses (or `HIT`), it can double once per second, up to 1000/s. Only published story requests containing `cv` qualify for this cached tier.
- A cache miss, missing cache header, or new cache version caps the rate at 25/s again. A 429 halves it and caps it at 25/s immediately, with a floor of 1/s and a five-second cooldown before growth resumes. Waiting requests recheck the rate after changes; requests already sent cannot be recalled.
- Drafts, space metadata, and requests discovering `cv` have an additional fixed 25/s budget, so they cannot borrow the cached tier's allowance. Both budgets use a burst size of one. The client currently only fetches individual stories and space metadata; listings would need their own limits. See [Storyblok's rate limits](https://www.storyblok.com/docs/api/content-delivery/v2#rate-limits).
- Requests wait up to one second for a rate-limit permit. Longer queues fail rather than holding up server-rendered pages indefinitely.
- Transient transport errors, HTTP 429, and 5xx responses other than 501 get up to three retries. Other 4xx responses, cancellation, and invalid TLS certificates are not retried.
- Retries use exponential backoff starting at 200 ms, capped at two seconds before adding up to 25% jitter. A valid `Retry-After` on 429/503 takes precedence, accepts seconds or an HTTP date, and is never shortened by jitter.
- Each operation has a ten-second total deadline, including permit waits, retries, and reading the response. A shorter caller deadline takes precedence.

Published stories initially omit `cv`. The client learns it from the first successful story response, includes it in subsequent published requests, and adopts newer response values without regressing when concurrent responses arrive out of order. Draft requests omit `cv` and cannot change the published cache version.

An old `cv` can remain cached indefinitely, so cached responses alone cannot discover content updates. Every 30 seconds, the next published request omits `cv` to discover the current value again. This also preserves tokens with a TTL, where `space.version` and the content response's `cv` can differ. See [Storyblok's caching model](https://www.storyblok.com/docs/concepts/caching). Concurrent initial or refresh requests can each omit `cv` until a successful response provides it.

The rate limiter is local to each client instance. Multiple application instances do not share a quota. Cache hits are feedback rather than a guarantee that the next request will also be cached; growth is bounded, and misses return the client to the conservative tier.

## Test

```sh
make test
```
