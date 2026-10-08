# 0008: Render components with templ

- Status: accepted
- Date: 2026-10-08

## Context

Components rendered through `html/template`. Every request escaped templates
anew unless it reused a clone, which needed a free list of clones that survives
garbage collections, and render state had to be bound into the template
functions of each clone. Field and method names in templates were checked only
when a template ran, so a renamed field broke a page at runtime.

Measured with `GOGC=800`, which keeps the small benchmark heaps from being
dominated by garbage collections, on an Apple M4 Pro. `BenchmarkPage` in
`internal/components` renders the seeded launch page with site chrome;
`BenchmarkShowStory` in `internal/server` serves a page from a story decoded
once:

| Per page                 | 1 CPU  | 8 CPUs | Allocations |
| ------------------------ | ------ | ------ | ----------- |
| Render, `html/template`  | 240 µs | 47 µs  | 3,500       |
| Render, templ            | 54 µs  | 13 µs  | 1,095       |
| Request, `html/template` | 114 µs | 21 µs  | 1,603       |
| Request, templ           | 28 µs  | 7 µs   | 484         |

Options:

- **Keep `html/template`.** No build step, but runtime field errors and the
  clone pool remain.
- **templ.** Views compile to Go functions: field errors move to compile time,
  views take typed parameters, and rendering writes straight to the output. It
  adds a code generation step.
- **gomponents or similar Go HTML builders.** Compile-time checked without
  generation, but markup turns into nested function calls far from HTML.

## Decision

Components render with templ. Each component has a `.templ` view next to its
`.go`, `.css`, and optional `.js` files. `register` takes the view, so every CMS
component has one; a map from component name to view renders dynamic `bloks`.
Render state (locale, used components, first section) reaches views through the
context. Fragments for enhanced requests are a `Fragment()` method on the block.

The generated `*_templ.go` files are committed, as templ recommends, so the code
builds without running the generator; `make` targets regenerate first.

## Consequences

- Renaming a field or helper fails `go build` instead of a page.
- The template clone pool and its tuning are gone.
- Editing a `.templ` file requires `make generate` (or `templ generate --watch`)
  before Go sees the change. A CI check must ensure the committed output matches
  its sources.
- templ normalizes whitespace: line breaks between elements disappear, and a
  text expression or inline element followed by a line break keeps one space.
  Write label text and adjacent inline elements on one line where a space would
  show.
- Attribute values are always double-quoted and escaped, e.g. `hx-vals` JSON
  renders with `&#34;`; browsers decode it the same.
