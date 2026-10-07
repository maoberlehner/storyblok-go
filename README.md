# storyblok-go

A marketing website rendered in Go from Storyblok content, with live preview in
the Visual Editor.

## Requirements

- Go 1.27+
- [mkcert](https://github.com/FiloSottile/mkcert) (the Visual Editor needs an
  HTTPS preview URL)

## Run

```sh
echo 'STORYBLOK_PREVIEW_TOKEN=<preview token of your space>' > .env
make run
```

`make run` creates a local certificate on first run, then serves
https://localhost:8080 with the dev toolbar enabled (press `S` and click a block
to open it in Storyblok).

For live preview, set `https://localhost:8080/` as the preview URL in your space
settings.

## Storyblok requests

The Storyblok client retries with [Failsafe-go](https://failsafe-go.dev/) and
paces requests with
[golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate). The limits
below are fixed and apply per client instance; multiple application instances do
not share a quota.

- Published story requests with a `cv` start at 25 requests/second. After ten
  consecutive cache hits (`X-Cache: Hit from cloudfront` or `HIT`), the rate
  doubles at most once per second, up to 1000/s. A cache miss, a missing cache
  header, or a new `cv` returns it to 25/s.
- Drafts, space metadata, and `cv` discovery have a separate budget of 25/s.
- Both budgets allow a burst of 25. A 429 halves the rate of the budget it came
  from, at most once per second and down to 1/s, and pauses growth for five
  seconds. Successful responses then restore the rate to 25/s.
- A request that cannot get a permit within two seconds fails with
  `ErrRateLimited`. The server answers those page requests with 503.
- Transport errors, 429, and 5xx responses other than 501 are retried up to
  three times, with exponential backoff from 200 ms to 2 s plus up to 25%
  jitter. A `Retry-After` on 429/503 (seconds or HTTP date) is used as-is. If it
  exceeds the remaining deadline, the request fails with that status
  immediately.
- Each attempt times out after four seconds, and each request after ten seconds,
  including permit waits and retries.

Published requests send the latest known `cv`
([Storyblok's caching model](https://www.storyblok.com/docs/concepts/caching)).
The client learns it from story responses and never moves it backwards. An old
`cv` can stay cached indefinitely, so every 30 seconds one published request
omits `cv` to discover the current one while concurrent requests keep using the
known value. `space.version` is not used because it differs from `cv` for tokens
with a minimum cache TTL. Drafts never send or change `cv`.

## Test

```sh
make test
```

## Format

```sh
make fmt
```

Runs `gofmt` and [oxfmt](https://oxc.rs/docs/guide/usage/formatter) (Markdown,
CSS, JS). Go templates (`*.html`) are excluded because oxfmt cannot parse
template actions inside tags.

## Components and schemas

The starter contains exactly two CMS components:

- `page-landing-page`: required `title` and a `sections` field restricted to
  section blocks.
- `block-section-intro`: required `heading` and optional plain `text`.

Every CMS component has matching `.go`, `.schema.go`, `.html`, and `.css` files
in `internal/components/`. Register the typed definition from `.schema.go`;
registration without a schema is not supported. JSON field names and Go types
must match the schema. Field order determines editor order. Base components are
not registered with the CMS.

The schema package currently supports `text`, `textarea`, and `bloks`. Extend
its field validation and compiler when adding other field types. Block
categories resolve to explicit, sorted component allowlists; pages can only
contain sections. `validate` checks definitions and templates offline;
`make test` also checks companion files and rendering.

```sh
go run ./cmd/storyblok-schema validate
go run ./cmd/storyblok-schema validate --out schemas.json
```

## Schema sync

Set `STORYBLOK_MANAGEMENT_TOKEN` to a personal Management API token in your
shell or CI secrets. It is separate from the Content Delivery preview token. Set
`STORYBLOK_SPACE_ID` or pass `--space`. For non-EU spaces set
`STORYBLOK_MAPI_URL` or `--api-url` to the
[regional Management API base URL](https://www.storyblok.com/docs/api/management).

```sh
go run ./cmd/storyblok-schema plan --space 123 --out schema-plan.json
# Review the diff, migration checks, and schema-plan.json.
go run ./cmd/storyblok-schema apply --space 123 --plan schema-plan.json
```

`plan` performs reads only. `apply` checks the target, current definitions, and
live schema against the saved plan before writing. Keep the code revision used
to generate the plan. A changed or stale plan must be regenerated. Schemas are
matched by technical name; numeric component IDs are resolved per space.
Existing field IDs are preserved. Component schemas, display names, and
root/nestable flags are code-owned; top-level editor metadata such as folders
and icons is left untouched.

New components are created as empty shells before their fields are configured,
so references can resolve. Writes are sequential, rate-limited, and verified
afterward. A successful second plan has no changes. Apply is not transactional:
after a partial failure, generate a new plan and apply it. Ambiguous failed
writes are not blindly retried. Avoid concurrent manual schema edits during
apply; CI serializes syncs per space.

The CLI prints `MIGRATION CHECK` for removed fields, changed types/content
constraints, new required fields, changed component roles, and remote-only
components. These are conservative schema checks, not a scan of story data. They
are advisory and do not block apply. Perform content migrations on demand with
separate one-off scripts. The CLI never migrates stories or deletes remote
components.

**Existing content:** the old rendering components and global
navigation/configuration have been removed from this repository. Existing
stories still using those names need a one-off migration before they render with
the new components. Schema sync does not rename them. Unknown blocks are visible
as diagnostics in editor preview and omitted from published output.

## CI

The `Storyblok schemas` GitHub Actions workflow runs tests and offline
validation on pull requests. To sync, configure the `storyblok` environment with
the `STORYBLOK_MANAGEMENT_TOKEN` secret and dispatch the workflow for the
desired code revision and space. It uploads a plan artifact; the optional
`apply` input applies that run's plan. For a preview first, leave `apply`
disabled, inspect the artifact, then run again with `apply` enabled (the new run
generates a fresh plan). No Management API credentials are used in pull request
validation.

MAPI references:
[components](https://www.storyblok.com/docs/api/management/components),
[schema fields](https://www.storyblok.com/docs/api/management/components/the-component-schema-field-object),
[updates and existing story values](https://www.storyblok.com/docs/api/management/components/update-a-component).
