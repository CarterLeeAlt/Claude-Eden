import assert from "node:assert/strict";
import { createWorkbench } from "./workbench-fixture.mjs";

let exports = 0;
const { browser, page, state, errors } = await createWorkbench({
  viewport: { width: Number(process.env.EDEN_STORAGE_WIDTH || 1280), height: 850 },
  clientContext: { mode: "local", connection_id: "local", client_platform: "windows", server_platform: "windows", connection: { state: "disconnected" } },
  nativeRequest: async ({ action }) => {
    if (action === "exportDiagnostics") { exports++; return { saved: true }; }
    return null;
  },
});
try {
  const attempts = [];
  let location = null;
  state.account.guest_system = "debian-13";
  state.account.unlocked = false;
  state.account.require_authentication = true;
  Object.assign(state.status, {
    local_vm_storage: true, guest_system: "debian-13", ready: false, vm_running: false,
    storage_recovery: { required: true, error: "无法访问环境目录，请连接迁移使用的磁盘" },
  });
  await page.route("**/api/environment/storage", route => {
    assert.equal(route.request().method(), "GET", "pending recovery admitted a migration");
    return route.fulfill({ json: location || { recovery: state.status.storage_recovery } });
  });
  await page.route("**/api/environment/storage/recover", route => {
    attempts.push(route.request().postDataJSON());
    state.status.job = {
      id: `storage-recovery-${attempts.length}`, kind: "storage-recovery", running: true,
      message: "正在恢复环境存储并检查数据位置…",
    };
    return route.fulfill({ json: state.status.job });
  });
  await page.route("**/api/diagnostics", route => route.fulfill({ json: { bytes: 2048, files: 2 } }));
  await page.reload();
  await page.getByRole("region", { name: "解锁 Claude Eden", exact: true }).waitFor();
  await page.getByLabel("账户密码", { exact: true }).fill("fixture-password");
  await page.getByRole("button", { name: "解锁 Claude Eden", exact: true }).click();
  await page.getByRole("heading", { name: "环境存储需要恢复", exact: true }).waitFor();
  assert.equal(await page.getByRole("button", { name: "启动 Claude Eden 环境", exact: true }).count(), 0);
  await page.getByRole("button", { name: "恢复环境存储", exact: true }).click();
  const panel = page.getByRole("region", { name: "本机环境存储", exact: true });
  const retry = panel.getByRole("button", { name: "重试恢复", exact: true });
  await retry.waitFor();
  await panel.getByText(state.status.storage_recovery.error, { exact: true }).waitFor();
  assert.equal(await panel.getByLabel("新存储位置").count(), 0);
  assert.equal(await page.getByRole("region", { name: "环境版本", exact: true }).count(), 0);

  await retry.click();
  await panel.getByRole("status").waitFor();
  assert.deepEqual(attempts, [{}]);
  assert.equal(await retry.isDisabled(), true);
  await retry.evaluate(button => { button.click(); button.click(); });
  assert.equal(attempts.length, 1, "disabled recovery button submitted the same operation twice");
  await page.getByRole("button", { name: "导出诊断包", exact: true }).click();
  await page.getByText("诊断包已保存。分享前请检查其中内容。", { exact: true }).waitFor();
  assert.equal(exports, 1, "recovery blocked diagnostic export");

  state.status.storage_recovery.error = "环境目录仍不可读，请检查原磁盘连接后重试";
  state.status.job = { ...state.status.job, running: false, error: state.status.storage_recovery.error };
  await panel.getByText(state.status.storage_recovery.error, { exact: true }).waitFor();
  await page.waitForFunction(() => [...document.querySelectorAll(".environment-storage button")].every(button => !button.disabled));
  await retry.click();
  await panel.getByRole("status").waitFor();
  assert.deepEqual(attempts, [{}, {}]);
  assert.equal(await retry.isDisabled(), true);
  const geometry = await panel.evaluate(element => {
    const actions = element.querySelector(".storage-actions").getBoundingClientRect();
    const progress = element.querySelector('[role="status"]').getBoundingClientRect();
    return { actionsBottom: actions.bottom, progressTop: progress.top };
  });
  assert.ok(geometry.progressTop >= geometry.actionsBottom + 8, "recovery progress overlaps its controls");
  if (process.env.EDEN_STORAGE_SCREENSHOT) {
    await panel.scrollIntoViewIfNeeded();
    await page.screenshot({ path: process.env.EDEN_STORAGE_SCREENSHOT });
  }
  assert.equal(state.requests.filter(request => request.path === "/start").length, 0);

  location = { directory: "D:\\Eden 环境", has_environment: true, custom: true, backup: "C:\\Users\\demo\\.eden\\state" };
  state.status.storage_recovery = { required: false };
  state.status.job = { ...state.status.job, running: false, error: "", message: "环境存储已恢复，可以启动环境" };
  await panel.getByText(location.directory, { exact: true }).waitFor();
  await page.waitForFunction(() => !document.querySelector(".environment-storage input")?.disabled);
  assert.equal(await retry.count(), 0);
  await page.getByRole("button", { name: "返回工作台", exact: true }).first().click();
  await page.getByRole("button", { name: "启动 Claude Eden 环境", exact: true }).click();
  await page.waitForFunction(() => document.querySelector(".welcome h1")?.textContent === "Claude Eden 环境已就绪");
  assert.equal(state.requests.filter(request => request.path === "/start").length, 1);
  assert.deepEqual(errors, []);
  console.log(`Storage recovery login, settings, failure cause, duplicate rejection, diagnostics, retry, preparation and normal start passed (${page.viewportSize().width}px)`);
} finally { await browser.close(); }
