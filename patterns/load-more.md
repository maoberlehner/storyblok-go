# Load more

Appends the next page of a listing. Reference:
`internal/components/block-section-articles.{go,html,css}`.

## Markup

A native GET form with one submit button and the state as hidden fields:

```html
<div id="b-<short id>-more">
  <p>Showing 6 of 22 articles</p>
  <form
    method="get"
    action="/landing/launch"
    hx-get="/landing/launch"
    hx-vals='{"_block": "<short id>"}'
    hx-target="#b-<short id>-list"
    hx-swap="beforeend"
    hx-disable="find button"
  >
    <!-- The state of the page's other blocks, e.g. another listing: -->
    <input type="hidden" name="page-<other short id>" value="3" />
    <input type="hidden" name="page-<short id>" value="2" />
    <button type="submit">Load more articles</button>
  </form>
</div>
```

`page-<short id>` is the number of pages the listing shows, not an offset, so
the URL describes the whole state. The short ID (first 8 hex digits of the block
UID) makes it unique per listing; the block declares it with `StateParams()`.

## Without JavaScript

The form reloads the page with the current query plus the new page count, so
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
  keeps the count and drops the form.
- The response sets `HX-Replace-Url` to the current browser URL
  (`HX-Current-URL`) with only this listing's parameter updated. The other
  hidden fields may be outdated after earlier enhanced requests, so they are
  ignored. A reload, or going back after following an article link, renders the
  same items without JavaScript.
- `hx-disable` disables the button during the request; `.htmx-request` on the
  form shows the button's progress indicator.
- The first new item has `autofocus`, so focus moves to it after the swap.
  Screen readers announce the link, and keyboard users continue from there.
- Responses vary by `HX-Request`; the proxy includes it in its cache key, so
  fragments are cached like pages.

## Adapting

1. Declare the state parameters with `StateParams()` and read them in `Load`;
   fetch one page for `req.Enhanced && req.Targets(b)`, all pages otherwise.
2. Render `req.StateQuery()` minus your own parameters as hidden fields.
3. Define the items template once and use it in the section and in its
   `-fragment` template.
4. Test both paths (see `TestLoadMore` in `internal/server`).
