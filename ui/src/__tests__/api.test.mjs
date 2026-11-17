import test from "node:test";
import assert from "node:assert/strict";
import { normalizeURL } from "../lib/api.ts";
import { createNativeTransport } from "./native-runtime.mjs";
import { getClientContext, setClientContext } from "../lib/client-context.ts";
const settle = () => new Promise((resolve) => setImmediate(resolve));
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
test("address input retains loopback inside guest and rejects active schemes", () => {
  assert.equal(normalizeURL("localhost:3000"), "http://localhost:3000/");
  assert.equal(
    normalizeURL("127.0.0.1:18881/test"),
    "http://127.0.0.1:18881/test",
  );
  assert.equal(normalizeURL("example.com"), "https://example.com/");
  assert.equal(
    normalizeURL("hello eden"),
    "https://www.google.com/search?q=hello%20eden",
  );
  assert.throws(() => normalizeURL("javascript:alert(1)"));
  assert.throws(() => normalizeURL("file:///etc/passwd"));
});

import { api, native, nativeAvailable } from "../lib/api.ts";
function nativeRuntime(t, fetcher, bridge) {
  const saved = {
    fetch: globalThis.fetch,
    window: globalThis.window,
    location: globalThis.location,
  };
  globalThis.fetch = fetcher;
  globalThis.window = {
    edenRuntimeTransport: createNativeTransport(bridge),
  };
  globalThis.location = new URL("http://localhost:17777/");
  t.after(() => {
    for (const [key, value] of Object.entries(saved)) {
      if (value === undefined) delete globalThis[key];
      else globalThis[key] = value;
    }
  });
}
const json = (data, status = 200) =>
  new Response(JSON.stringify(data), { status });
test("Cookie details select their real response contract before the request is sent", async (t) => {
  nativeRuntime(t, async (path, options) => {
    assert.equal(path, "/api/browser/cookies");
    assert.equal(JSON.parse(options.body).mode, "details");
    return json({ value: "测试 Cookie" });
  });
  assert.deepEqual(
    await api("/browser/cookies", { id: "cookie-tab", mode: "details" }),
    { value: "测试 Cookie" },
  );
});
test("an unregistered endpoint is rejected without sending HTTP", async (t) => {
  nativeRuntime(t, () => assert.fail("unknown endpoint must not be sent"));
  await assert.rejects(api("/unknown-test-endpoint", {}), /缺少响应数据契约/);
});
test("leaving a page cancels pending HTTP without a false service-failure report, and returning starts a fresh lifetime", async (t) => {
  let calls = 0;
  nativeRuntime(t, async (path, options) => {
    assert.equal(
      path,
      "/api/diagnostics",
      "page cancellation must not create an error report",
    );
    calls++;
    if (calls === 1)
      return new Promise((_, reject) =>
        options.signal.addEventListener(
          "abort",
          () => reject(options.signal.reason),
          { once: true },
        ),
      );
    assert.equal(options.signal.aborted, false);
    return json({ bytes: 0, files: 0 });
  });
  globalThis.window = new EventTarget();
  const pending = api("/diagnostics");
  window.dispatchEvent(new Event("pagehide"));
  await assert.rejects(pending, (error) => error.name === "AbortError");
  window.dispatchEvent(new Event("pageshow"));
  assert.deepEqual(await api("/diagnostics"), { bytes: 0, files: 0 });
  assert.equal(calls, 2);
});
test("a response body interrupted by page departure is cancellation rather than malformed JSON", async (t) => {
  let reading;
  const started = new Promise((resolve) => {
    reading = resolve;
  });
  nativeRuntime(t, async (path, options) => {
    assert.equal(
      path,
      "/api/diagnostics",
      "body cancellation must not create an error report",
    );
    return {
      ok: true,
      status: 200,
      json: () =>
        new Promise((_, reject) => {
          options.signal.addEventListener(
            "abort",
            () => reject(new TypeError("Failed to fetch")),
            { once: true },
          );
          reading();
        }),
    };
  });
  globalThis.window = new EventTarget();
  const pending = api("/diagnostics");
  await started;
  window.dispatchEvent(new Event("pagehide"));
  await assert.rejects(pending, (error) => error.name === "AbortError");
});
test("aborted session opens preserve their cancellation reason and do not replay", async (t) => {
  const controller = new AbortController();
  const reason = new Error("打开会话超时");
  let calls = 0;
  nativeRuntime(
    t,
    async (_path, options) => {
      calls++;
      assert.equal(options.signal, controller.signal);
      return new Promise((_, reject) =>
        options.signal.addEventListener(
          "abort",
          () => reject(options.signal.reason),
          { once: true },
        ),
      );
    },
    () => {
      throw new Error("unexpected renewal");
    },
  );
  const pending = api(
    "/cli/sessions/test/open",
    {},
    undefined,
    {},
    { signal: controller.signal },
  );
  controller.abort(reason);
  await assert.rejects(pending, (error) => error === reason);
  assert.equal(calls, 1);
});
test("Windows native calls use generated bindings and the official Wails transport", async (t) => {
  const calls = [];
  nativeRuntime(
    t,
    () => {
      throw Error("unexpected request");
    },
    () => {},
  );
  window.edenRuntimeTransport.handle = async (request) => {
    calls.push(request);
    return "clipboard text";
  };
  assert.equal(nativeAvailable(), true);
  assert.equal(await native("clipboardRead"), "clipboard text");
  assert.equal(calls.length, 1);
  assert.equal(calls[0].action, "clipboardRead");
  assert.deepEqual(calls[0].args, []);
});
test("caught startup failures use the generated typed service and validate acknowledgement", async (t) => {
  const calls = [];
  nativeRuntime(t, async () => json({ ok: true }));
  window.edenRuntimeTransport.handle = async (request) => {
    calls.push(request);
    return calls.length === 1 ? true : "invalid acknowledgement";
  };
  const message = "工作台显示异常：测试失败";
  assert.equal(await native("workbenchFailed", { message }), true);
  assert.deepEqual(calls[0], { action: "workbenchFailed", args: [message] });
  await assert.rejects(
    native("workbenchFailed", { message }),
    /返回的数据结构无效/,
  );
});
test("native input owners remain ordered across delayed activation, hide, restore and replacement instances", async (t) => {
  const first = deferred(),
    calls = [];
  let owner = "";
  nativeRuntime(
    t,
    async () => json({ ok: true }),
    async (request) => {
      calls.push([request.operation, request.owner]);
      if (request.owner === "A" && request.operation === "activate")
        await first.promise;
      if (request.operation === "activate") owner = request.owner;
      else if (request.operation === "deactivate" && owner === request.owner)
        owner = "";
      return { accepted: true, returnedFocus: false };
    },
  );
  const activation = native("remoteInput", {
    operation: "activate",
    owner: "A",
  });
  await settle();
  owner = ""; // The native window clears its owner when hidden.
  const cleanup = native("remoteInput", {
    operation: "deactivate",
    owner: "A",
  });
  const replacement = native("remoteInput", {
    operation: "activate",
    owner: "B",
  });
  const caret = native("remoteInput", { operation: "caret", owner: "B" });
  await settle();
  assert.deepEqual(calls, [["activate", "A"]]);
  first.resolve();
  await Promise.all([activation, cleanup, replacement, caret]);
  assert.deepEqual(calls, [
    ["activate", "A"],
    ["deactivate", "A"],
    ["activate", "B"],
    ["caret", "B"],
  ]);
  assert.equal(owner, "B");
});
test("failed input calls release the order, while queued calls keep their original connection identity", async (t) => {
  const first = deferred(),
    calls = [],
    original = getClientContext();
  t.after(() => setClientContext(original));
  nativeRuntime(
    t,
    async () => json({ ok: true }),
    async (request) => {
      calls.push(request.owner);
      if (request.owner === "old") await first.promise;
      return { accepted: true, returnedFocus: false };
    },
  );
  const old = assert.rejects(
    native("remoteInput", { operation: "activate", owner: "old" }),
    /测试输入失败/,
  );
  const stale = assert.rejects(
    native("remoteInput", { operation: "deactivate", owner: "old" }),
    /连接已变化/,
  );
  await settle();
  setClientContext({ ...original, connection_id: "replacement" });
  first.reject(new Error("测试输入失败"));
  await Promise.all([old, stale]);
  await native("remoteInput", { operation: "activate", owner: "new" });
  assert.deepEqual(calls, ["old", "new"]);
});
test("clipboard cancellation completes before new reads and writes without waiting for an old blocked read", async (t) => {
  const oldRead = deferred(),
    cancel = deferred(),
    calls = [];
  let reads = 0;
  nativeRuntime(
    t,
    async () => json({ ok: true }),
    async (request) => {
      calls.push(request.action);
      if (request.action === "clipboardSyncCancel") return cancel.promise;
      if (request.action === "clipboardSyncRead" && ++reads === 1)
        return oldRead.promise;
      if (request.action === "clipboardSyncRead")
        return { revision: 2, files: "new-files" };
      if (request.action === "clipboardSyncWrite") return { revision: 3 };
      return true;
    },
  );
  const old = native("clipboardSyncRead");
  await settle();
  const cancellation = native("clipboardSyncCancel");
  const read = native("clipboardSyncRead"),
    write = native("clipboardSyncWrite", { text: "new", revision: 2 });
  await settle();
  assert.deepEqual(calls, ["clipboardSyncRead", "clipboardSyncCancel"]);
  await native("clipboardSyncRelease", { files: "old-files" });
  cancel.resolve(true);
  await cancellation;
  assert.equal((await read).files, "new-files");
  assert.equal((await write).revision, 3);
  oldRead.resolve({ revision: 1, text: "obsolete" });
  await old;
});
test("overlapping cancels cannot overtake each other or let obsolete queued clipboard work start", async (t) => {
  const first = deferred(),
    second = deferred(),
    calls = [];
  let cancellations = 0;
  nativeRuntime(
    t,
    async () => json({ ok: true }),
    async (request) => {
      calls.push(request.action);
      if (request.action === "clipboardSyncCancel")
        return ++cancellations === 1 ? first.promise : second.promise;
      return { revision: 1, text: "current" };
    },
  );
  const a = native("clipboardSyncCancel");
  const obsolete = assert.rejects(
    native("clipboardSyncWrite", { text: "obsolete", revision: 1 }),
    /剪贴板连接已变化/,
  );
  const b = native("clipboardSyncCancel");
  const current = native("clipboardSyncRead");
  await settle();
  assert.deepEqual(calls, ["clipboardSyncCancel"]);
  first.resolve(true);
  await a;
  await obsolete;
  await settle();
  assert.deepEqual(calls, ["clipboardSyncCancel", "clipboardSyncCancel"]);
  second.resolve(true);
  await b;
  assert.equal((await current).text, "current");
  assert.deepEqual(calls, [
    "clipboardSyncCancel",
    "clipboardSyncCancel",
    "clipboardSyncRead",
  ]);
});
for (const failure of [false, "reject", "invalid"]) {
  test(`clipboard cancel failure (${failure}) blocks new native work until a successful cancel`, async (t) => {
    const calls = [];
    let recover = false;
    nativeRuntime(
      t,
      async () => json({ ok: true }),
      async (request) => {
        calls.push(request.action);
        if (request.action === "clipboardSyncCancel") {
          if (recover) return true;
          if (failure === "reject") throw new Error("测试取消失败");
          return failure === "invalid" ? "invalid" : false;
        }
        return { revision: 1, text: "current" };
      },
    );
    const expected =
      failure === false
        ? /未能取消旧剪贴板操作/
        : failure === "reject"
          ? /测试取消失败/
          : /返回的数据结构无效/;
    await assert.rejects(native("clipboardSyncCancel"), expected);
    await assert.rejects(native("clipboardSyncRead"), expected);
    await assert.rejects(
      native("clipboardSyncWrite", { text: "must not write" }),
      expected,
    );
    assert.deepEqual(calls, ["clipboardSyncCancel"]);
    recover = true;
    await native("clipboardSyncCancel");
    assert.equal((await native("clipboardSyncRead")).text, "current");
  });
}
test("expired concurrent requests renew once and accept the rejected stop exactly once", async (t) => {
  let authorized = false,
    renewals = 0,
    stops = 0,
    exchanges = 0,
    release;
  const entry = new Promise((resolve) => {
    release = resolve;
  });
  nativeRuntime(
    t,
    async (path, options) => {
      if (path === "/api/session") {
        assert.equal(JSON.parse(options.body).token, "one-use-test-ticket");
        authorized = true;
        exchanges++;
        return json({});
      }
      if (!authorized) return new Response("expired", { status: 401 });
      if (path === "/api/stop") {
        stops++;
        return json({
          id: "requested-stop",
          kind: "stop",
          running: true,
          message: "",
          error: "",
          started: "",
          finished: "",
          canceled: false,
        });
      }
      return json({ ok: true });
    },
    async ({ action }) => {
      assert.equal(action, "refreshSession");
      renewals++;
      return entry;
    },
  );
  const requests = Promise.all([api("/stop", {}), api("/account/suspend", {})]);
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(renewals, 1);
  release({ url: "http://localhost:17777/#one-use-test-ticket" });
  const [stop] = await requests;
  assert.equal(stop.id, "requested-stop");
  assert.equal(stops, 1);
  assert.equal(exchanges, 1);
});
test("non-authentication failures never replay a mutation", async (t) => {
  let calls = 0;
  nativeRuntime(
    t,
    async () => {
      calls++;
      return new Response("operation failed", { status: 500 });
    },
    () => {
      throw Error("unexpected renewal");
    },
  );
  await assert.rejects(api("/stop", {}), { status: 500 });
  assert.equal(calls, 1);
});
test("a rejected renewed session has a bounded retry", async (t) => {
  let attempts = 0,
    renewals = 0;
  nativeRuntime(
    t,
    async (path) => {
      if (path === "/api/session") return json({});
      attempts++;
      return new Response("expired", { status: 401 });
    },
    async () => {
      renewals++;
      return { url: "http://localhost:17777/#test-ticket" };
    },
  );
  await assert.rejects(api("/stop", {}), { status: 401 });
  assert.equal(attempts, 2);
  assert.equal(renewals, 1);
});
test("a new control address reconnects without sending a ticket to the old origin", async (t) => {
  const actions = [];
  nativeRuntime(
    t,
    async (path) => {
      assert.equal(path, "/api/stop");
      return new Response("expired", { status: 401 });
    },
    async ({ action }) => {
      actions.push(action);
      return action === "reconnect"
        ? true
        : { url: "http://localhost:17778/#test-ticket" };
    },
  );
  await assert.rejects(api("/stop", {}), /地址已更新/);
  assert.deepEqual(actions, ["refreshSession", "reconnect"]);
});
