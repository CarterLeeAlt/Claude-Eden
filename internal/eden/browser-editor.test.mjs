import test from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import { setTimeout as delay } from "node:timers/promises";
import { authenticateEditor } from "./browser-editor.mjs";

function fixture(cookies = [], checked = 302, login = 302, header = "code-server-session=fresh; HttpOnly; SameSite=Lax") {
  const calls = [], requests = [];
  const cdp = async (method, params, session) => {
    calls.push({ method, params, session });
    if (method === "Storage.getCookies") {
      assert.equal(session, undefined, "cookie inspection must not await a restored renderer");
      return { cookies };
    }
    assert.equal(method, "Storage.setCookies");
    return {};
  };
  const request = async (url, options) => {
    assert.equal(url, "http://127.0.0.1:18080/login");
    assert.equal(options.redirect, "manual");
    requests.push(options);
    return new Response(null, { status: options.method === "POST" ? login : checked, headers: options.method === "POST" ? { "Set-Cookie": header } : {} });
  };
  return { cdp, request, calls, requests };
}
const existing = { name: "code-server-session", value: "prior", domain: "127.0.0.1", path: "/" };

test("a restored document without its session cookie authenticates before reuse", async () => {
  const f = fixture();
  assert.equal(await authenticateEditor(f.cdp, "private-fixture", { request: f.request }), true);
  assert.equal(f.requests.length, 1);
  assert.equal(f.requests[0].body.get("password"), "private-fixture");
  assert.deepEqual(f.calls[1].params.cookies, [{ name: existing.name, value: "fresh", url: "http://127.0.0.1:18080/", path: "/", httpOnly: true, sameSite: "Lax" }]);
});
test("a valid editor session keeps the live document without renewing authentication", async () => {
  const f = fixture([existing, { ...existing, name: "other", value: "unrelated" }]);
  assert.equal(await authenticateEditor(f.cdp, "private-fixture", { request: f.request }), false);
  assert.deepEqual(f.requests[0].headers, { Cookie: "code-server-session=prior" });
  assert.equal(f.requests.length, 1); assert.equal(f.calls.length, 1);
});
test("an expired session is replaced before its document is rebound", async () => {
  const f = fixture([existing], 200);
  assert.equal(await authenticateEditor(f.cdp, "private-fixture", { request: f.request }), true);
  assert.equal(f.requests.length, 2); assert.equal(f.calls.length, 2);
});
test("new editors use the shared default cookie store without attaching a page", async () => {
  const f = fixture();
  assert.equal(await authenticateEditor(f.cdp, "private-fixture", { request: f.request }), true);
  assert.deepEqual(f.calls.map(item => item.method), ["Storage.getCookies", "Storage.setCookies"]);
});
test("server and login failures never publish cookies or claim a ready session", async () => {
  for (const f of [fixture([existing], 500), fixture([], 302, 200), fixture([], 302, 302, "unrelated=value"), fixture([], 302, 302, "code-server-session=")]) {
    await assert.rejects(authenticateEditor(f.cdp, "private-fixture", { request: f.request }), /authentication/);
    assert.ok(f.calls.every(item => item.method !== "Storage.setCookies"));
  }
});

test("startup authentication waits for a slow editor and remains cancellable", { timeout: 10000 }, async () => {
  let arrived;
  const server = http.createServer((request, response) => { arrived(response); });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  const request = (_url, options) => fetch(`http://127.0.0.1:${server.address().port}/login`, options);
  try {
    for (const cancel of [false, true]) {
      const accepted = new Promise(resolve => { arrived = resolve; });
      const controller = new AbortController();
      const f = fixture();
      const pending = authenticateEditor(f.cdp, "private-fixture", { request, signal: controller.signal });
      const response = await accepted;
      if (cancel) {
        controller.abort();
        await assert.rejects(pending, { name: "AbortError" });
        assert.ok(f.calls.every(item => item.method !== "Storage.setCookies"));
      } else {
        await delay(5200);
        assert.equal(controller.signal.aborted, false);
        response.writeHead(302, { "Set-Cookie": "code-server-session=fresh; HttpOnly; SameSite=Lax" });
        response.end();
        assert.equal(await pending, true);
      }
    }
  } finally {
    server.closeAllConnections();
    await new Promise(resolve => server.close(resolve));
  }
});
