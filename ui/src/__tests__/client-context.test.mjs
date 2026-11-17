import test from "node:test";
import assert from "node:assert/strict";
import { api, native } from "../lib/api.ts";
import { runtimeURL, setClientContext } from "../lib/client-context.ts";
import { createNativeTransport, flushNative } from "./native-runtime.mjs";

function environment(t) {
  const saved = {
    fetch: globalThis.fetch,
    window: globalThis.window,
    location: globalThis.location,
  };
  globalThis.location = new URL("http://localhost:17777/");
  globalThis.window = { edenRuntimeTransport: createNativeTransport() };
  setClientContext({ mode: "local", connection_id: "first" });
  t.after(() => {
    setClientContext({ mode: "local", connection_id: "local" });
    for (const [key, value] of Object.entries(saved)) {
      if (value === undefined) delete globalThis[key];
      else globalThis[key] = value;
    }
  });
}

test("an old API result cannot continue an action after the local connection changes", async (t) => {
  environment(t);
  let release;
  globalThis.fetch = (path, options) => {
    assert.equal(path, "/api/input/files/start");
    assert.equal(options.headers["X-Eden-Connection"], "first");
    return new Promise((resolve) => {
      release = resolve;
    });
  };
  const pending = api("/input/files/start", { target: "browser" });
  setClientContext({ mode: "local", connection_id: "second" });
  release(new Response(JSON.stringify({ id: "old-upload" })));
  await assert.rejects(pending, (error) => error.code === "connection_changed");
});

test("a native picker finishing after a connection change cannot yield an upload handle", async (t) => {
  environment(t);
  let release;
  window.edenRuntimeTransport.handle = (request) => {
    assert.equal(request.action, "chooseUploadFiles");
    assert.deepEqual(request.args, [true]);
    return new Promise((resolve) => {
      release = resolve;
    });
  };
  const pending = native("chooseUploadFiles", { multiple: true });
  await flushNative();
  setClientContext({ mode: "local", connection_id: "second" });
  release({ fileSelection: "old-file-handle", count: 1 });
  await assert.rejects(pending, (error) => error.code === "connection_changed");
});

test("quitting before a local connection exists reaches the native bridge", async (t) => {
  environment(t);
  setClientContext({ mode: "none", connection_id: "first" });
  globalThis.fetch = () => {
    throw new Error("quit must not call a runtime API");
  };
  let request;
  window.edenRuntimeTransport.handle = async (value) => {
    request = value;
    return true;
  };
  await native("quitNow");
  assert.deepEqual(request, { action: "quitNow", args: [] });
});

const localGateway = (state) => ({
  mode: "local",
  desktop_gateway: true,
  connection_id: "local-view",
  client_platform: "windows",
  server_platform: "windows",
  connection: {
    state,
    error_code: state === "failed" ? "connection_lost" : "",
  },
});

test("quit still reaches the desktop after the selected CLI controller exits", async (t) => {
  environment(t);
  setClientContext(localGateway("connected"));
  const calls = [];
  let status;
  globalThis.fetch = async (path) => {
    calls.push(path);
    if (path === "/api/client/context")
      return Response.json(localGateway("failed"));
    return new Response("connection unavailable", { status });
  };
  window.edenRuntimeTransport.handle = ({ action }) => {
    calls.push(action);
    return true;
  };
  for (status of [502, 409]) {
    calls.length = 0;
    await native("quitNow");
    assert.deepEqual(calls, [
      "/api/account/suspend",
      "/api/client/context",
      "quitNow",
    ]);
  }
  calls.length = 0;
  setClientContext(localGateway("failed"));
  await native("quitNow");
  assert.deepEqual(calls, ["quitNow"]);
});

test("a disconnected view from another generation cannot authorize a pending quit", async (t) => {
  environment(t);
  setClientContext(localGateway("connected"));
  globalThis.fetch = async (path) =>
    path === "/api/client/context"
      ? Response.json({
          ...localGateway("failed"),
          connection_id: "replacement",
        })
      : new Response("view changed", { status: 409 });
  window.edenRuntimeTransport.handle = () => assert.fail("stale quit");
  await assert.rejects(native("quitNow"), { status: 409 });
});

test("quit still reaches the desktop when the CLI HTTP listener is gone", async (t) => {
  environment(t);
  setClientContext({ mode: "local", connection_id: "local" });
  globalThis.fetch = async (path) => {
    if (path === "/api/diagnostics/events") return Response.json({});
    assert.equal(path, "/api/account/suspend");
    throw new TypeError("Failed to fetch");
  };
  const actions = [];
  window.edenRuntimeTransport.handle = ({ action }) => {
    actions.push(action);
    return true;
  };
  await native("quitNow");
  assert.deepEqual(actions, ["quitNow"]);
});

test("an offline quit cannot cross a connection change during account suspension", async (t) => {
  environment(t);
  setClientContext(localGateway("connected"));
  let failRequest;
  globalThis.fetch = (path) => {
    if (path === "/api/diagnostics/events")
      return Promise.resolve(Response.json({}));
    return new Promise((resolve, reject) => {
      failRequest = reject;
    });
  };
  window.edenRuntimeTransport.handle = () => assert.fail("stale quit");
  const quitting = native("quitNow");
  setClientContext({
    ...localGateway("connected"),
    connection_id: "replacement",
  });
  failRequest(new TypeError("Failed to fetch"));
  await assert.rejects(quitting, { code: "connection_changed" });
});

test("a reachable local controller must lock or report already locked before quit", async (t) => {
  environment(t);
  setClientContext(localGateway("connected"));
  window.dispatchEvent = () => {};
  const calls = [];
  let status = 500;
  globalThis.fetch = async (path) => {
    calls.push(path);
    if (path === "/api/client/context")
      return Response.json(localGateway("connected"));
    return Response.json({}, { status });
  };
  window.edenRuntimeTransport.handle = ({ action }) => {
    calls.push(action);
    return true;
  };
  for (status of [500, 409, 502]) {
    await assert.rejects(native("quitNow"), { status });
    assert.ok(!calls.includes("quitNow"));
  }
  for (status of [200, 423]) {
    calls.length = 0;
    await native("quitNow");
    assert.deepEqual(calls, ["/api/account/suspend", "quitNow"]);
  }
});

test("a clipboard operation retains its connection generation and rejects a late result", async (t) => {
  environment(t);
  let release;
  window.edenRuntimeTransport.handle = (request) => {
    assert.deepEqual(request, { action: "clipboardRead", args: [] });
    return new Promise((resolve) => {
      release = resolve;
    });
  };
  const pending = native("clipboardRead");
  await flushNative();
  setClientContext({ mode: "local", connection_id: "second" });
  release("clipboard from the previous connection");
  await assert.rejects(pending, (error) => error.code === "connection_changed");
});

test("stream URLs carry the current local connection generation", (t) => {
  environment(t);
  assert.equal(
    runtimeURL("/browser/events?after=1"),
    "/api/browser/events?after=1&connection=first",
  );
  setClientContext({ mode: "local", connection_id: "reconnected" });
  assert.equal(
    runtimeURL("/browser/events?after=1"),
    "/api/browser/events?after=1&connection=reconnected",
  );
});
