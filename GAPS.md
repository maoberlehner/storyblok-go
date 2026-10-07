# Gaps

Known differences from [AGENTS.md](AGENTS.md). Remove resolved entries; record
deliberate exceptions with reasons.

- **CMS rollout:** The QA space has the current schemas and the seeded demo
  content (`make seed`). Its older stories (`home`, `about`, `blog-index/…`)
  still use removed components and render empty; they need one-off migrations or
  deletion. The CLI reports remote-only components but never deletes them.
- **Schema coverage:** The compiler supports text, textarea, multilink, and
  blocks. Other field types, semantic palette fields, and content/media
  components should be added when needed. Seeded story links set only
  `cached_url`, so the editor shows them without a linked story.
- **Enhancement / forms:** htmx 4 is self-hosted. Load more and the contact form
  follow [patterns/](patterns/README.md). Forms always post against the
  published story, so a form on an unpublished page fails in the Visual Editor.
  Submissions are only logged (`server.LogInbox`); there is no spam protection.
  Preview and dev tooling deliberately require JS because their purpose is live
  editing/debugging; published content renders without them.
- **Legacy tooling CSS:** `static/dev-toolbar.css` still uses pixel values,
  off-scale spacing, and literal colors. Public component CSS now uses relative
  units and global color properties.
- **Asset delivery:** Four variants are implemented behind `ASSET_DELIVERY`
  (default `bundle`) and measured with `make benchmark` on the compose stack
  (Chrome 155, slow 4G, 4x CPU slowdown, pages and fragments served from the
  proxy cache, medians of 5 runs; 2026-10-07). The demo landing page has nine
  sections using 19 components; their CSS is 12 KB raw, 2.2 KB brotli. Only
  `inline` delivers JS per instance; the others link one script bundle.

  | Metric                        | inline |   head |   links | bundle |
  | ----------------------------- | -----: | -----: | ------: | -----: |
  | Cold: transfer KB / requests  | 21.5/2 | 21.6/3 | 27.5/23 | 22.0/4 |
  | Cold: FCP ms                  |    932 |    964 |    1260 |   1044 |
  | Repeat visit: transfer KB     |    0.1 |    0.1 |     0.1 |    0.1 |
  | Navigation: transfer KB       |    5.2 |    4.3 |     2.7 |    2.4 |
  | Load more: KB per click       |    1.4 |    0.7 |     0.7 |    0.7 |
  | After 3× load more: inline KB |   57.1 |   12.1 |       0 |      0 |

  Linked CSS costs a render-blocking round trip on cold visits; per-component
  links (`links`) add per-request overhead on top and are slowest. Repeat visits
  cost nothing in any mode (ETag revalidation, 304). Inlined CSS only costs on
  navigation to other pages: about 2 KB per page. `inline` repeats CSS per
  instance and doubles fragment size with no gain over `head`. Minifying would
  save about 0.25 KB of CSS and 0.3 KB of JS per page after brotli.

  Recommendation, pending decision: `head` (CSS inlined once per component type,
  JS as one bundle); remove the other variants. Inlining keeps winning while a
  page's CSS fits the first round trip with its HTML (about 14 KB compressed,
  roughly 70 KB raw); scoped component CSS is far from that. Not measured: other
  browsers and a real CDN.

- **Compression of errors:** nginx's brotli and gzip modules skip error
  statuses, so 422 form responses are sent uncompressed (5 KB fragment, 17 KB
  page with `head` delivery).
- **CI:** There is no CI. Tests, offline schema validation, and schema
  plan/apply run locally. A pipeline for pull request checks and reviewed schema
  syncs per space is pending.
- **Verification:** The browser support matrix and WCAG 2.2 AA target have not
  had a full audit. The patterns were checked in Chrome with and without
  JavaScript (keyboard, focus after swaps and reloads, reduced motion, 375 and
  1280 px widths); Firefox, Safari, zoom, and screen readers are unchecked. Live
  MAPI behavior still needs a smoke test against a development space; HTTP
  fixture tests cover the sync contract locally.
