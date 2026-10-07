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

The shared Storyblok client uses [Failsafe-go](https://failsafe-go.dev/) for retries and rate limiting:

- Requests are spaced at a maximum of 25 per second per client, including retries and site configuration fetches. This leaves headroom below Storyblok's [50/s single-entry limit](https://www.storyblok.com/docs/api/content-delivery/v2#rate-limits). The client currently only fetches individual stories and space metadata; listings would need their own limits.
- Requests wait up to one second for a rate-limit permit. Longer queues fail rather than holding up server-rendered pages indefinitely.
- Transient transport errors, HTTP 429, and 5xx responses other than 501 get up to three retries. Other 4xx responses, cancellation, and invalid TLS certificates are not retried.
- Retries use exponential backoff starting at 200 ms, capped at two seconds before adding up to 25% jitter. A valid `Retry-After` on 429/503 takes precedence, accepts seconds or an HTTP date, and is never shortened by jitter.
- Each operation has a ten-second total deadline, including permit waits, retries, and reading the response. A shorter caller deadline takes precedence.

The rate limiter is local to each client instance. Multiple application instances do not share a quota. Failsafe-go does not automatically adjust this request rate from `X-Cache` headers or 429 responses; that would require a separate cache-aware controller. Its adaptive throttler rejects requests based on recent failures rather than adjusting a requests-per-second limit.

## Test

```sh
make test
```
