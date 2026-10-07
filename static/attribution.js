// Remembers where a visit came from — UTM parameters, the external referrer,
// and the first page — for the visit's duration, and fills them into forms on
// later pages. Without JavaScript, forms only carry the current URL's UTM
// parameters.
const key = "attribution";
const utmNames = [
  "utm_source",
  "utm_medium",
  "utm_campaign",
  "utm_term",
  "utm_content",
];

const firstTouch = () => {
  try {
    const stored = JSON.parse(sessionStorage.getItem(key));
    if (stored) return stored;
  } catch {}
  const params = new URLSearchParams(location.search);
  const data = { landing_page: location.pathname + location.search };
  for (const name of utmNames) {
    const value = params.get(name);
    if (value) data[name] = value;
  }
  try {
    if (
      document.referrer &&
      new URL(document.referrer).origin !== location.origin
    ) {
      data.referrer = document.referrer;
    }
    sessionStorage.setItem(key, JSON.stringify(data));
  } catch {}
  return data;
};

const attribution = firstTouch();

// Runs in the capture phase, before htmx serializes the form.
document.addEventListener(
  "submit",
  (event) => {
    for (const input of event.target.querySelectorAll(
      "input[data-attribution]",
    )) {
      if (!input.value && attribution[input.name])
        input.value = attribution[input.name];
    }
  },
  true,
);
