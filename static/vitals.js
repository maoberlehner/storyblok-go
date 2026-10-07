// Reports Core Web Vitals of published pages to the server, which aggregates
// them as metrics. Only runs with JavaScript; there is no fallback by design.
const report = ({ name, value, rating }) => {
  const body = JSON.stringify({ name, value, rating, path: location.pathname });
  navigator.sendBeacon(
    "/vitals",
    new Blob([body], { type: "application/json" }),
  );
};

for (const observe of [
  webVitals.onLCP,
  webVitals.onINP,
  webVitals.onCLS,
  webVitals.onFCP,
  webVitals.onTTFB,
]) {
  observe(report);
}
