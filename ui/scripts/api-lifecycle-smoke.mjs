import assert from "node:assert/strict";
import { createWorkbench, waitForFixture } from "./workbench-fixture.mjs";

let release, started = false, blocked = true;
const gate = new Promise(resolve => { release = resolve; });
const fixture = await createWorkbench({ beforeNavigate: async page => {
  await page.route("**/api/diagnostics", async route => {
    started = true;
    if (blocked) await gate;
    await route.fulfill({ json: { bytes: 0, files: 0 } });
  });
} });
try {
  const { page, errors } = fixture;
  await page.waitForLoadState("networkidle");
  await page.evaluate(async () => {
    const { api } = await import("/src/lib/api.ts");
    void api("/diagnostics").catch(error => {
      if (error.name !== "AbortError") throw error;
    });
  });
  await waitForFixture(() => started);
  await page.reload();
  await page.getByRole("heading", { name: "先启动 Claude Eden 环境", exact: true }).waitFor();
  blocked = false;
  release();
  assert.deepEqual(await page.evaluate(async () => {
    const { api } = await import("/src/lib/api.ts");
    return api("/diagnostics");
  }), { bytes: 0, files: 0 });
  await page.waitForLoadState("networkidle");
  assert.deepEqual(errors, []);
  console.log("PASS: real page departure cancels a pending API request without a false failure, and the reloaded page uses a fresh request lifetime");
} finally {
  release();
  await fixture.browser.close();
}
