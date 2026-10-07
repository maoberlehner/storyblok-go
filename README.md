# storyblok-go

A marketing website rendered in Go from Storyblok content, with live preview in
the Visual Editor.

## Requirements

- Go 1.27+
- [mkcert](https://github.com/FiloSottile/mkcert) (the Visual Editor needs an
  HTTPS preview URL)

## Run

```sh
echo 'STORYBLOK_PREVIEW_TOKEN=<preview token of your space>' > .env
make run
```

`make run` downloads the fonts and creates a local certificate on first run,
then serves https://localhost:8080 with the dev toolbar enabled (press `S` and
click a block to open it in Storyblok).

For live preview, set `https://localhost:8080/` as the preview URL in your space
settings.

## Storyblok requests

The Storyblok client retries with [Failsafe-go](https://failsafe-go.dev/) and
paces requests with
[golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate). The limits
below are fixed and apply per client instance; multiple application instances do
not share a quota.

- Published story requests with a `cv` start at 25 requests/second. After ten
  consecutive cache hits (`X-Cache: Hit from cloudfront` or `HIT`), the rate
  doubles at most once per second, up to 1000/s. A cache miss, a missing cache
  header, or a new `cv` returns it to 25/s.
- Drafts, space metadata, and `cv` discovery have a separate budget of 25/s.
- Both budgets allow a burst of 25. A 429 halves the rate of the budget it came
  from, at most once per second and down to 1/s, and pauses growth for five
  seconds. Successful responses then restore the rate to 25/s.
- A request that cannot get a permit within two seconds fails with
  `ErrRateLimited`. The server answers those page requests with 503.
- Transport errors, 429, and 5xx responses other than 501 are retried up to
  three times, with exponential backoff from 200 ms to 2 s plus up to 25%
  jitter. A `Retry-After` on 429/503 (seconds or HTTP date) is used as-is. If it
  exceeds the remaining deadline, the request fails with that status
  immediately.
- Each attempt times out after four seconds, and each request after ten seconds,
  including permit waits and retries.

Published requests send the latest known `cv`
([Storyblok's caching model](https://www.storyblok.com/docs/concepts/caching)).
The client learns it from story responses and never moves it backwards. An old
`cv` can stay cached indefinitely, so every 30 seconds one published request
omits `cv` to discover the current one while concurrent requests keep using the
known value. `space.version` is not used because it differs from `cv` for tokens
with a minimum cache TTL. Drafts never send or change `cv`.

## Test

```sh
make test
```

## Format

```sh
make fmt
```

Runs `gofmt` and [oxfmt](https://oxc.rs/docs/guide/usage/formatter) (Markdown,
CSS, JS). Go templates (`*.html`) are excluded because oxfmt cannot parse
template actions inside tags.
