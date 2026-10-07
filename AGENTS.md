# Storyblok Go Website

Render Storyblok sites with Go and `html/template`, using htmx v4 for
progressive enhancement and plain modern CSS.

These are target practices; we do not follow all of them yet, but strive to.
Follow them for new work and improve touched code incrementally.
[GAPS.md](GAPS.md) tracks known deviations and pending decisions; update it when
these change. Avoid unrelated migrations.

[adr/](adr/README.md) records architecture decisions with their context and
evidence. Read the relevant record before changing such a decision; add one when
a decision chooses between real alternatives, is costly to reverse, or rests on
measurements or research.

## Components

- Structure HTML into components; keep their `.go`, `.html`, `.css`, and
  optional `.js` files alongside each other.
- Every CMS rendering component must have a sibling `.schema.go` definition,
  validated against its Go content fields. Base components have no CMS schema.
  Go definitions are the source of truth for CMS schemas pushed through the
  Management API.
- `base-*`: reusable, CMS-independent building blocks. Extract repeated patterns
  into these components, not global CSS rules; global custom properties may be
  shared. Examples: `base-form`, `base-form-element`, `base-form-field`,
  `base-form-button`, `base-intro`, `base-table`.
- CMS components are pages and blocks; use these names in code and Storyblok
  technical identifiers. Apply them to new components and migrate existing
  identifiers gradually with one-off scripts:
  - `page-<type>` (optionally suffixed): e.g. `page-blog`, `page-landing-page`.
    Page content consists only of consecutive sections as direct children. Every
    page has required `title` (rendered as the `h1` and document title) and
    `description` (meta description) fields; schema validation enforces them.
  - `block-section-*`: page sections, e.g. `block-section-newsletter`,
    `block-section-testimonials`.
  - `block-media-*`: visual blocks, e.g. `block-media-image`,
    `block-media-video`, `block-media-collage`. Sections provide dedicated media
    slots, separate from content areas.
  - `block-content-*`: composable content, e.g. `block-content-headline`,
    `block-content-list`, `block-content-cta`. CMS content areas allow all
    blocks in this category to be mixed, but do not accept media blocks.
- Scope component styles to their component. Keep base components independent of
  page/block implementations. Preserve existing Storyblok identifiers until an
  explicit mapping or content migration is in place.
- Schema sync validates, plans, and applies component definitions; it flags
  changes that may require content migrations. Perform migrations on demand with
  separate one-off scripts, not an automatic migration engine.

## Storyblok content operations

Use the `sba` CLI (`@markus/storyblok-agent`) and the `storyblok-content-ops`
skill to find, read, and change story content and assets, including one-off
content migrations. Component schemas still come from the Go definitions via
schema sync. `make skills` links the skill from the global install; credentials
go in `.env` (see `.env.template`).

## CSS

- Use plain modern CSS and global custom properties for colors.
- CMS color choices use a semantic palette (e.g. default, muted, accent) mapped
  to global custom properties; no arbitrary editor-entered colors.
- Use `rem` and other relative units; no `px` in CSS.
- Layout spacing uses `0.25rem` increments through `2rem`, then `0.5rem`
  increments. Paragraph and heading spacing may use font-relative `em` values.
  Allow documented optical-alignment exceptions, typically for icons.
- Prefer container queries when a component adapts to its context; media queries
  when the whole page layout changes. User-preference media queries remain
  appropriate.
- Keep shared visual patterns in base components; global resets, font
  declarations, and document defaults are distinct from reusable component
  styling.

## JavaScript / forms

- Use htmx as progressive enhancement. Everything works without JavaScript
  unless a case-specific exception is documented with its reason and fallback.
- Use native forms with real `action`, `method`, named controls, and server-side
  validation. Use GET for reads and POST for changes.
- Single server actions, including “load more,” are forms with one submit button
  (plus state fields as needed). Navigation uses links; local controls such as
  popover toggles use native controls.
- Keep component JS alongside its HTML; use global files for global behavior and
  import shared helpers from `utils/<name>.js`.
- Preserve labels, keyboard access, focus, and understandable errors with and
  without enhancement. Initialization must tolerate repeated component instances
  and HTML replacement.

## Patterns

[patterns/](patterns/README.md) holds reference implementations for recurring
interactions; follow them for similar features and add new ones there:

- [Load more](patterns/load-more.md): paginated listings and other "fetch the
  next part" actions.
- [Form](patterns/form.md): server validation, error summary, inline errors,
  inline success, and component JavaScript conventions.

## Asset delivery

Pages inline the CSS of the components they render in `<head>`, once per
component type. htmx fragments carry no CSS, so they may only render components
their page already rendered. Component scripts are bundled into one deferred
`app.js`, each file in its own function scope.

## Verification

Support current and previous major versions of Chrome, Edge, Firefox, and
Safari, including mobile Chrome and Safari.

Target WCAG 2.2 AA. For UI changes, check relevant pages at narrow/wide widths,
keyboard operation, zoom, reduced motion, focus/error handling after htmx
updates, and behavior with JavaScript disabled.

Run `make test` for Go/rendering changes. Run `make fmt` before committing.

## Git

Plain imperative subjects (e.g. `Add newsletter section`); no Conventional
Commits prefixes.

No merge commits: rebase onto `main` and fast-forward merge only.
