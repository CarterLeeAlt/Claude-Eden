// Exercise the shipped noVNC client through its real RFB handshake and raw frames.
// The fixture owns all requests and never reaches a user's VM.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { createWorkbench } from "./workbench-fixture.mjs";

const { startDisplay } = await import("./rfb-wire-fixture.mjs");
const display = await startDisplay(process.env.EDEN_UI_URL || "http://127.0.0.1:5199");
const fixture = await createWorkbench({ mockWebSockets: false, url: display.url });
const { page, state, browser, errors } = fixture;
const report = display.stats;
report.checks = [];
const browserErrors = [];
page.on("console", (message) => {
  if (message.type() === "error") browserErrors.push(message.text());
});
try {
  await page.getByRole("button", { name: "启动 Claude Eden 环境", exact: true }).click();
  await page.getByRole("heading", { name: "Claude Eden 环境已就绪", exact: true }).waitFor();
  await page.getByRole("button", { name: "打开浏览器", exact: true }).click();
  await page.waitForFunction(() => Boolean(document.querySelector(".remote-canvas canvas")));
  await page.getByText("正在连接画面", { exact: true }).waitFor({ state: "hidden" });
  assert.ok(report.connections > 0);
  const deadline = Date.now() + 3000;
  while (report.frames < 5 && Date.now() < deadline)
    await new Promise((resolve) => setTimeout(resolve, 20));
  assert.ok(report.frames >= 5, "display did not deliver its first frames");
  const originalTabs = structuredClone(state.tabs);
  const before = report.frames;
  await page.waitForFunction(() => document.querySelector(".remote-canvas canvas")?.width > 0);
  await page.getByRole("button", { name: "工作台", exact: true }).click();
  // Bounded observation proves frames stop, rather than only inspecting CSS.
  await new Promise((resolve) => setTimeout(resolve, 500));
  const hiddenStart = report.frames;
  await new Promise((resolve) => setTimeout(resolve, 1000));
  report.hiddenFrames = report.frames - hiddenStart;
  report.visibleFrames = before;
  if (!process.env.EDEN_PERF_BASELINE) {
    assert.equal(report.hiddenFrames, 0, "hidden workbench continued receiving frames");
    assert.equal(report.closed, 0, "hidden display lost its retained connection");
    assert.ok(report.visibility.includes(false), "hidden display demand was not released");
  }
  assert.deepEqual(state.tabs, originalTabs, "hiding the display changed guest tabs");
  await page.getByRole("button", { name: "打开浏览器", exact: true }).click();
  await page.getByText("正在连接画面", { exact: true }).waitFor({ state: "hidden" });
  await page.waitForFunction(() => document.querySelector(".remote-canvas canvas")?.width > 0);
  const resumed = report.frames;
  await new Promise((resolve) => setTimeout(resolve, 250));
  assert.ok(report.frames > resumed, "display frames did not resume");
  assert.equal(report.connections, 1, "returning to the browser repeated its handshake");
  if (!process.env.EDEN_PERF_BASELINE) {
    await page.evaluate(() => {
      window.edenWindowVisible = false;
      window.dispatchEvent(new Event("eden-window-visibility"));
    });
    await new Promise((resolve) => setTimeout(resolve, 300));
    const paused = report.frames;
    await new Promise((resolve) => setTimeout(resolve, 300));
    assert.equal(report.frames, paused, "occluded native window continued receiving frames");
    await page.evaluate(() => {
      window.edenWindowVisible = true;
      window.dispatchEvent(new Event("eden-window-visibility"));
    });
    await page.getByText("正在连接画面", { exact: true }).waitFor({ state: "hidden" });
    await new Promise((resolve) => setTimeout(resolve, 250));
    assert.ok(report.frames > paused, "native window visibility did not resume frames");
    await page.getByRole("button", { name: "工作台", exact: true }).click();
    display.disconnectDisplays();
    await new Promise(resolve => setTimeout(resolve, 1500));
    assert.equal(report.connections, 1, "hidden disconnected display retried without a viewer");
    await page.getByRole("button", { name: "打开浏览器", exact: true }).click();
    await page.locator('.remote-view:not(.concealed) .view-overlay').waitFor({ state: 'hidden' });
    assert.equal(report.connections, 2, "hidden disconnection did not reconnect on return");

    await page.route('**/api/claude-desktop', route => route.fulfill({ json: { installed: true, running: true, version: 'fixture' } }));
    await page.route('**/api/claude-desktop/vnc-credentials', route => route.fulfill({ json: { password: '', display_scale: 2 } }));
    await page.getByRole("button", { name: "Claude Desktop", exact: true }).click();
    const desktop = page.getByRole('region', { name: 'Claude Desktop', exact: true });
    await desktop.locator('.remote-view canvas').waitFor();
    await desktop.locator('.view-overlay').waitFor({ state: 'hidden' });
    const desktopStream = report.streams.find(value => value.path === '/api/claude-desktop/vnc');
    assert.ok(desktopStream);
    await new Promise(resolve => setTimeout(resolve, 300));
    const pausedBrowser = report.streams[1].frames, desktopFrames = desktopStream.frames;
    await new Promise(resolve => setTimeout(resolve, 300));
    assert.equal(report.streams[1].frames, pausedBrowser);
    assert.ok(desktopStream.frames > desktopFrames);
    await page.getByRole("button", { name: "打开浏览器", exact: true }).click();
    await new Promise(resolve => setTimeout(resolve, 300));
    const pausedDesktop = desktopStream.frames;
    await new Promise(resolve => setTimeout(resolve, 300));
    assert.equal(desktopStream.frames, pausedDesktop);
    assert.ok(report.streams[1].frames > pausedBrowser);
    await page.getByRole("button", { name: "Claude Desktop", exact: true }).click();
    await desktop.locator('.view-overlay').waitFor({ state: 'hidden' });
    await new Promise(resolve => setTimeout(resolve, 300));
    assert.ok(desktopStream.frames > pausedDesktop);
    assert.equal(report.connections, 3, "switching Browser/Desktop replaced a retained connection");
    assert.equal(report.closed, 1, "switching Browser/Desktop unexpectedly closed a transport");
    await page.getByRole("button", { name: "打开浏览器", exact: true }).click();
    report.checks.push('hidden disconnect reconnects only on return', 'Browser and Desktop retain independent suspended streams');
  }
  assert.deepEqual(errors, []);
  report.checks.push("real RFB frames", "tabs preserved", "frames resumed");
  if (!process.env.EDEN_PERF_BASELINE) report.checks.push("hidden frames stopped", "native occlusion paused");
  const soakSeconds = Number(process.env.EDEN_PERF_SOAK_SECONDS || 0);
  if (soakSeconds > 0) {
    const session = await page.context().newCDPSession(page);
    await session.send("Performance.enable");
    report.soak = [];
    const started = Date.now();
    while ((Date.now() - started) / 1000 < soakSeconds) {
      const beforeResume = report.frames;
      await page.evaluate(() => { window.edenWindowVisible = true; window.dispatchEvent(new Event("eden-window-visibility")); });
      await page.getByText("正在连接画面", { exact: true }).waitFor({ state: "hidden" });
      await new Promise((resolve) => setTimeout(resolve, 1000));
      assert.ok(report.frames > beforeResume, "soak: visible display did not resume frames");
      await page.evaluate(() => { window.edenWindowVisible = false; window.dispatchEvent(new Event("eden-window-visibility")); });
      await new Promise((resolve) => setTimeout(resolve, 500));
      const hidden = report.frames;
      await new Promise((resolve) => setTimeout(resolve, 8500));
      assert.equal(report.frames, hidden, "soak: frames continued in hidden window");
      const { metrics } = await session.send("Performance.getMetrics");
      report.soak.push({ seconds: (Date.now() - started) / 1000, ...Object.fromEntries(metrics.filter((x) => ["JSHeapUsedSize", "JSHeapTotalSize", "TaskDuration", "Documents", "Nodes"].includes(x.name)).map((x) => [x.name, x.value])) });
      if (process.env.EDEN_PERF_REPORT) await fs.writeFile(process.env.EDEN_PERF_REPORT, JSON.stringify(report, null, 2));
    }
    report.checks.push(`${soakSeconds}s retained display suspend/resume soak`);
    await session.detach();
  }
  const output = process.env.EDEN_PERF_REPORT;
  if (output) await fs.writeFile(output, JSON.stringify(report, null, 2));
  console.log(JSON.stringify(report));
} catch (error) {
  console.error(JSON.stringify({ report, errors, browserErrors, visible: await page.locator("body").innerText() }));
  throw error;
} finally {
  await browser.close();
  await display.close();
}
