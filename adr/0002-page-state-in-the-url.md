# 0002: Keep page state in the URL for enhanced interactions

- Status: accepted
- Date: 2026-10-07

## Context

Interactions such as "load more" and forms must work without JavaScript and be
enhanced with htmx. Without JavaScript, every interaction is a full page
request, so the server must be able to render any state from the request alone.
Several stateful blocks can share a page, and published pages are cached by URL.

## Decision

- Each block keeps its state in its own query parameters, such as
  `page-<short id>` for a listing. The short ID is the first 8 hex digits of the
  block's random UID.
- Forms and redirects carry the state of the page's other blocks over, so one
  block's request doesn't reset the others.
- Interactions request the page URL. Enhanced requests add `_block=<short id>`
  (htmx `hx-vals` or a hidden field for POST) and get only that block's
  fragment. Responses vary by `HX-Request`, which the proxy includes in its
  cache key.
- After an enhanced GET, `HX-Replace-Url` is the current browser URL
  (`HX-Current-URL`) with only the target block's parameters updated, since
  other values in the request can be outdated.
- Focus moves with `autofocus` in the rendered HTML, which htmx applies after a
  swap and browsers on page load. URLs therefore have no fragment, which would
  make browsers skip autofocus.

The patterns in `patterns/` implement this.

## Consequences

- Reloading, sharing, or going back to a URL renders the same state, with and
  without JavaScript, and from the cache.
- A "load more" page re-renders all loaded pages without JavaScript, from one
  API request (at most 100 items).
- Two blocks on a page could share a short ID with negligible probability; then
  the first one receives the other's requests.
- URLs with view state are not separate content for search engines (see
  [0003](0003-search-engine-indexing.md)).
