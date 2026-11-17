import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { join } from "node:path";
import { createWorkbench, waitForFixture } from "./workbench-fixture.mjs";

// Fault injection replaces only the lazy app module. The actual error boundary,
// generated DesktopService and official Wails runtime handle the failure.
for (const rejectReport of [false, true]) {
  const calls = [];
  const fixture = await createWorkbench({
    waitForReady: false,
    nativeRequest: request => {
      calls.push(request);
      if (request.action !== "workbenchFailed") return;
      if (rejectReport) throw new Error("测试：启动结果未能保存");
      return true;
    },
    beforeNavigate: page => page.route("**/src/app/App.tsx", route => route.fulfill({
      contentType: "text/javascript",
      body: 'throw new Error("测试：工作台模块初始化失败");',
    })),
  });
  const { page, browser } = fixture;
  try {
    await page.getByRole("heading", { name: "工作台显示遇到异常", exact: true }).waitFor();
    await waitForFixture(() => calls.some(request => request.action === "workbenchFailed"));
    const failures = calls.filter(request => request.action === "workbenchFailed");
    assert.equal(failures.length, 1);
    assert.match(failures[0].message, /工作台模块初始化失败/);
    assert.equal(calls.some(request => request.action === "workbenchReady"), false);
    await page.getByRole("button", { name: "导出诊断包", exact: true }).waitFor();
    if (rejectReport)
      await page.getByRole("alert").filter({ hasText: "未能报告启动失败：测试：启动结果未能保存" }).waitFor();
    else assert.equal(await page.getByRole("alert").count(), 0);
    await page.getByRole("button", { name: "导出诊断包", exact: true }).click();
    await waitForFixture(() => calls.some(request => request.action === "exportDiagnostics"));
  } finally {
    await browser.close();
  }
}
for (const endpoint of ["session", "settings"]) {
  for (const report of ["accepted", "rejected", "declined"]) {
    const calls = [];
    let fail = true;
    const fixture = await createWorkbench({
      waitForReady: false,
      url: new URL("#startup-ticket", process.env.EDEN_UI_URL || "http://127.0.0.1:5198").href,
      nativeRequest: request => {
        calls.push(request);
        if (request.action !== "workbenchFailed") return;
        if (report === "rejected") throw new Error("测试：启动结果未能保存");
        return report === "accepted";
      },
      beforeNavigate: page => page.route(`**/api/${endpoint}`, route => fail ? route.fulfill({
        status: 500, contentType: "text/plain; charset=utf-8", body: `测试：首次 ${endpoint} 初始化失败`,
      }) : route.fallback()),
    });
    const { page, browser, errors } = fixture;
    try {
      await page.getByRole("heading", { name: endpoint === "session" ? "连接本机工作台" : "连接 Claude Eden 工作台", exact: true }).waitFor();
      await page.getByRole("alert").filter({ hasText: `首次 ${endpoint} 初始化失败` }).waitFor();
      await waitForFixture(() => calls.some(request => request.action === "workbenchFailed"));
      const failures = calls.filter(request => request.action === "workbenchFailed");
      assert.equal(failures.length, 1);
      assert.match(failures[0].message, new RegExp(`工作台初始化失败：.*首次 ${endpoint} 初始化失败`));
      assert.equal(calls.some(request => request.action === "workbenchReady"), false);
      if (report !== "accepted")
        await page.getByRole("alert").filter({ hasText: report === "rejected" ? "未能报告启动失败：测试：启动结果未能保存" : "未能报告启动失败：原生窗口未接受启动失败报告。" }).waitFor();
      else assert.equal(await page.getByRole("alert").count(), 1);
      if (process.env.EDEN_STARTUP_EVIDENCE && report === "rejected") {
        await mkdir(process.env.EDEN_STARTUP_EVIDENCE, { recursive: true });
        await page.screenshot({ path: join(process.env.EDEN_STARTUP_EVIDENCE, `${endpoint}.png`) });
      }
      if (endpoint === "session") {
        fail = false;
        await page.getByRole("button", { name: "重新加载", exact: true }).click();
        await page.getByRole("heading", { name: "先启动 Claude Eden 环境", exact: true }).waitFor();
        await waitForFixture(() => calls.some(request => request.action === "workbenchReady"));
      } else {
        await page.getByRole("button", { name: "重新连接", exact: true }).click();
        await waitForFixture(() => calls.some(request => request.action === "reconnect"));
      }
      assert.deepEqual(errors, []);
    } finally { await browser.close(); }
  }
}

{
  const calls = [];
  let fail = true;
  const { page, browser, errors } = await createWorkbench({
    waitForReady: false,
    nativeRequest: request => { calls.push(request); },
    beforeNavigate: page => page.route("**/api/status", route => fail ? route.fulfill({
      status: 503, contentType: "text/plain; charset=utf-8", body: "测试：工作区状态暂时不可用",
    }) : route.fallback()),
  });
  try {
    await page.getByRole("alert").filter({ hasText: "工作区状态暂时不可用" }).waitFor();
    assert.equal(calls.some(request => ["workbenchFailed", "workbenchReady"].includes(request.action)), false);
    if (process.env.EDEN_STARTUP_EVIDENCE) {
      await mkdir(process.env.EDEN_STARTUP_EVIDENCE, { recursive: true });
      await page.screenshot({ path: join(process.env.EDEN_STARTUP_EVIDENCE, "status.png") });
    }
    fail = false;
    await page.getByRole("button", { name: "重试连接", exact: true }).click();
    await page.getByRole("heading", { name: "先启动 Claude Eden 环境", exact: true }).waitFor();
    await waitForFixture(() => calls.some(request => request.action === "workbenchReady"));
    fail = true;
    await page.evaluate(() => window.emitTestDesktopEvent("eden-window-visibility", true));
    await page.getByRole("alert").filter({ hasText: "工作台连接中断，后台状态暂时未知。" }).waitFor();
    assert.equal(calls.filter(request => request.action === "workbenchReady").length, 1);
    assert.equal(calls.some(request => request.action === "workbenchFailed"), false);
    fail = false;
    await page.evaluate(() => window.emitTestDesktopEvent("eden-window-visibility", false));
    await page.evaluate(() => window.emitTestDesktopEvent("eden-window-visibility", true));
    await page.getByRole("alert").filter({ hasText: "工作台连接中断，后台状态暂时未知。" }).waitFor({ state: "hidden" });
    assert.deepEqual(errors, []);
  } finally { await browser.close(); }
}
console.log("PASS: lazy-module and first session/settings failures report once through generated bindings; report rejection is visible; status failures stay recoverable and never become initialization failures");
