# 0003: Canonical URLs without view state and a sitemap for discovery

- Status: accepted
- Date: 2026-10-07

## Context

Page URLs can carry view state ([0002](0002-page-state-in-the-url.md)): loaded
"load more" pages, form confirmations, and Visual Editor preview parameters. A
"load more" page repeats the first page with more items. Crawlers don't submit
forms, so items beyond the first page of a listing are not reachable for them.
Google ignores `rel="next"`/`rel="prev"` and recommends crawlable links for
paginated content, with each page canonical to itself.

## Decision

- `SITE_URL` sets the public origin. Every published page links its absolute URL
  without query parameters as canonical.
- `/sitemap.xml` lists every published story with a page component and its
  publish date; `/robots.txt` points to it.
- Previews are `noindex` and have no canonical URL.
- "Load more" stays a form; there are no crawlable pagination links.

## Consequences

- View-state URLs consolidate on the clean page URL instead of competing with
  it.
- Crawlers find articles through the sitemap and the articles' own URLs, not
  through listings. Listings themselves are only indexed with their first page.
- If paginated listing pages should rank on their own, they need separate,
  non-cumulative URLs with crawlable links, which this decision does not
  provide.
