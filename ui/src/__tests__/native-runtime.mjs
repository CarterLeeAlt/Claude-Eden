import { readFileSync } from "node:fs";
import assert from "node:assert/strict";
import { setTransport } from "@wailsio/runtime";

const source = readFileSync(
  new URL(
    "../../.generated/bindings/github.com/sylvinus/agent-vm/internal/desktop/desktopservice.ts",
    import.meta.url,
  ),
  "utf8",
);
const methods = new Map(
  [...source.matchAll(/export function (\w+)\([^]*?\$Call\.ByID\((\d+)/g)].map(
    ([, name, id]) => [Number(id), name[0].toLowerCase() + name.slice(1)],
  ),
);

// Exercise the real generated bindings and official custom transport; only the OS service is replaced.
export function createNativeTransport(handle = async () => true) {
  const transport = {
    handle,
    async call(objectID, method, windowName, request) {
      assert.equal(objectID, 0);
      assert.equal(method, 0);
      assert.equal(windowName, "");
      assert.ok(
        methods.has(request.methodID),
        "method must come from generated DesktopService bindings",
      );
      const action = methods.get(request.methodID);
      return transport.handle({
        action,
        args: request.args,
        ...(typeof request.args[0] === "object" ? request.args[0] : {}),
      });
    },
  };
  setTransport(transport);
  return transport;
}

export const flushNative = () =>
  new Promise((resolve) => setImmediate(resolve));
