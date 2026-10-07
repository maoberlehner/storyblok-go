# Patterns

Reference implementations for recurring interactions. Each pattern names its
files; copy the approach, not necessarily the code.

- [Load more](load-more.md): append the next page of a listing.
- [Form](form.md): server-validated form with error summary, inline errors, and
  inline success.

Both share these conventions:

- Requests address a page section by its UID in the `_block` parameter
  (`components.TargetParam`): forms as a hidden field, htmx GET requests with
  `hx-vals`. The page URL handles the request, so the no-JavaScript response is
  the same page in a new state.
- Page state lives in the query string, one parameter set per block
  (`StateParams()`). Forms and redirects carry the state of the other blocks
  over (`Request.StateQuery`).
- htmx requests (`HX-Request: true`) to a section render only its fragment: the
  `<component>-fragment` template if the component defines one. Responses vary
  by `HX-Request`.
- Focus moves with `autofocus` in the rendered HTML. htmx applies it after a
  swap and browsers on page load, so both paths behave the same. Browsers skip
  autofocus when the URL has a fragment, so redirects and form actions don't use
  one.
- Blocks that need more than their content implement `Load(ctx, content, req)`.
  Form blocks implement `components.FormHandler`.
