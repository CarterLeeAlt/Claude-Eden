import assert from "node:assert/strict";
import { createWorkbench, waitForFixture } from "./workbench-fixture.mjs";

const calls = [];
const view = {
  mode: "local", desktop_gateway: true, connection_id: "test-local-view",
  client_platform: "windows", server_platform: "windows",
  connection: { state: "connected" },
};
const { page, browser, errors } = await createWorkbench({
  clientContext: view,
  nativeRequest: async (request) => { calls.push(request); return null; },
});
const quit = () => page.evaluate(() => window.emitTestDesktopEvent("eden-native-action", "quit"));
const exits = () => calls.filter(({ action }) => action === "quitNow").length;
const refused = (route) => route.abort("connectionrefused");
const stops = [];
let releaseBindings;
const bindingsGate = new Promise(resolve => { releaseBindings = resolve; });
page.on("request", (request) => {
  if (new URL(request.url()).pathname === "/api/stop") stops.push(request);
});
try {
  // Lose the gateway while the workbench still shows the previous environment.
  await page.route("**/api/**", refused);
  await page.locator(".connection-banner").waitFor();
  await quit();
  const dialog = page.getByRole("dialog", { name: "退出 Claude Eden" });
  await dialog.getByRole("button", { name: "取消" }).click();
  assert.equal(exits(), 0);
  await quit();
  await dialog.getByRole("button", { name: "仅退出桌面应用" }).click();
  await waitForFixture(() => exits() === 1);
  assert.equal(exits(), 1);

  // Initial context cannot be loaded on reopening the page.
  let bindingsWaiting = false;
  const holdBindings = async route => {
    bindingsWaiting = true;
    await bindingsGate;
    await route.continue();
  };
  await page.route("**/.generated/bindings/**/desktopservice.*", holdBindings);
  await page.reload();
  await page.getByRole("button", { name: "重新加载", exact: true }).waitFor();
  await waitForFixture(() => bindingsWaiting);
  await quit();
  releaseBindings();
  await waitForFixture(() => exits() === 2);
  assert.equal(exits(), 2);
  await page.unroute("**/.generated/bindings/**/desktopservice.*", holdBindings);

  // Gateway is alive, but its selected CLI controller has exited.
  view.connection = { state: "failed", error_code: "connection_lost" };
  await page.unroute("**/api/**", refused);
  await page.reload();
  await page.getByRole("button", { name: "重新连接本机", exact: true }).waitFor();
  await quit();
  await waitForFixture(() => exits() === 3);
  assert.equal(exits(), 3);
  assert.equal(stops.length, 0);
  assert.deepEqual(errors, []);
} catch (error) {
  console.error({ actions: calls.map(call => call.action), errors, body: await page.locator("body").innerText() });
  throw error;
} finally {
  releaseBindings();
  await browser.close();
}
