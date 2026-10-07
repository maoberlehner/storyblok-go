// Compares the component asset delivery modes (see ASSET_DELIVERY in main.go)
// on the seeded landing pages. For each mode it starts the site, then
// measures in Chrome on a throttled connection:
//
//   cold        first visit with an empty cache
//   repeat      the same page again with a warm cache
//   navigation  another landing page that shares most components
//   load more   three "load more" fragment requests
//   form error  an invalid contact form submission
//
// Run from the repository root with `make benchmark`. The site fetches
// content from Storyblok, so timings include API latency; compare transfer
// sizes first and timings as medians. Time to first byte is not reported:
// Navigation Timing misattributes the emulated latency.
import { spawn } from "node:child_process";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { chromium } from "playwright";

const MODES = ["inline", "head", "bundle"];
const RUNS = Number(process.env.RUNS ?? 5);
const FIRST_PAGE = "/landing/launch";
const SECOND_PAGE = "/landing/partners";
const LOAD_MORE_CLICKS = 3;
// Lighthouse's "slow 4G" profile with 4x CPU slowdown.
const NETWORK = {
  offline: false,
  latency: 150,
  downloadThroughput: (1.6 * 1024 * 1024) / 8,
  uploadThroughput: (750 * 1024) / 8,
};
const CPU_SLOWDOWN = 4;

const binary = join(await mkdtemp(join(tmpdir(), "asset-benchmark-")), "site");
await run("go", ["build", "-o", binary, "."]);

const results = {};
for (const [i, mode] of MODES.entries()) {
  const port = 9100 + i;
  const server = await startServer(mode, port);
  try {
    results[mode] = await measureMode(`https://localhost:${port}`);
  } finally {
    server.kill();
  }
}
printReport(results);

async function measureMode(origin) {
  const browser = await chromium.launch({ channel: "chrome" });
  const runs = [];
  try {
    for (let i = 0; i < RUNS; i++) runs.push(await measureRun(browser, origin));
  } finally {
    await browser.close();
  }
  return runs;
}

async function measureRun(browser, origin) {
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();
  const cdp = await context.newCDPSession(page);
  await cdp.send("Network.enable");
  await cdp.send("Network.emulateNetworkConditions", NETWORK);
  await cdp.send("Emulation.setCPUThrottlingRate", { rate: CPU_SLOWDOWN });
  const transfer = trackTransfer(cdp);

  const result = {};
  result.cold = await visit(page, transfer, origin + FIRST_PAGE);
  result.repeat = await visit(page, transfer, origin + FIRST_PAGE);
  result.navigation = await visit(page, transfer, origin + SECOND_PAGE);

  await visit(page, transfer, origin + FIRST_PAGE);
  result.loadMore = [];
  for (let i = 0; i < LOAD_MORE_CLICKS; i++) {
    const items = await page.locator(".block-section-articles__item").count();
    transfer.reset();
    const start = Date.now();
    await page.getByRole("button", { name: "Load more articles" }).click();
    await page.waitForFunction(
      (n) =>
        document.querySelectorAll(".block-section-articles__item").length > n &&
        document.activeElement?.matches(".base-card__link"),
      items,
    );
    result.loadMore.push({ ms: Date.now() - start, ...transfer.totals() });
  }
  result.dom = await page.evaluate(() => ({
    styleElements: document.querySelectorAll("style").length,
    inlineScripts: document.querySelectorAll("script:not([src])").length,
    inlineBytes: [...document.querySelectorAll("style, script:not([src])")]
      .map((e) => e.textContent.length)
      .reduce((a, b) => a + b, 0),
  }));

  transfer.reset();
  const start = Date.now();
  await page.getByRole("button", { name: "Send message" }).click();
  await page.waitForFunction(() =>
    document.activeElement?.matches(".base-form-error-summary"),
  );
  result.formError = { ms: Date.now() - start, ...transfer.totals() };

  await context.close();
  return result;
}

async function visit(page, transfer, url) {
  transfer.reset();
  await page.goto(url, { waitUntil: "load" });
  // Cached pages can fire load before their first paint, so wait for both
  // paint entries.
  const timing = await page.evaluate(async () => {
    const entry = (type, match) =>
      new Promise((resolve) =>
        new PerformanceObserver((list) => {
          const found = list.getEntries().filter(match).at(-1);
          if (found) resolve(found);
        }).observe({ type, buffered: true }),
      );
    const [fcp, lcp] = await Promise.all([
      entry("paint", (e) => e.name === "first-contentful-paint"),
      entry("largest-contentful-paint", () => true),
    ]);
    const nav = performance.getEntriesByType("navigation")[0];
    return {
      fcp: fcp.startTime,
      lcp: lcp.startTime,
      load: nav.loadEventEnd || nav.loadEventStart,
    };
  });
  // Let late responses, such as the favicon, finish before counting.
  await page.waitForTimeout(200);
  return { ...timing, ...transfer.totals() };
}

// Sums encoded bytes from the network, so cache hits count as zero.
function trackTransfer(cdp) {
  let bytes = 0;
  let requests = 0;
  cdp.on("Network.loadingFinished", (e) => {
    bytes += e.encodedDataLength;
    if (e.encodedDataLength > 0) requests++;
  });
  return {
    reset: () => ((bytes = 0), (requests = 0)),
    totals: () => ({ bytes, requests }),
  };
}

function startServer(mode, port) {
  const server = spawn(binary, [], {
    env: {
      ...process.env,
      ADDR: `:${port}`,
      ASSET_DELIVERY: mode,
      TLS_CERT_FILE: ".certs/localhost.pem",
      TLS_KEY_FILE: ".certs/localhost-key.pem",
      DEV_TOOLBAR: "",
    },
    stdio: ["ignore", "ignore", "pipe"],
  });
  return new Promise((resolve, reject) => {
    server.stderr.on("data", (chunk) => {
      if (chunk.toString().includes("listening")) resolve(server);
    });
    server.on("exit", (code) =>
      reject(new Error(`server exited with ${code}`)),
    );
  });
}

function run(command, args) {
  return new Promise((resolve, reject) => {
    spawn(command, args, { stdio: "inherit" }).on("exit", (code) =>
      code === 0
        ? resolve()
        : reject(new Error(`${command} exited with ${code}`)),
    );
  });
}

function median(values) {
  const sorted = values.filter((v) => v != null).sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
}

function printReport(results) {
  const kb = (bytes) => (bytes / 1024).toFixed(1);
  const ms = (value) => Math.round(value);
  const rows = [
    ["cold: transfer KB", (r) => kb(r.cold.bytes)],
    ["cold: requests", (r) => r.cold.requests],
    ["cold: FCP ms", (r) => ms(r.cold.fcp)],
    ["cold: LCP ms", (r) => ms(r.cold.lcp)],
    ["cold: load ms", (r) => ms(r.cold.load)],
    ["repeat: transfer KB", (r) => kb(r.repeat.bytes)],
    ["repeat: FCP ms", (r) => ms(r.repeat.fcp)],
    ["navigation: transfer KB", (r) => kb(r.navigation.bytes)],
    ["navigation: FCP ms", (r) => ms(r.navigation.fcp)],
    [
      "load more: transfer KB/click",
      (r) => kb(sum(r.loadMore, "bytes") / r.loadMore.length),
    ],
    [
      "load more: ms/click",
      (r) => ms(sum(r.loadMore, "ms") / r.loadMore.length),
    ],
    ["form error: transfer KB", (r) => kb(r.formError.bytes)],
    ["form error: ms", (r) => ms(r.formError.ms)],
    ["after load more: <style> elements", (r) => r.dom.styleElements],
    ["after load more: inline scripts", (r) => r.dom.inlineScripts],
    ["after load more: inline KB", (r) => kb(r.dom.inlineBytes)],
  ];
  console.log(
    `\nMedians of ${RUNS} runs; slow 4G, ${CPU_SLOWDOWN}x CPU slowdown.\n`,
  );
  console.log(`| Metric | ${MODES.join(" | ")} |`);
  console.log(`| --- | ${MODES.map(() => "---:").join(" | ")} |`);
  for (const [label, metric] of rows) {
    const cells = MODES.map((mode) => {
      const values = results[mode].map((r) => Number(metric(r)));
      return String(median(values));
    });
    console.log(`| ${label} | ${cells.join(" | ")} |`);
  }
}

function sum(items, key) {
  return items.reduce((total, item) => total + item[key], 0);
}
