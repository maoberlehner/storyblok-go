# 0001: Inline component CSS in `<head>`, bundle component scripts

- Status: accepted
- Date: 2026-10-07

## Context

Components keep their CSS and JS next to their templates. Those files can reach
the browser in several ways, with different costs on first visits, repeat
visits, navigation between pages, and htmx fragment updates. Published pages are
served from a shared cache and revalidated by browsers with ETags
(`Cache-Control: no-cache`).

We compared four variants on the compose stack (nginx cache and Brotli in front
of the app), in Chrome 155 with slow 4G emulation and 4x CPU slowdown, medians
of 5 runs. The landing page had nine sections using 19 components; their CSS was
12 KB raw, 2.2 KB Brotli-compressed.

| Metric                        | inline |   head |   links |  bundle |
| ----------------------------- | -----: | -----: | ------: | ------: |
| Cold: transfer KB / requests  | 21.5/2 | 21.6/3 | 27.5/23 |  22.0/4 |
| Cold: first contentful paint  | 932 ms | 964 ms | 1260 ms | 1044 ms |
| Repeat visit: transfer KB     |    0.1 |    0.1 |     0.1 |     0.1 |
| Navigation: transfer KB       |    5.2 |    4.3 |     2.7 |     2.4 |
| Load more: KB per click       |    1.4 |    0.7 |     0.7 |     0.7 |
| After 3× load more: inline KB |   57.1 |   12.1 |       0 |       0 |

- **inline:** CSS and JS next to every component instance.
- **head:** CSS of used components in `<head>`, once per component type.
- **links:** one `<link>` per used component stylesheet.
- **bundle:** one stylesheet for all components.

All variants but `inline` linked one script bundle. The tool is
`tools/asset-benchmark` at commit `540a686`.

## Decision

Pages inline the CSS of the components they render in `<head>`, once per
component type, after the global document defaults. Component scripts are
concatenated into one deferred, immutable-cached `app.js`, each file wrapped in
its own function scope. CSS and JS are not minified.

## Consequences

- First visits skip the render-blocking stylesheet request, about 100 ms ahead
  of a linked bundle and 300 ms ahead of per-component links on slow 4G.
- Navigating to another page re-sends that page's CSS, about 2 KB compressed.
  Repeat visits cost nothing in any variant (304 on ETag revalidation).
- htmx fragments carry no CSS, so a fragment may only render components its page
  already rendered.
- Inlining per instance doubled fragment size and repeated scripts per instance;
  per-component links paid per-request overhead on top of the round trip.
- Minification would save about 0.25 KB of CSS and 0.3 KB of JS per page after
  compression, not worth a build step.
- Revisit when a page's CSS no longer fits the first round trip together with
  its HTML: about 14 KB compressed, roughly 70 KB raw. Per-component links with
  Early Hints or a size budget that switches to links are the candidates then. A
  cookie that marks cached CSS was rejected: it needs `Vary` on a cookie, which
  shared caches handle poorly, may need consent, and doesn't prove the
  stylesheet is still cached.
