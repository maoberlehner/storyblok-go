# Load more

Appends the next page of a listing. Reference:
`internal/components/block-section-articles.{go,html,css}`.

## Markup

A native GET form with one submit button and the state as hidden fields:

```html
<div id="b-<uid>-more">
  <p>Showing 6 of 22 articles</p>
  <form
    method="get"
    action="/landing/launch"
    hx-get="/landing/launch"
    hx-target="#b-<uid>-list"
    hx-swap="beforeend"
    hx-disable="find button"
  >
    <input type="hidden" name="_block" value="<uid>" />
    <input type="hidden" name="page" value="2" />
    <button type="submit">Load more articles</button>
  </form>
</div>
```

`page` is the number of pages to show, not an offset, so the URL describes the
whole state.

## Without JavaScript

The form reloads the page. The section renders pages 1 to `page` from one API
request (`per_page = page × 6`, capped by the API maximum of 100). The first new
item's link has `autofocus`: focus moves to it and it scrolls into view.

## With htmx

- The server renders only the requested page: the new items, then an
  `<hx-partial>` that replaces the `…-more` region. On the last page that region
  keeps the count and drops the form.
- The response sets `HX-Replace-Url` to the request URL. A reload, or going back
  after following an article link, renders the same items without JavaScript,
  with focus on the item it last loaded.
- `hx-disable` disables the button during the request; `.htmx-request` on the
  form shows the button's progress indicator.
- The first new item has `autofocus`, so focus moves to it after the swap.
  Screen readers announce the link, and keyboard users continue from there.

## Limits

- A GET form replaces the whole query string, so only one listing per page keeps
  its state in the URL.
- Items need stable IDs by position (`b-<uid>-item-<n>`) so both paths render
  the same markup.

## Adapting

1. Implement `Load` on the section: read `page` when `req.Targets(b)`; fetch one
   page for `req.Enhanced` requests, all pages otherwise.
2. Define the items template once and use it in the section and in its
   `-fragment` template.
3. Test both paths (see `TestLoadMore` in `internal/server`).
