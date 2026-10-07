# Form

A server-validated POST form with an error summary, inline errors, and an inline
success message. Reference: `block-section-contact.{go,html,css}` and the base
components `base-form`, `base-form-field`, `base-form-error-summary`, and
`base-form-button` in `internal/components/`.

## Request flow

The form posts to its own page with `_block=<section short id>`. The server
finds the section, which must implement `components.FormHandler`, and calls
`Submit`. Submissions from other origins are rejected
(`http.CrossOriginProtection`).

| Request    | Invalid                          | Valid                                               |
| ---------- | -------------------------------- | --------------------------------------------------- |
| without JS | 422, full page, title "Error: …" | 303 to `?<state>&sent=<short id>`, page confirms it |
| htmx       | 422, form fragment               | 200, success fragment                               |

The form has `hx-post`, `hx-target="this"`, and `hx-swap="outerHTML"`; htmx 4
swaps 422 responses by default. The success message carries the form's ID, so
the fragment replaces the form in both cases.

## Validation and errors

Follows the GOV.UK Design System guidance on
[validation](https://design-system.service.gov.uk/patterns/validation/) and the
[error summary](https://design-system.service.gov.uk/components/error-summary/):

- `novalidate` on the form: the server validates and reports all errors at once,
  in one consistent style. Keep `required`, `type`, and `autocomplete`; they
  still help assistive technology and autofill.
- The error summary is the first element of the form. It has `tabindex="-1"` and
  `autofocus`, so it takes focus after a failed submit with and without
  JavaScript. Each error links to its field's ID.
- Each invalid field shows its error between hint and control, prefixed with a
  visually hidden "Error:", and has `aria-invalid="true"` and `aria-describedby`
  listing the hint and error IDs.
- Messages say what to do ("Enter your email address"), match between summary
  and field, and appear in field order.
- Submitted values are kept. Line breaks are normalized from CRLF, so length
  limits count like the browser does.
- Don't use `maxlength`: browsers truncate silently. Validate the limit on the
  server and show a character count instead (`CharacterLimit` on
  `BaseFormField`).

## Success

The success message replaces the form, has `tabindex="-1"` and `autofocus`, and
starts with a heading. The redirect after a submission without JavaScript has no
URL fragment, because browsers skip autofocus on URLs with one.

## Component JavaScript

`base-form-field.js` (character count) shows the conventions for scripts that
must survive repeated instances and htmx swaps. Scripts run once, from the
bundle, in their own scope.

- Delegate events to `document`, so swapped-in fields work without setup.
- Render state that is needed before the first interaction on load and on
  `htmx:after:swap`.
- The page works without the script; it only adds the count.

## Spam protection and attribution

Every `BaseForm` renders a honeypot input (`hp_leave_empty`, a name autofill
ignores, hidden from people and assistive technology), a signed `form_started`
token, and hidden attribution fields (UTM parameters, external referrer, landing
page). The server checks submissions before calling `Submit`:

- A filled honeypot drops the submission but answers like a success (`Confirm`),
  so bots learn nothing.
- A missing or invalid token (e.g. after changing `FORM_SECRET` while pages with
  old tokens are cached) shows the form again with a form-level error and a
  fresh token (`Reject`), so people never lose a message silently.
- A submission sent within 3 seconds of rendering shows the form again with a
  form-level error and the entered values (`Reject`); people with autofill can
  simply send again.
- Tokens never expire, because pages are cached for up to a day.

Without JavaScript, attribution carries the current URL's UTM parameters.
`static/attribution.js` keeps the first page's values for the visit in
`sessionStorage` and fills them in on submit.

## Adapting

1. Define fields as `BaseFormField` values in Go, with IDs derived from the
   section's `ElementID`.
2. Implement `Submit`: normalize values, validate into a field → message map,
   deliver valid submissions to the `Inbox` with `AttributionFrom(values)`.
   Implement `Confirm` and `Reject`, and pass `req.FormGuard()` from `Load` to
   `BaseForm.Guard`.
3. Render `{{base "base-form" .BaseForm}}` in the section's `-fragment`
   template, or the success message once it is sent.
4. Test both paths (see `TestContactForm` in `internal/server`).
