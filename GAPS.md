# Gaps

Known differences from [AGENTS.md](AGENTS.md). Remove resolved entries; record
deliberate exceptions with reasons.

- **CMS rollout:** The QA space has the current schemas and the seeded demo
  content (`make seed`). Its older stories (`about`, `blog-index/…`) still use
  removed components and render empty; they need one-off migrations or deletion.
  The CLI reports remote-only components but never deletes them.
- **Schema coverage:** The compiler supports text, textarea, markdown, option,
  asset, multilink, bloks, and groups. Other field types, semantic palette
  fields, and content/media components should be added when needed. Seeded story
  links set only `cached_url`, so the editor shows them without a linked story.
  Media blocks: only `block-media-image`. Images are served as AVIF through
  `<picture>` with WebP/original fallback; Storyblok's image service negotiates
  only WebP by itself.
- **Enhancement / forms:** htmx 4 is self-hosted. Load more and the contact form
  follow [patterns/](patterns/README.md). Forms always post against the
  published story, so a form on an unpublished page fails in the Visual Editor.
  Submissions are only logged (`server.LogInbox`), with attribution values. Spam
  protection is a honeypot and time trap; there is no rate limiting. Attribution
  across pages requires JavaScript (`sessionStorage`). Preview and dev tooling
  deliberately require JS because their purpose is live editing/debugging;
  published content renders without them. Web vitals reporting requires
  JavaScript by nature.
- **Legacy tooling CSS:** `static/dev-toolbar.css` still uses pixel values,
  off-scale spacing, and literal colors. Public component CSS now uses relative
  units and global color properties.
- **Compression of errors:** nginx's brotli and gzip modules skip error
  statuses, so 422 form responses are sent uncompressed (about 4 KB fragment, 17
  KB page).
- **CI:** There is no CI. Tests, offline schema validation, and schema
  plan/apply run locally. A pipeline for pull request checks and reviewed schema
  syncs per space is pending.
- **Verification:** The browser support matrix and WCAG 2.2 AA target have not
  had a full audit. The patterns were checked in Chrome with and without
  JavaScript (keyboard, focus after swaps and reloads, reduced motion, 375 and
  1280 px widths); Firefox, Safari, zoom, and screen readers are unchecked. The
  site chrome, images, content section, palette, forms (time trap with htmx,
  attribution), and English/German pages were checked in Chrome at 500 and 1280
  px; keyboard (popover Esc and focus return); the production-like stack's
  headers. Schema plan/apply and seeding were smoke-tested against the QA space.
- **Languages:** German URLs exist for every page outside `de/`, translated or
  not (ADR 0005). Slugs are not translated for field-level pages. In spaces that
  publish languages separately (QA space), a German version that was never
  published answers 404 although `hreflang`, the sitemap, and the language
  switcher list it; `make seed` publishes German for seeded stories. Navigation
  links point to field-level URLs, so links to pages with a folder-level
  translation go through a redirect.
