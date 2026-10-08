# Patterns

Reference implementations for recurring interactions. Each pattern names its
files; copy the approach, not necessarily the code.

- [Load more](load-more.md): append the next page of a listing.
- [Form](form.md): server-validated form with error summary, inline errors, and
  inline success.
- [Component JavaScript](#component-javascript): scripts that survive repeated
  instances and htmx swaps.

Both share these conventions:

- Requests address a page section by its short ID (`components.ShortID`, the
  first 8 hex digits of its UID) in the `_block` parameter
  (`components.TargetParam`): forms as a hidden field, htmx GET requests from
  links with `hx-vals`. The page URL handles the request, so the no-JavaScript
  response is the same page in a new state.
- Page state lives in the query string, one parameter set per block
  (`StateParams()`). Links, forms, and redirects carry the state of the other
  blocks over (`Request.StateQuery`).
- htmx requests (`HX-Request: true`) to a section render only its fragment: the
  view its `Fragment()` method returns, if the component has one. History
  restores (`HX-History-Restore-Request: true`), which htmx swaps into the body,
  get whole pages and error pages. Responses vary by both headers.
- Focus moves with `autofocus` in the rendered HTML. htmx applies it after a
  swap and browsers on page load, so both paths behave the same. Browsers skip
  autofocus when the URL has a fragment, so links, redirects, and form actions
  don't use one.
- Blocks that need more than their content or the request implement
  `Load(ctx, content, req)`. Decoded stories are shared between requests, so
  `Load` leaves the block unchanged and returns a view that embeds it and holds
  the request's data (`articlesView`, `contactView`); the view renders in the
  block's place. Form views implement `components.FormHandler`.

## Component JavaScript

`base-copy-link.js` (copy the page link, on articles) shows the conventions for
component scripts. Scripts run once, from the bundle, in their own scope.

- Delegate events to `document`, so swapped-in elements work without setup.
- Render state that is needed before the first interaction on load and on
  `htmx:after:swap`.
- Controls that only work with JavaScript render with `hidden`; the script
  reveals them where the browser supports what they need. Without the script the
  page loses only that control.
