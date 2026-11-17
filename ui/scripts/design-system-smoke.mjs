// Real React components and DOM styles, isolated API peers and a disposable
// browser profile. This is visual/UI regression, not guest/network acceptance.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { createServer } from "./isolated-vite.mjs";
import { createWorkbench } from "./workbench-fixture.mjs";

const output = path.resolve(import.meta.dirname, "../../.tmp/design-system-acceptance", new Date().toISOString().replace(/[:.]/g, "-"));
await fs.mkdir(output, { recursive: true });
const server = await createServer({ server: { host: "127.0.0.1", port: 0, strictPort: true } });
await server.listen();
const url = `http://127.0.0.1:${server.httpServer.address().port}`;
const results = [];
let failure;
async function capture(fixture, name) {
  const { page } = fixture;
  await page.evaluate(() => document.fonts.ready);
  const values = await page.evaluate(() => {
    const root = getComputedStyle(document.documentElement);
    const pageTitle = document.querySelector(".settings-content .section-title h3");
    const headings = [...document.querySelectorAll(".settings-content .section-title h3, .local-workbench-connect h1, .initialization h1")]
      .filter(el => el.getClientRects().length)
      .map(el => ({ text: el.textContent, size: getComputedStyle(el).fontSize,
        expected: el.matches(".settings-content .section-title h3") && el !== pageTitle ? 16 : 20 }));
    const primary = [...document.querySelectorAll("button.primary")].filter(el => el.getClientRects().length)
      .map(el => ({ text: el.textContent, color: getComputedStyle(el).backgroundColor, size: getComputedStyle(el).fontSize }));
    return { paper: root.getPropertyValue("--paper").trim(), accent: root.getPropertyValue("--accent").trim(), family: root.fontFamily, headings, primary };
  });
  assert.equal(values.paper, "#faf9f5", name);
  assert.equal(values.accent, "#a64b32", name);
  assert.match(values.family, /Inter/);
  for (const heading of values.headings) assert.ok(Math.abs(parseFloat(heading.size) - heading.expected) < 0.01, `${name}: ${heading.text}: ${heading.size}, expected ${heading.expected}`);
  for (const button of values.primary) assert.ok(Math.abs(parseFloat(button.size) - 14) < 0.01, `${name}: ${button.text}: ${button.size}`);
  assert.equal(await page.getByText(/后台返回的数据结构无效/).count(), 0, `${name}: contract error visible`);
  assert.deepEqual(fixture.errors, [], name);
  await page.screenshot({ path: path.join(output, `${name}.png`), animations: "disabled" });
  results.push({ name, ...values });
}
try {
  const connectionContext = { mode: "none", desktop_gateway: true, connection_id: "visual-connection", client_platform: "windows", server_platform: "windows", connection: { state: "disconnected" } };
  const connection = await createWorkbench({ url, waitForReady: false, clientContext: connectionContext,
    clientRequest: (route, endpoint) => endpoint === "/client/context"
      ? route.fulfill({ json: connectionContext })
      : route.fulfill({ status: 409, contentType: "text/plain", body: "受控连接失败示例" }),
  });
  try {
    await connection.page.getByRole("button", { name: "重新连接本机", exact: true }).waitFor();
    await capture(connection, "connection-recovery");
  } finally { await connection.browser.close(); }

  const fixture = await createWorkbench({ url, waitForReady: false });
  const { page, browser, state } = fixture;
  page.setDefaultTimeout(15000);
  try {
    await page.locator(".welcome").waitFor();
    await capture(fixture, "workbench");
    await page.getByRole("button", { name: "添加项目", exact: true }).click();
    await page.getByRole("dialog").waitFor();
    await capture(fixture, "project-dialog");
    await page.getByRole("dialog").getByRole("button", { name: "取消", exact: true }).click();
    await page.getByRole("button", { name: "设置", exact: true }).click();
    const groups = page.getByRole("navigation", { name: "设置分组" });
    for (const name of ["通用", "账户", "密码保险箱", "已归档会话", "外观", "网络与代理", "时区与语言", "环境与恢复", "关于"]) {
      await groups.getByRole("button", { name, exact: true }).click();
      await page.locator(".settings-content .section-title").first().waitFor();
      await capture(fixture, `settings-${name}`);
    }
    await page.setViewportSize({ width: 820, height: 580 });
    await capture(fixture, "settings-compact-window");
    await page.setViewportSize({ width: 1280, height: 850 });
    await page.getByRole("button", { name: "返回工作台", exact: true }).click();
    state.status.ready = state.status.vm_running = true;
    state.tabs = [{ id: "design-web", kind: "web", title: "浏览器界面验收", url: "https://example.test" }];
    state.active = "design-web";
    await page.reload();
    await page.getByRole("button", { name: "打开浏览器", exact: true }).click();
    await page.getByRole("button", { name: "浏览器菜单", exact: true }).click();
    await capture(fixture, "browser-menu");
    await page.getByRole("menu", { name: "浏览器菜单" }).getByRole("menuitem", { name: "设置", exact: true }).click();
    await page.getByRole("region", { name: "浏览器设置", exact: true }).waitFor();
    await capture(fixture, "browser-settings");
  } finally { await browser.close(); }

  const initial = await createWorkbench({ url, waitForReady: false, initialization: { required: true, tracked: false, prepared: false } });
  try {
    await initial.page.locator(".guest-profile-actions").waitFor();
    await capture(initial, "initialization-system");
    await initial.page.getByRole("button", { name: "下一步：配置代理", exact: true }).click();
    await initial.page.locator(".proxy-manager").waitFor();
    await capture(initial, "initialization-proxy");
  } finally { await initial.browser.close(); }

  const portableInitial = await createWorkbench({ url, waitForReady: false, portable: true, initialization: { required: true, tracked: false, prepared: false } });
  try {
    await portableInitial.page.locator(".guest-profile-actions").waitFor();
    assert.equal(await portableInitial.page.locator("#initialization-directory").isDisabled(), true);
    assert.equal(await portableInitial.page.getByRole("button", { name: "选择文件夹", exact: true }).count(), 0);
    await capture(portableInitial, "portable-initialization");
  } finally { await portableInitial.browser.close(); }

  const portableSettings = await createWorkbench({ url, waitForReady: false, portable: true });
  try {
    await portableSettings.page.locator(".welcome").waitFor();
    await portableSettings.page.getByRole("button", { name: "设置", exact: true }).click();
    await portableSettings.page.getByRole("navigation", { name: "设置分组" }).getByRole("button", { name: "环境与恢复", exact: true }).click();
    const storage = portableSettings.page.getByRole("region", { name: "本机环境存储", exact: true });
    await storage.getByText(/便携数据全部保存在/).waitFor();
    assert.equal(await storage.getByText("新存储位置", { exact: true }).count(), 0);
    await storage.scrollIntoViewIfNeeded();
    await capture(portableSettings, "portable-storage-settings");
  } finally { await portableSettings.browser.close(); }

  const failed = await createWorkbench({ url, waitForReady: false, beforeNavigate: async page => {
    await page.route("**/api/environment/initialization", route => route.fulfill({ status: 503, contentType: "text/plain", body: "受控初始化离线示例" }));
  } });
  try {
    await failed.page.locator(".initialization-error-content").waitFor();
    await capture(failed, "initialization-recovery");
  } finally { await failed.browser.close(); }
  console.log(`PASS: ${results.length} real-component visual scenes. ${output}`);
} catch (error) {
  failure = error;
  console.error(error.stack || error);
  throw error;
} finally {
  await fs.writeFile(path.join(output, "results.json"), JSON.stringify({ scenes: results, kind: "isolated-ui-only" }, null, 2));
  try { await server.close(); }
  catch (error) { throw failure ? new AggregateError([failure, error], "UI scene and cleanup failures") : error; }
}
