# 0005: Field-level and folder-level translations side by side

- Status: accepted
- Date: 2026-10-07

## Context

Most pages share their structure across languages and only need translated text
(Storyblok field-level translation). Some markets need pages that differ in
structure or slug, or exist in one language only (folder-level translation). The
site must support both without editors choosing a mode per space.

## Decision

- English is the default language without URL prefix; German lives under `/de/`.
  The Go schemas mark text fields translatable.
- `/de/<path>` shows the story `de/<path>` if it exists (folder-level), else the
  field-level German version of `<path>`. If that story has a published
  alternate (same `group_id`) under `de/`, `/de/<path>` redirects to it.
- Every story outside `de/` therefore has a German URL; untranslated fields show
  the English text.
- Story links in German content point to German pages. The API prefixes links in
  field-level content with `de/`; the server normalizes them so links to
  folder-level stories don't double the prefix.
- `hreflang` links, the sitemap, and the language switcher list each page's
  versions; a German-only page links the switcher to the English home page.
- No redirects based on `Accept-Language`: they break caching and crawlers.

## Consequences

- German URLs of untranslated pages show English text. Excluding them would need
  a per-story flag.
- Slugs are not translated for field-level pages; folder-level stories can use
  German slugs.
- The Visual Editor previews field-level German at the default path through
  `_storyblok_lang`.
