# Gaps

Known differences from [AGENTS.md](AGENTS.md), based on repository inspection. Remove resolved entries; record deliberate exceptions with reasons. Live Storyblok schema restrictions have not been audited.

- **Taxonomy / composition:** `internal/components/` uses names such as `enterprise_page`, `enterprise_cta`, and `blocks_group`; no `base-*` / `page-*` / `block-*` taxonomy. `enterprise_page.go` accepts unrestricted `Blocks` in `Body`, rendered directly by `enterprise_page.html`; section-only children are not enforced locally.
- **Shared patterns:** `button.css` exposes shared `.button`, `.link`, and `.button-group` rules without corresponding base HTML components. Reuse exists, but not through the proposed base-component layer.
- **CSS units / spacing:** `base.css` has pixel-based tokens; component CSS also uses pixels and off-scale spacing (e.g. `custom_boxes_grid.css`: `10px` gap, `30px` padding). `enterprise_video.go` embeds CSS in Go, including pixel dimensions and literal colors.
- **Colors:** Global color properties already exist, but literals remain, e.g. in `static/dev-toolbar.css` and the video iframe styles in `enterprise_video.go`. `blocks_group.html` passes CMS background values into inline properties without mapping semantic palette choices to global tokens; the agreed semantic-only palette is not implemented.
- **Responsive components:** Component layouts use viewport media queries (e.g. `card_grid.css`, `feature_showcase.css`); no container queries are present.
- **Enhancement / forms:** htmx is not loaded or used. No form templates or public form submission handlers exist yet, so the form conventions are unexercised. `static/preview.js` uses fetch + Idiomorph; preview and dev tooling require JS and need explicit exception treatment.
- **Asset delivery:** `render.go` concatenates all component CSS into a hashed, unminified stylesheet. Preview/dev scripts remain separate global files; there is no component JS pipeline or `utils/` module convention in use. Benchmark pending: inline beside component HTML, inline with used CSS deduplicated in `<head>`, and external bundles. Delivery decision remains open.
