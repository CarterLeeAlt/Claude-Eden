import test from "node:test";
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { responseSchemaFor, parseResponse } from "../lib/contracts.ts";
import { nativeResults } from "../lib/native-contracts.ts";
import { browserCookies } from "../../../internal/eden/browser-cookies.mjs";

test("every literal production API call has a response contract before it can send a request", async () => {
  const missing = [];
  async function inspect(directory) {
    for (const item of await readdir(directory, { withFileTypes: true })) {
      if (item.name === "__tests__") continue;
      const file = join(directory, item.name);
      if (item.isDirectory()) {
        await inspect(file);
        continue;
      }
      if (!/\.(ts|tsx)$/.test(file)) continue;
      const source = ts.createSourceFile(
        file,
        await readFile(file, "utf8"),
        ts.ScriptTarget.Latest,
        true,
      );
      function visit(node) {
        if (
          ts.isCallExpression(node) &&
          ts.isIdentifier(node.expression) &&
          node.expression.text === "api" &&
          ts.isStringLiteral(node.arguments[0])
        ) {
          const path = node.arguments[0].text;
          const body = node.arguments[1];
          const method =
            node.arguments[2] && ts.isStringLiteral(node.arguments[2])
              ? node.arguments[2].text
              : body && body.getText(source) !== "undefined"
                ? "POST"
                : "GET";
          if (!responseSchemaFor(path, method))
            missing.push(`${file}: ${method} ${path}`);
        }
        ts.forEachChild(node, visit);
      }
      visit(source);
    }
  }
  await inspect(fileURLToPath(new URL("..", import.meta.url)));
  assert.deepEqual(missing, []);
});

test("read and mutation contracts for shared paths remain distinct", () => {
  const job = {
    id: "job",
    kind: "settings",
    running: true,
    message: "正在保存",
  };
  assert.deepEqual(
    parseResponse(responseSchemaFor("/settings", "POST"), job, "/settings"),
    job,
  );
  assert.equal(
    responseSchemaFor("/settings", "GET").safeParse(job).success,
    false,
  );
  assert.equal(
    responseSchemaFor("/environment/storage", "GET").safeParse(job).success,
    false,
  );
  assert.equal(
    responseSchemaFor("/environment/storage", "POST").safeParse(job).success,
    true,
  );
  assert.equal(
    responseSchemaFor("/terminals", "GET").safeParse([]).success,
    true,
  );
  assert.equal(
    responseSchemaFor("/terminals", "POST").safeParse([]).success,
    false,
  );
  assert.equal(responseSchemaFor("/unknown", "POST"), undefined);
});

test("vault browser status, unavailable service and mutation acknowledgements follow their distinct Go replies", () => {
  const state = responseSchemaFor("/vault/browser", "POST", {
    action: "status",
  });
  const unavailable = {
    available: false,
    message: "浏览器尚未启动",
    entries: [],
    candidates: [],
  };
  assert.deepEqual(state.parse(unavailable), {
    ...unavailable,
    origin: "",
    document: "",
  });
  assert.equal(state.safeParse({ ok: true }).success, false);
  for (const action of ["fill", "save", "never", "dismiss"])
    assert.deepEqual(
      responseSchemaFor("/vault/browser", "POST", { action }).parse({
        ok: true,
      }),
      { ok: true },
    );
});

test("environment storage accepts the Go recovery-only reply while requiring location fields outside recovery", () => {
  const schema = responseSchemaFor("/environment/storage", "GET");
  const recovery = { recovery: { required: true, error: "原磁盘未连接" } };
  assert.deepEqual(schema.parse(recovery), recovery);
  const location = { directory: "D:/Eden_Data", has_environment: true };
  assert.deepEqual(schema.parse(location), location);
  for (const invalid of [
    {},
    { directory: "D:/Eden_Data" },
    { recovery: { required: false } },
  ])
    assert.equal(schema.safeParse(invalid).success, false);
});

test("the real fresh-account status accepts an undetected location as null and validates detected locations", () => {
  // server.go emits s.location (*Location), which is nil before a location probe.
  const status = {
    auto_timezone: true,
    can_restore: false,
    environment_directory: "/workspace/profile-downloads",
    environment_identity_directory: "/workspace/profile-downloads",
    environment_update_error: "",
    environment_update_required: false,
    guest_system: "",
    job: { id: "", kind: "", running: false, message: "" },
    keyboard: "native",
    local_vm_storage: true,
    location: null,
    network_error: "",
    project_error: "",
    project_errors: null,
    project_groups: [],
    projects: [],
    ready: false,
    safety: { verified_at: "0001-01-01T00:00:00Z" },
    storage_recovery: { required: false },
    vm_running: false,
    workspace: "",
  };
  const schema = responseSchemaFor("/status", "GET");
  assert.equal(schema.parse(status).location, null);
  const location = {
    ip: "192.0.2.1",
    country: "Test",
    country_code: "US",
    region: "",
    city: "",
    timezone: "Etc/UTC",
    checked_at: "2026-11-17T00:00:00Z",
  };
  assert.deepEqual(schema.parse({ ...status, location }).location, location);
  for (const invalid of [undefined, {}, "unknown", 0, { ...location, ip: 42 }])
    assert.equal(
      schema.safeParse({ ...status, location: invalid }).success,
      false,
    );
});

test("native results require the actual DesktopService JSON contract", () => {
  assert.equal(nativeResults.remoteInput.safeParse(true).success, false);
  assert.equal(
    nativeResults.remoteInput.safeParse({
      accepted: true,
      returnedFocus: false,
    }).success,
    true,
  );
  assert.equal(
    nativeResults.remoteInput.safeParse({ returnedFocus: true }).success,
    false,
  );
  assert.equal(
    nativeResults.appUpdate.safeParse({ state: "idle" }).success,
    false,
  );
  assert.equal(
    nativeResults.appUpdate.safeParse({
      phase: "idle",
      currentVersion: "1.0",
      message: "",
      retryable: false,
      retryAt: "",
    }).success,
    true,
  );
  assert.equal(
    nativeResults.clipboardReadContent.safeParse({
      revision: 1,
      fileSelection: "selection",
      count: 2,
    }).success,
    true,
  );
});

test("proxy status retains the Go store's opaque string revision and rejects numeric replacements", () => {
  const proxy = {
    nodes: [],
    selected: "",
    port: 17896,
    running: false,
    active: false,
    state: "unconfigured",
    state_label: "尚未配置",
    network_open: false,
    core_available: true,
    version: "",
    revision: "",
    bootstrap_dns: "",
    udp_blocked: true,
    ipv6_blocked: true,
  };
  const schema = responseSchemaFor("/proxy", "GET");
  for (const revision of ["", "opaque-proxy-revision"])
    assert.equal(schema.parse({ ...proxy, revision }).revision, revision);
  for (const revision of [undefined, null, 0, 1, {}])
    assert.equal(schema.safeParse({ ...proxy, revision }).success, false);
});

test("guest IME caret rectangles survive the API boundary, including an unavailable rectangle", () => {
  const schema = responseSchemaFor("/input", "POST");
  for (const rect of [[240, 120, 0, 24], null]) {
    const value = { available: true, context: "a".repeat(32), rect };
    assert.deepEqual(schema.parse(value), value);
  }
  assert.deepEqual(schema.parse({ available: false }), { available: false });
  for (const rect of [[1, 2, 3], [1, 2, 3, "4"], "unknown"])
    assert.equal(schema.safeParse({ available: true, rect }).success, false);
});

test("the actual guest Cookie handler's list, details, deletion and clear results match their contracts", async () => {
  let records = [
    {
      name: "session",
      value: "测试值",
      domain: "example.test",
      path: "/",
      size: 10,
      session: true,
      expires: -1,
      secure: true,
      httpOnly: true,
    },
  ];
  const cdp = async (method) => {
    if (method === "Storage.getCookies") return { cookies: records };
    if (["Network.deleteCookies", "Storage.clearCookies"].includes(method)) {
      records = [];
      return {};
    }
    throw new Error(`Unexpected CDP method: ${method}`);
  };
  const tab = { targetId: "cookie-test", url: "https://example.test/" };
  const input = { scope: "all", mode: "list", id: tab.targetId };
  const request = async (body) =>
    parseResponse(
      responseSchemaFor("/browser/cookies", "POST", body),
      await browserCookies(cdp, async () => "fixture-session", tab, body),
      "/browser/cookies",
    );
  const list = await request(input);
  assert.equal(list.cookies.length, 1);
  assert.equal("value" in list.cookies[0], false);
  assert.deepEqual(
    await request({
      ...input,
      mode: "details",
      cookieKey: list.cookies[0].key,
    }),
    { value: "测试值" },
  );
  assert.equal(
    (
      await request({
        ...input,
        mode: "delete",
        cookieKey: list.cookies[0].key,
      })
    ).total,
    0,
  );
  assert.equal((await request({ ...input, mode: "clear" })).total, 0);
});

test("Tailscale accepts its current Go state and missing-component state without inventing a running field", () => {
  const schema = responseSchemaFor("/tailscale", "GET");
  const stopped = {
    installed: true,
    version: "1.104.5",
    enabled: false,
    backend: "Stopped",
    ips: [],
    peers: [],
    keys: [],
    user: "developer",
    ssh_port: 22,
    forwards: null,
  };
  assert.equal(schema.parse(stopped).backend, "Stopped");
  assert.deepEqual(schema.parse(stopped).forwards, []);
  const missing = {
    installed: false,
    enabled: false,
    backend: "NotInstalled",
    ips: [],
    peers: [],
    keys: [],
  };
  assert.equal(schema.parse(missing).installed, false);
  assert.equal(
    schema.safeParse({ ...stopped, enabled: "false" }).success,
    false,
  );
});
