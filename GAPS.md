# Gaps

Known differences from [AGENTS.md](AGENTS.md). Remove resolved entries; record
deliberate exceptions with reasons.

- **CMS rollout:** Only `page-landing-page` and `block-section-intro` remain.
  Both have colocated Go schemas; the CLI validates and syncs them. Live
  schemas/content have not been inspected or migrated. Existing stories using
  removed components need one-off migration scripts; the CLI reports remote-only
  components but never deletes them.
- **Schema coverage:** The compiler supports text, textarea, and blocks. Other
  field types, semantic palette fields, and base form/content/media components
  should be added when needed.
- **Enhancement / forms:** htmx is not loaded; there are no public form handlers
  yet. Preview and dev tooling deliberately require JS because their purpose is
  live editing/debugging; published content renders without them.
- **Legacy tooling CSS:** `static/dev-toolbar.css` still uses pixel values,
  off-scale spacing, and literal colors. Public component CSS now uses relative
  units and global color properties.
- **Asset delivery:** Component CSS is concatenated into a hashed, unminified
  stylesheet; preview/dev scripts remain separate. Benchmark pending: inline
  beside component HTML, inline with used CSS deduplicated in `<head>`, and
  external bundles. Delivery decision remains open.
- **CI:** There is no CI. Tests, offline schema validation, and schema
  plan/apply run locally. A pipeline for pull request checks and reviewed schema
  syncs per space is pending.
- **Verification:** The browser support matrix and WCAG 2.2 AA target have not
  had a full audit. Live MAPI behavior still needs a smoke test against a
  development space; HTTP fixture tests cover the sync contract locally.
