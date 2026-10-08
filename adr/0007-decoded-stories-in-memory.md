# 0007: Share decoded published stories between requests

- Status: accepted
- Date: 2026-10-08

## Context

With raw responses in memory
([ADR 0006](0006-storyblok-responses-in-memory.md)), every request still decoded
its story and the site settings: about 20% of CPU under load in the petermax.at
port. Blocks also held per-request data: `Load` stored a listing's articles or a
form's values and errors on the decoded block, so two requests could never
render the same decoded story.

Options:

- **Copy the decoded story per request.** Needs a deep copy of every block type,
  by hand or by reflection, which costs much of what decoding does.
- **Keep decoding per request.** Simple, but pays for decoding on every render.
- **Share decoded stories and move request data out of blocks.** `Load` returns
  a view that embeds the block and holds the request's data; the renderer shows
  the view in the block's place.

## Decision

The server keeps decoded published stories in an LRU keyed by slug, language,
locale, and `cv`, bounded by 32 MiB of their JSON (decoded, the seed stories
take about half that). Concurrent misses for one key share a single fetch, which
continues if the request that started it is canceled. Drafts and requests before
the first `cv` is known always decode. `Load` returns a view, and `LoadSections`
collects them in a `components.Loaded` map that the page passes to the renderer.

`internal/lru` provides the cache for both this and the response cache.

## Consequences

- Rendering the seed home page takes about 160 µs instead of 190 µs per request
  (`BenchmarkShowStory`, 10 cores). The remaining time is mostly rendering:
  string building per component and garbage collection.
- Blocks are read-only after decoding. A block that changes itself during a
  request races with other requests; `TestCachedStoryKeepsRequestsApart` runs
  under the race detector to catch this.
- Every path that renders a story's sections loads them first, including the 404
  page; a listing or form without its view fails to render.
- Templates of views keep their field access (`.Listing`, `.State`), since the
  view embeds the block.
- A cached story may reflect a newer `cv` than its key, never an older one,
  because the key's `cv` is read before fetching.
