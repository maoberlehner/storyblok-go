// Local development toolbar: press "Edit in Storyblok" or S, then click a
// block to open it in the Visual Editor.
const { spaceId, storyId } = document.currentScript.dataset;
const SHORTCUT = "s";

const toolbar = document.createElement("div");
toolbar.className = "dev-toolbar";
toolbar.innerHTML = `<button type="button" class="dev-toolbar__button" aria-pressed="false">Edit in Storyblok <kbd>S</kbd></button>`;
const button = toolbar.querySelector("button");

const highlight = document.createElement("div");
highlight.className = "dev-toolbar-highlight";
highlight.hidden = true;

document.body.append(toolbar, highlight);

let selecting = false;
let pointer = { x: 0, y: 0 };

function editorUrl(uid) {
  return `https://app.storyblok.com/#/me/spaces/${spaceId}/stories/0/0/${storyId}/blok/${uid}`;
}

function blockAt(target) {
  return target instanceof Element ? target.closest("[data-dev-blok]") : null;
}

function setSelecting(value) {
  selecting = value;
  button.setAttribute("aria-pressed", String(value));
  document.documentElement.classList.toggle("dev-toolbar-selecting", value);
  if (!value) highlight.hidden = true;
}

function showHighlight(block) {
  if (!block) {
    highlight.hidden = true;
    return;
  }
  const rect = block.getBoundingClientRect();
  Object.assign(highlight.style, {
    top: `${rect.top}px`,
    left: `${rect.left}px`,
    width: `${rect.width}px`,
    height: `${rect.height}px`,
  });
  highlight.dataset.label = block.dataset.devComponent;
  highlight.hidden = false;
}

button.addEventListener("click", () => setSelecting(!selecting));

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && selecting) {
    setSelecting(false);
    return;
  }
  const typing = event.target.closest?.("input, textarea, select, [contenteditable]");
  if (event.key.toLowerCase() !== SHORTCUT || typing || event.metaKey || event.ctrlKey || event.altKey) return;
  event.preventDefault();
  setSelecting(!selecting);
});

document.addEventListener("pointermove", (event) => {
  pointer = { x: event.clientX, y: event.clientY };
  if (selecting) showHighlight(blockAt(event.target));
});

// Scrolling moves content under a resting pointer.
window.addEventListener("scroll", () => {
  if (selecting) showHighlight(blockAt(document.elementFromPoint(pointer.x, pointer.y)));
}, { passive: true });

// Capture phase so links and buttons inside the block don't fire.
document.addEventListener("click", (event) => {
  if (!selecting || toolbar.contains(event.target)) return;
  event.preventDefault();
  event.stopPropagation();
  const block = blockAt(event.target);
  if (!block) return;
  window.open(editorUrl(block.dataset.devBlok), "_blank", "noopener");
  setSelecting(false);
}, { capture: true });
