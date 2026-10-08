# Load more

Appends the next page of a listing. Reference:
`internal/components/block-section-articles.{go,html,css}`.

## Markup

A link to the page with the next state, enhanced with htmx (`base-button` with
`ButtonEnhancement`):

```html
<div id="b-<short id>-more">
  <p>Showing 6 of 22 articles</p>
  <!-- The query keeps the state of the page's other blocks, e.g. another
       listing (page-<other short id>=3). -->
  <a
    class="base-button"
    href="/landing/launch?page-<other short id>=3&page-<short id>=2"
    hx-get="/landing/launch?page-<other short id>=3&page-<short id>=2"
    hx-vals='{"_block": "<short id>"}'
    hx-target="#b-<short id>-list"
    hx-swap="beforeend"
    hx-sync="this:drop"
    >Load more articles<span class="base-button__indicator"></span
  ></a>
</div>
```

A link rather than a form: the next state has its own URL and needs no input, so
it can open in a new tab, be copied, and be followed by crawlers.

`page-<short id>` is the number of pages the listing shows, not an offset, so
the URL describes the whole state. The short ID (first 8 hex digits of the block
UID) makes it unique per listing; the block declares it with `StateParams()`.

## Without JavaScript

The link reloads the page with the current query plus the new page count, so
every listing keeps its state. The section renders pages 1 to N from one API
request (`per_page = N × 6`, capped by the API maximum of 100). The first new
item's link has `autofocus`: focus moves to it and it scrolls into view. Other
forms on the page, such as the contact form, also carry the query over
(`Request.StateQuery`).

## With htmx

- `hx-vals` adds `_block`, which tells the server to render only this section's
  fragment. It never appears in URLs.
- The server renders only the requested page: the new items, then an
  `<hx-partial>` that replaces the `…-more` region. On the last page that region
  keeps the count and drops the link.
- The response sets `HX-Replace-Url` to the current browser URL
  (`HX-Current-URL`) with only this listing's parameter updated. The link's
  other parameters may be outdated after earlier enhanced requests, so they are
  ignored. A reload, or going back after following an article link, renders the
  same items without JavaScript.
- `hx-sync="this:drop"` ignores clicks while the request runs; htmx would
  otherwise queue them and append the same page twice. `.htmx-request` on the
  link shows its progress indicator.
- The first new item has `autofocus`, so focus moves to it after the swap.
  Screen readers announce the link, and keyboard users continue from there.
- Responses vary by `HX-Request`; the proxy includes it in its cache key, so
  fragments are cached like pages.

## Search engines

Crawlers can follow the link; `/sitemap.xml` lists every item as well. Pages
with state parameters link the clean URL as canonical: they repeat the first
page with more items and are not separate content.

## Adapting

1. Declare the state parameters with `StateParams()` and read them in `Load`;
   fetch one page for `req.Enhanced && req.Targets(b)`, all pages otherwise, and
   return a view of the block with the items.
2. Link to `req.StateQuery()` with your parameter set to the next state.
3. Define the items template once and use it in the section and in its
   `-fragment` template.
4. Test both paths (see `TestLoadMore` in `internal/server`).
