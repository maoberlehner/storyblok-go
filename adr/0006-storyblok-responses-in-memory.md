# 0006: Keep published Storyblok responses in memory, keyed by `cv`

- Status: accepted
- Date: 2026-10-08

## Context

Every page render requested its story and the site settings from Storyblok's
CDN. The nginx page cache hides this for hot pages, but each miss (a new page, a
query-parameter variant, an htmx fragment, a background revalidation) waited for
those requests. Measured in a port of this project (petermax.at) against its
Storyblok space, next to the Astro app it replicates, 1 CPU per container:

| Steady-state TTFB, page cache off | p50    |
| --------------------------------- | ------ |
| Go, no response cache             | 310 ms |
| Astro (60 s in-memory SWR cache)  | 15 ms  |
| Go, this decision                 | 17 ms  |

Throughput through nginx rose from 98 to 109 requests/s at 10 connections, now
without a local Storyblok proxy in between.

Options:

- **Rely on the page cache.** Keeps the app stateless, but misses stay slow and
  each one costs Storyblok requests.
- **Time-based cache, like Astro.** Serves content up to the TTL old, on top of
  the `cv` delay, and needs a revalidation strategy.
- **Cache by request URL including `cv`.** Storyblok's own CDN keys content this
  way: a URL with a given `cv` always returns the same content.
- **Cache decoded stories.** Would also save decoding (about 20% of CPU under
  load), but renders would share decoded values, which then must never be
  mutated.

## Decision

The Content Delivery client keeps the raw responses of published requests that
carry a `cv`, keyed by request URL, in a 64 MiB LRU. Drafts and `cv` discovery
requests always reach the API.

## Consequences

- Freshness is unchanged: content appears once the client learns the new `cv`,
  within the 30-second discovery interval.
- Every page misses once after a publish, since the `cv` changes for the whole
  space.
- `cv` discovery runs in the background, so no request waits for it once a `cv`
  is known.
- Memory grows with the content in use, by at most 64 MiB per instance.
- Responses are still decoded per request; caching decoded stories remains an
  option if CPU becomes the bottleneck.
