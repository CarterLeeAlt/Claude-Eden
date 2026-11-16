import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { createWorkbench } from "./workbench-fixture.mjs";

const output = process.env.EDEN_TEST_CAPTURE;
if (output) await fs.mkdir(output, { recursive: true });
let failure = true, release, pending = false;
const requests = [];
const fixture = await createWorkbench({ beforeNavigate: async page => {
  await page.route("**/api/environment/device-check", async route => {
    const { target } = route.request().postDataJSON(); requests.push(target);
    if (pending) await new Promise(resolve => { release = resolve; });
    if (failure) return route.fulfill({ status: 409, contentType: "text/plain; charset=utf-8", body: "环境操作进行中，请稍后检测" });
    return route.fulfill({ json: { target, checked_at: new Date().toISOString(), renderer_class: target === "browser" ? "software" : undefined,
      system: { hostname: "Sample-PC", username: "alex", home: "/home/projects", audio_description: "Sample Speakers" },
      browser: target === "browser" ? { user_agent: "Mozilla/5.0 (X11; Linux x86_64) Chrome/156.0.0.0", webgl: { renderer: "ANGLE (SwiftShader Device)" }, screen: { width: 1920, height: 1080, pixel_ratio: 1 }, viewport: { width: 1280, height: 720 } } : undefined,
      checks: target === "browser" ? [
        { id: "hostname", title: "客机主机名", state: "passed", evidence: "实际值：Sample-PC" },
        { id: "graphics-backend", title: "实际图形后端", state: "warning", evidence: "检测到软件渲染：ANGLE (SwiftShader Device)" },
        { id: "browser-audio", title: "浏览器音频设备", state: "unknown", evidence: "文档未获设备标签权限" },
      ] : [{ id: "desktop-process", title: "Claude Desktop 应用", state: "observed", evidence: "已观测到应用运行状态" }, { id: "desktop-renderer", title: "Claude Desktop 内部渲染器", state: "unknown", evidence: "应用没有受支持的本地渲染自检接口" }],
    } });
  });
} });
try {
  const { page, state, errors } = fixture;
  await page.getByTitle("网络与隔离检查", { exact: true }).click();
  const modal = page.getByRole("dialog", { name: "网络与隔离检查", exact: true });
  const checks = modal.getByRole("region", { name: "设备配置与渲染检测" });
  assert.equal(await checks.getByRole("button", { name: "检测浏览器", exact: true }).isDisabled(), true);
  await page.keyboard.press("Escape");
  Object.assign(state.status, { ready: true, vm_running: true, safety: { gate: { mode: "closed" } } });
  await page.reload();
  await page.getByTitle("网络与隔离检查", { exact: true }).click();
  await checks.getByRole("button", { name: "检测浏览器", exact: true }).click();
  await checks.getByRole("alert").filter({ hasText: "环境操作进行中" }).waitFor();
  failure = false;
  await checks.getByRole("button", { name: "检测浏览器", exact: true }).click();
  await checks.getByText("实际图形环境：软件渲染", { exact: true }).waitFor();
  await checks.getByText("需关注", { exact: true }).waitFor();
  await checks.getByText("未观测", { exact: true }).waitFor();
  await checks.getByText("浏览器实际观测值", { exact: true }).click();
  await checks.getByText("1920 × 1080，像素比 1", { exact: true }).waitFor();
  for (const width of [1280, 560]) {
    await page.setViewportSize({ width, height: 850 });
    await checks.scrollIntoViewIfNeeded();
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    if (output) await page.screenshot({ path: path.join(output, `browser-${width}.png`) });
  }
  await checks.getByRole("button", { name: "检测 Claude Desktop", exact: true }).click();
  await checks.getByText("应用没有受支持的本地渲染自检接口", { exact: true }).waitFor();
  assert.equal(await checks.getByText("实际图形环境：软件渲染", { exact: true }).count(), 0);
  if (output) await page.screenshot({ path: path.join(output, "desktop.png") });
  pending = true;
  await checks.getByRole("button", { name: "检测浏览器", exact: true }).click();
  await checks.getByRole("status").waitFor();
  assert.equal(await checks.getByRole("button", { name: "检测浏览器", exact: true }).isDisabled(), true);
  await page.keyboard.press("Escape");
  await modal.waitFor({ state: "hidden" });
  release(); pending = false;
  await page.getByTitle("网络与隔离检查", { exact: true }).click();
  await checks.getByRole("button", { name: "检测浏览器", exact: true }).waitFor();
  assert.equal(await checks.getByText("实际图形环境：软件渲染", { exact: true }).count(), 0);
  assert.equal(state.requests.filter(r => r.path === "/safety/check" || r.path === "/start").length, 0);
  assert.equal(state.status.safety.gate.mode, "closed");
  assert.deepEqual(requests, ["browser", "browser", "desktop", "browser"]);
  assert.deepEqual(errors, []);
  console.log("PASS device checks: stopped state, failure/retry, offline results, truthful software/unknown states, separate Desktop evidence, close/cancel and widths");
} finally { release?.(); await fixture.browser.close(); }
