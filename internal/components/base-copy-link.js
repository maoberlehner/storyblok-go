// Copies the page's canonical URL. The control renders hidden and is revealed
// where the Clipboard API exists, also after htmx swaps. The click listener is
// delegated, so controls swapped in later work too.

const SELECTOR = "[data-base-copy-link]";
const STATUS_DURATION_MS = 5000;
const statusTimers = new WeakMap();

function pageURL() {
  return document.querySelector('link[rel="canonical"]')?.href ?? location.href;
}

function showStatus(status, text) {
  status.textContent = text;
  clearTimeout(statusTimers.get(status));
  // Clearing lets screen readers announce the same text on the next click.
  statusTimers.set(
    status,
    setTimeout(() => (status.textContent = ""), STATUS_DURATION_MS),
  );
}

async function copy(control) {
  const button = control.querySelector("button");
  const status = control.querySelector("[role=status]");
  try {
    await navigator.clipboard.writeText(pageURL());
    showStatus(status, button.dataset.copied);
  } catch {
    showStatus(status, button.dataset.failed);
  }
}

function revealAll() {
  if (!navigator.clipboard) return;
  document.querySelectorAll(SELECTOR).forEach((control) => {
    control.hidden = false;
  });
}

document.addEventListener("click", (event) => {
  const button = event.target.closest(`${SELECTOR} button`);
  if (button) copy(button.closest(SELECTOR));
});
document.addEventListener("htmx:after:swap", revealAll);
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", revealAll);
} else {
  revealAll();
}
