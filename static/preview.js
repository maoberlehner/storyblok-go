// Live preview for the Storyblok Visual Editor: every edit is rendered on the
// server and morphed into the page, so the preview always matches production
// markup without a client-side renderer.
const { resolveRelations } = document.currentScript.dataset;
const bridge = new window.StoryblokBridge({
  resolveRelations: resolveRelations ? resolveRelations.split(",") : [],
});
const root = document.querySelector("[data-preview-root]");
let pending;

bridge.on("input", async ({ story }) => {
  // A newer edit supersedes the in-flight render so responses can't arrive out of order.
  pending?.abort();
  pending = new AbortController();
  try {
    const response = await fetch(window.location.href, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(story),
      signal: pending.signal,
    });
    if (!response.ok) throw new Error(`Preview render failed: ${response.status}`);
    window.Idiomorph.morph(root, await response.text(), { morphStyle: "innerHTML" });
  } catch (error) {
    if (error.name !== "AbortError") console.error(error);
  }
});

bridge.on(["published", "change"], () => window.location.reload());
