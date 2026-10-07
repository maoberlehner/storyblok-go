// Shows the remaining characters below textareas with data-character-limit.
// Screen readers hear the count once typing pauses. Listeners are delegated,
// so fields swapped in later work too.

const SELECTOR = "textarea[data-character-limit]";
const ANNOUNCE_DELAY_MS = 1000;
const numberFormat = new Intl.NumberFormat(document.documentElement.lang);
const announceTimers = new WeakMap();
const pluralRules = new Intl.PluralRules(document.documentElement.lang);

function message(field) {
  const remaining = Number(field.dataset.characterLimit) - field.value.length;
  const count = numberFormat.format(Math.abs(remaining));
  const form =
    pluralRules.select(Math.abs(remaining)) === "one" ? "One" : "Other";
  const over = remaining < 0;
  const template = field.dataset[`count${over ? "Over" : "Remaining"}${form}`];
  return { text: template.replace("%s", count), over };
}

function elementsFor(field) {
  let count = field.nextElementSibling;
  if (!count?.classList.contains("base-form-field__count")) {
    count = document.createElement("p");
    count.className = "base-form-field__count";
    count.setAttribute("aria-hidden", "true");
    const announcer = document.createElement("p");
    announcer.className = "base-form-field__announcer";
    announcer.setAttribute("aria-live", "polite");
    field.after(count, announcer);
  }
  return { count, announcer: count.nextElementSibling };
}

function update(field, announce) {
  const { count, announcer } = elementsFor(field);
  const { text, over } = message(field);
  count.textContent = text;
  count.classList.toggle("base-form-field__count--over", over);
  if (!announce) return;
  clearTimeout(announceTimers.get(field));
  announceTimers.set(
    field,
    setTimeout(() => (announcer.textContent = text), ANNOUNCE_DELAY_MS),
  );
}

function updateAll() {
  document.querySelectorAll(SELECTOR).forEach((field) => update(field, false));
}

document.addEventListener("input", (event) => {
  if (event.target.matches(SELECTOR)) update(event.target, true);
});
document.addEventListener("htmx:after:swap", updateAll);
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", updateAll);
} else {
  updateAll();
}
