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
  Only one "load more" listing per page keeps its state in the URL. Submissions
  are only logged (`server.LogInbox`); there is no spam protection. Preview and
  dev tooling deliberately require JS because their purpose is live
  editing/debugging; published content renders without them.
- **Legacy tooling CSS:** `static/dev-toolbar.css` still uses pixel values,
  off-scale spacing, and literal colors. Public component CSS now uses relative
  units and global color properties.
- **Asset delivery:** All three variants are implemented behind `ASSET_DELIVERY`
  (default `bundle`) and measured with `make benchmark` (Chrome 155, slow 4G, 4x
  CPU slowdown, HTTP/2 on localhost, medians of 5 runs, demo landing pages;
  2026-10-07):

  | Metric                        |   inline |     head |   bundle |
  | ----------------------------- | -------: | -------: | -------: |
  | Cold: transfer KB / requests  | 19.1 / 2 | 19.1 / 2 | 19.5 / 4 |
  | Cold: FCP ms                  |      988 |      948 |     1156 |
  | Repeat visit: transfer KB     |      3.9 |      3.9 |      1.7 |
  | Navigation: transfer KB       |      3.8 |      3.8 |      1.5 |
  | Load more: KB per click       |      1.3 |      0.6 |      0.6 |
  | Form error: KB                |      2.6 |      1.7 |      0.9 |
  | After 3× load more: `<style>` |       39 |        1 |        0 |
  | After 3× load more: inline KB |     43.7 |     18.7 |        0 |

  Bundles cost a render-blocking round trip on cold visits (about 200 ms later
  FCP); inlining saves it. Inline repeats CSS per instance, which doubles
  fragment size and bloats the DOM, with no gain over `head`. `head` re-sends
  about 2 KB of CSS per page view that a bundle would cache. JS is not
  render-blocking, but inline delivery runs component scripts once per instance
  (5 copies for one form).

  Recommendation, pending decision: CSS as `head`, JS as one deferred, cacheable
  bundle; then remove the unused variants. Revisit `head` if component CSS per
  page grows far beyond today's ~2 KB compressed. Not measured: other browsers,
  CDN/edge caching, and larger component sets.

- **CI:** There is no CI. Tests, offline schema validation, and schema
  plan/apply run locally. A pipeline for pull request checks and reviewed schema
  syncs per space is pending.
- **Verification:** The browser support matrix and WCAG 2.2 AA target have not
  had a full audit. The patterns were checked in Chrome with and without
  JavaScript (keyboard, focus after swaps and reloads, reduced motion, 375 and
  1280 px widths); Firefox, Safari, zoom, and screen readers are unchecked. Live
  MAPI behavior still needs a smoke test against a development space; HTTP
  fixture tests cover the sync contract locally.
