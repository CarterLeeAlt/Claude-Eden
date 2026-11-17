import assert from "node:assert/strict";
import { createWorkbench, versionsView } from "./workbench-fixture.mjs";

for (const system of ["debian-13"]) {
  const calls = [];
  const fixture = await createWorkbench({ beforeNavigate: async (page, state) => {
    state.account.guest_system = state.status.guest_system = system;
    state.status.vm_running = state.status.ready = true;
    await page.route("**/api/environment/versions", route => route.fulfill({ json: versionsView({
      vm_running: true, tools_update_allowed: true,
      components: { status: "update_required", update_required: true },
      controller: { version: "fixture-controller" },
      tools: { claude: { current: "2.1.300", required: "2.1.311" }, node: { current: "v24.22.2", required: "24.22.2" } },
    }) }));
    for (const operation of ["update", "tools"]) {
      await page.route(`**/api/environment/${operation}`, route => {
        calls.push(operation);
        return route.fulfill({ status: 409, contentType: "text/plain; charset=utf-8", body: "组件更新失败，请检查代理连接后重试。" });
      });
    }
  } });
  const { page, browser, errors } = fixture;
  try {
    await page.getByRole("button", { name: "设置", exact: true }).click();
    await page.getByRole("navigation", { name: "设置分组" }).getByRole("button", { name: "环境与恢复", exact: true }).click();
    const panel = page.getByRole("region", { name: "环境版本", exact: true });
    await panel.getByText("2.1.300", { exact: true }).waitFor();
    await panel.getByText("2.1.311", { exact: true }).waitFor();
    for (const operation of ["组件", "工具"]) {
      await panel.getByRole("button", { name: `更新环境${operation}`, exact: true }).click();
      const dialog = page.getByRole("dialog", { name: `更新环境${operation}？`, exact: true });
      const text = await dialog.innerText();
      assert.ok(text.includes("工作台终端、项目文件和登录状态会保留"));
      await dialog.getByRole("button", { name: `更新${operation}`, exact: true }).click();
      await panel.getByRole("alert").filter({ hasText: "组件更新失败" }).waitFor();
    }
    assert.deepEqual(calls, ["update", "tools"]);
    assert.deepEqual(errors, []);
    console.log(JSON.stringify({ system, passed: true, calls }));
  } finally {
    await browser.close();
  }
}
