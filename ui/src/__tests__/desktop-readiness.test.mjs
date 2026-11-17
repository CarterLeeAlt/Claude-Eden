import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";
import { transpileSource } from "./source-module.mjs";

const source = transpileSource(
  readFileSync(new URL("../lib/desktop-runtime.ts", import.meta.url), "utf8"),
);
const readiness = source.slice(
  source.indexOf("async function transportReady"),
  source.indexOf("export function desktopService"),
);
function fixture(native = true) {
  const window = new EventTarget();
  const start = vm.runInNewContext(readiness + "\ntransportReady", {
    window,
    nativeAvailable: () => native,
    setTimeout: () =>
      assert.fail("native readiness must not impose a startup-wide deadline"),
  });
  return { window, start };
}
test("desktop readiness waits for the real transport and ignores an incomplete readiness event", async () => {
  const f = fixture();
  let resolved = false;
  const pending = f.start().then((value) => {
    resolved = true;
    return value;
  });
  f.window.dispatchEvent(new Event("eden-runtime-transport-ready"));
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(resolved, false);
  const transport = { call() {} };
  f.window.edenRuntimeTransport = transport;
  f.window.dispatchEvent(new Event("eden-runtime-transport-ready"));
  assert.equal(await pending, transport);
  assert.equal(await f.start(), transport);
});
test("desktop readiness rejects a non-native context immediately rather than treating elapsed time as failure", async () => {
  const f = fixture(false);
  await assert.rejects(f.start(), /原生窗口连接不可用/);
});
