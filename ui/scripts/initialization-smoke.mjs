import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { createWorkbench } from "./workbench-fixture.mjs";

const evidence = process.env.EDEN_INITIALIZATION_EVIDENCE;
if (evidence) await fs.mkdir(evidence, { recursive: true });

for (const system of ["debian-13"]) {
  const platform = "windows";
  let starts = 0, rejectComplete = true;
  let directory = "C:/preview/eden-state";
  const initialDirectory = "C:/preview/Eden/EdenData";
  const fixture = await createWorkbench({
    initialization: { required: true, tracked: true, started: false },
    clientContext: { mode: "local", connection_id: "local", client_platform: platform, server_platform: platform },
    beforeNavigate: async (page, state) => {
      // Audio is outside this initialization flow; avoid unrelated in-flight audio requests during reload.
      await page.addInitScript(() => localStorage.setItem("eden.audio", JSON.stringify({ muted: true })));
      state.status.local_vm_storage = true;
      await page.route("**/api/environment/storage", route => {
        if (route.request().method() === "POST") {
          assert.equal(route.request().postDataJSON().previous, directory);
          directory = route.request().postDataJSON().directory;
          state.status.job = { id: "storage-default", kind: "storage", running: false };
          return route.fulfill({ json: state.status.job });
        }
        return route.fulfill({ json: { directory, has_environment: false, suggested_directory: directory === "C:/preview/eden-state" ? initialDirectory : undefined } });
      });
      await page.route("**/api/start", route => {
        starts++;
        const submitted = route.request().postDataJSON();
        assert.deepEqual(submitted, { guest_system: system });
        state.status.job = { id: "install-" + starts, running: true, kind: "guest-install", setup_progress: { system, phase: "image", operation: "download", label: "正在下载系统镜像", received: 25 * 1048576, total: 100 * 1048576, bytes_per_second: 1048576 }, log: "镜像下载已开始" };
        return route.fulfill({ json: state.status.job });
      });
      await page.route("**/api/environment/initialization", route => {
        const action = route.request().method() === "POST" ? route.request().postDataJSON()?.action : "";
        if (action === "complete" && rejectComplete) return route.fulfill({ status: 400, contentType: "text/plain; charset=utf-8", body: "环境网络检查尚未通过，请重新检查" });
        if (action === "install") state.initialization.started = true;
        if (action === "complete") state.initialization.required = false;
        return route.fulfill({ json: state.initialization });
      });
    },
  });
  const { page, state, errors, browser } = fixture;
  try {
    assert.equal(await page.getByRole("complementary", { name: "项目与终端" }).count(), 0);
    assert.equal(await page.getByRole("combobox", { name: "客户机系统" }).count(), 0);
    const storage = page.getByRole("region", { name: "环境存储目录" });
    await storage.getByLabel("存储目录", { exact: true }).waitFor();
    assert.equal(await storage.getByLabel("存储目录", { exact: true }).inputValue(), initialDirectory);
    await storage.getByText("这是 Eden 的环境数据目录。更新、重装或卸载应用会保留这里的数据；请勿手动删除。").waitFor();
    assert.equal(await storage.getByText("当前环境目录", { exact: true }).count(), 0);
    assert.equal(await storage.getByText("新存储位置", { exact: true }).count(), 0);
    await page.locator(".initialization-content").evaluate(async element => {
      await Promise.all(element.getAnimations({ subtree: true })
        .filter(animation => animation.animationName === "initialization-enter")
        .map(animation => animation.finished));
    });
    await page.getByText("Debian 13", { exact: true }).waitFor();
    if (evidence) await page.screenshot({ path: `${evidence}/${system}-system.png`, fullPage: true });
    await page.getByRole("button", { name: "下一步：配置代理" }).click();
    await page.getByRole("heading", { name: "连接你的网络代理" }).waitFor();
    await page.getByRole("button", { name: "上一步", exact: true }).click();
    await page.getByText("Debian 13", { exact: true }).waitFor();
    await page.getByRole("button", { name: "下一步：配置代理" }).click();
    await page.getByRole("button", { name: "开始安装", exact: true }).click();
    const progress = page.getByRole("region", { name: "环境安装进度" });
    await progress.getByText("25 MiB / 100 MiB", { exact: true }).waitFor();
    assert.equal(await progress.getByRole("progressbar").getAttribute("aria-valuenow"), "25");
    assert.equal(await page.getByRole("complementary", { name: "项目与终端" }).count(), 0);
    await page.reload();
    await progress.getByText("25 MiB / 100 MiB", { exact: true }).waitFor();
    assert.equal(starts, 1, "refresh must not restart the accepted installation");
    state.status.job = { ...state.status.job, running: false, error: "下载中断，已保存部分文件" };
    await progress.getByRole("alert").waitFor();
    if (evidence) await page.screenshot({ path: `${evidence}/${system}-failed.png`, fullPage: true });
    await progress.getByRole("button", { name: "继续安装", exact: true }).click();
    await progress.getByRole("progressbar").waitFor();
    assert.equal(starts, 2);
    await page.setViewportSize({ width: 600, height: 650 });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    if (evidence) await page.screenshot({ path: `${evidence}/${system}-progress-compact.png`, fullPage: true });
    state.status.job = { id: "install-2", kind: "start", running: false, message: "环境已启动，联网检查未通过", error: "联网检查未通过：浏览器检测通道返回 HTTP 503，外网保持关闭" };
    Object.assign(state.status, { ready: true, vm_running: true, safety: { gate: { mode: "closed" } } });
    await page.getByRole("heading", { name: "环境已启动，等待联网检查", exact: true }).waitFor();
    assert.equal(await page.getByRole("button", { name: "继续安装", exact: true }).count(), 0);
    let networkChecks = 0;
    await page.route("**/api/safety/check", route => {
      networkChecks++;
      state.status.job = { id: "retry-network", kind: "safety", running: true, message: "正在检查联网条件" };
      return route.fulfill({ json: state.status.job });
    });
    await page.getByRole("button", { name: "运行网络与隔离检查", exact: true }).click();
    await page.getByText("正在检查联网条件", { exact: true }).waitFor();
    assert.equal(networkChecks, 1);
    assert.equal(starts, 2, "network recovery must not restart installation");
    state.status.job = { id: "install-2", kind: "start", running: false };
    Object.assign(state.status, { ready: true, vm_running: true, safety: { gate: { mode: "open", until: new Date(Date.now() + 3600000).toISOString() } } });
    await page.getByRole("button", { name: "进入工作台", exact: true }).click();
    await page.getByRole("alert").filter({ hasText: "网络检查" }).waitFor();
    assert.equal(await page.getByRole("complementary", { name: "项目与终端" }).count(), 0, "server completion rejection must keep initialization visible");
    rejectComplete = false;
    await page.getByRole("button", { name: "进入工作台", exact: true }).click();
    await page.getByRole("complementary", { name: "项目与终端" }).waitFor();
    await page.evaluate(() => window.edenNativeAction("settings"));
    await page.getByRole("region", { name: "设置", exact: true }).waitFor();
    await page.reload();
    await page.getByRole("complementary", { name: "项目与终端" }).waitFor();
    await page.evaluate(() => window.edenNativeAction("quit"));
    const quitDialog = page.getByRole("dialog", { name: "退出 Eden", exact: true });
    await quitDialog.getByRole("button", { name: "取消", exact: true }).click();
    assert.equal(await page.getByRole("heading", { name: "为 Eden 准备独立环境" }).count(), 0);
    assert.deepEqual(errors, []);
    console.log(`PASS ${platform}/${system}: fixed guest, back navigation, progress, refresh, retry, completion gate and reopen`);
  } finally { await browser.close(); }
}

{
  let location = { directory: "/preview/default-state", has_environment: false }, writes = 0, fail = true;
  const f = await createWorkbench({ initialization: { required: true, tracked: true }, beforeNavigate: async (page, state) => {
    state.status.local_vm_storage = true;
    await page.route("**/api/environment/storage", route => {
      if (route.request().method() === "GET") return route.fulfill({ json: location });
      const body = route.request().postDataJSON();
      writes++;
      assert.equal(body.previous, "/preview/default-state");
      assert.equal(body.migrate, false);
      if (fail) return route.fulfill({ status: 400, contentType: "text/plain; charset=utf-8", body: "请选择专用空文件夹" });
      location = { ...location, directory: body.directory };
      state.status.job = { id: "save-storage", kind: "storage", running: false };
      return route.fulfill({ json: state.status.job });
    });
  } });
  try {
    const input = f.page.getByLabel("存储目录", { exact: true });
    assert.equal(await input.inputValue(), "/preview/default-state");
    await input.fill("/preview/chosen-state");
    assert.equal(writes, 0, "editing must not save before Next");
    await f.page.getByRole("button", { name: "下一步：配置代理" }).click();
    await f.page.getByRole("alert").filter({ hasText: "请选择专用空文件夹" }).waitFor();
    assert.equal(await input.inputValue(), "/preview/chosen-state");
    assert.equal(await f.page.getByRole("heading", { name: "连接你的网络代理" }).count(), 0);
    fail = false;
    await f.page.getByRole("button", { name: "下一步：配置代理" }).click();
    await f.page.getByRole("heading", { name: "连接你的网络代理" }).waitFor();
    assert.equal(location.directory, "/preview/chosen-state");
    assert.equal(writes, 2);
    await f.page.getByRole("button", { name: "上一步", exact: true }).click();
    assert.equal(await input.inputValue(), "/preview/chosen-state");
    assert.deepEqual(f.errors, []);
    console.log("PASS storage: one default-filled directory, save on Next, rejected path preserved, retry and back navigation");
  } finally { await f.browser.close(); }
}

{
  let imported = false, applied = false;
  const f = await createWorkbench({ initialization: { required: true, tracked: true }, beforeNavigate: async (page, state) => {
    state.status.local_vm_storage = true;
    await page.route("**/api/environment/storage", route => route.fulfill({ json: { directory: "C:/preview/eden-state", has_environment: false } }));
    await page.route("**/api/proxy", route => route.fulfill({ json: { active: applied, selected: applied ? "node" : "", sources: imported ? [{ id: "source", name: "自有节点", nodes: [{ id: "node", name: "本机测试代理", type: "socks5" }] }] : [], port: 17896, core_available: true } }));
    for (const action of ["import", "route"]) await page.route(`**/api/proxy/${action}`, route => {
      if (action === "import") imported = true;
      else applied = true;
      state.status.job = { id: action, kind: "proxy", running: false, message: "已完成" };
      return route.fulfill({ json: state.status.job });
    });
  } });
  try {
    const { page } = f;
    await page.getByRole("button", { name: "下一步：配置代理" }).click();
    const start = page.getByRole("button", { name: "开始安装", exact: true });
    assert.ok(await start.isDisabled());
    await page.getByRole("button", { name: "添加来源", exact: true }).click();
    await page.getByRole("button", { name: "手动节点", exact: true }).click();
    await page.getByLabel("来源名称", { exact: true }).fill("自有节点");
    await page.getByLabel("节点名称", { exact: true }).fill("本机测试代理");
    await page.getByLabel("服务器", { exact: true }).fill("127.0.0.1");
    await page.getByRole("button", { name: "导入并验证", exact: true }).click();
    await page.getByRole("tab", { name: "节点", exact: true }).click();
    await page.getByRole("button", { name: "选择节点 本机测试代理", exact: true }).click();
    await page.getByRole("button", { name: "应用代理", exact: true }).click();
    await page.getByText("已完成", { exact: true }).waitFor();
    await page.waitForFunction(() => [...document.querySelectorAll('button')].some(button => button.textContent === '开始安装' && !button.disabled));
    assert.deepEqual(f.errors, []);
    console.log("PASS first proxy: manual import, node selection and explicit apply unlock installation inside initialization");
  } finally { await f.browser.close(); }
}
