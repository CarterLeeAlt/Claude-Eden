import assert from "node:assert/strict";
import { createWorkbench } from "./workbench-fixture.mjs";

{
  for (const [host, accountSystem, statusSystem, label] of [
    ["windows", "debian-13", undefined, "Debian 13"],
    ["windows", undefined, "debian-13", "Debian 13"],
    ["windows", undefined, undefined, "Debian 13"],
  ]) {
    const { browser, page, state, errors } = await createWorkbench({
      clientContext: { mode: "local", connection_id: "local", client_platform: host, server_platform: host },
    });
    try {
      state.account.guest_system = accountSystem;
      state.status.guest_system = statusSystem;
      await page.reload();
      const sidebar = page.getByRole("complementary", { name: "项目与终端" });
      const icon = sidebar.getByRole("img", { name: `客户机系统：${label}` });
      await icon.waitFor();
      assert.equal(await icon.getAttribute("title"), `客户机系统：${label}`);
      const iconBox = await icon.boundingBox();
      const brandBox = await sidebar.locator(".sidebar-brand").boundingBox();
      const collapseBox = await sidebar.getByRole("button", { name: "收起侧栏" }).boundingBox();
      assert.ok(iconBox.x > brandBox.x && iconBox.x + iconBox.width < collapseBox.x);
      if (process.env.EDEN_TEST_CAPTURE) await page.screenshot({ path: `${process.env.EDEN_TEST_CAPTURE}-${label}.png` });
      await page.reload();
      await icon.waitFor();
      assert.deepEqual(errors, []);
      console.log(`PASS 客机 ${label}，客户端 ${host}`);
    } finally { await browser.close(); }
  }
  const fixture = await createWorkbench({
    beforeNavigate: async (page, state) => {
      state.status.local_vm_storage = true;
    },
  });
  try {
    const { page } = fixture;
    await page.getByRole("img", { name: "客户机系统：Debian 13" }).waitFor();
    assert.equal(await page.getByRole("combobox", { name: "客户机系统" }).count(),0);
    assert.deepEqual(fixture.errors, []);
    console.log("PASS 首次安装固定 Debian 13 图标");
  } finally { await fixture.browser.close(); }
}
