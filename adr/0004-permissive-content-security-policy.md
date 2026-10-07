# 0004: A Content Security Policy that tolerates tag managers

- Status: accepted
- Date: 2026-10-07

## Context

Marketing adds scripts through Google Tag Manager and similar tools without
deployments. GTM's custom HTML tags and custom JavaScript variables need inline
scripts and `eval`, and the snippets they load come from origins nobody lists in
advance. A nonce- or allowlist-based policy would break them or need a code
change for every new vendor.

## Decision

- Allow scripts, styles, connections, frames, and media from `'self'` and any
  HTTPS origin, plus `'unsafe-inline'` and `'unsafe-eval'` for scripts.
- Enforce what costs marketing nothing: `object-src 'none'`, `base-uri 'self'`,
  `form-action 'self'`, `upgrade-insecure-requests`, and
  `frame-ancestors 'self' https://app.storyblok.com` (the Visual Editor).
- Send `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: strict-origin-when-cross-origin`, and
  `Strict-Transport-Security: max-age=31536000` for HTTPS origins, without
  `includeSubDomains` or `preload`.
- Send no `Permissions-Policy`: restricting camera, microphone, or geolocation
  would also block embeds marketing adds (meetings, maps); the site itself uses
  none of them.
- Go sets the headers for every response; the CDN passes them through.

## Consequences

- The policy does not stop injected inline scripts (XSS). Template escaping
  remains the defense against those.
- Tightening later (nonces with `'strict-dynamic'`) requires inventorying the
  tag manager's tags first.
