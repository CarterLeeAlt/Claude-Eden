// Uses the built app, real workbench APIs, real Mihomo, and local CONNECT
// upstreams. No request reaches the user's running Eden or Clash instances.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import http from "node:http";
import net from "node:net";
import path from "node:path";
import { spawn, execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { chooseInline } from "./workbench-fixture.mjs";
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.EDEN_PLAYWRIGHT_MODULE || "playwright");
const run = promisify(execFile);
const root = path.resolve(import.meta.dirname, "../..");
const executable = process.env.EDEN_QA_EXECUTABLE;
assert.ok(executable && path.isAbsolute(executable) && executable.toLowerCase().endsWith(".exe"), "Set EDEN_QA_EXECUTABLE to the Eden Windows executable beside mihomo");
const output = path.join(root, ".tmp/Proxy_Simplification/ui/managed-proxy");
await fs.mkdir(output, { recursive: true });
await fs.writeFile(path.join(output, "result.json"), JSON.stringify({ passed: false, state: "running" }));
const profile = await fs.mkdtemp(path.join(output, "profile-"));
const config = path.join(profile, "config.json");
const env = { ...process.env, EDEN_CONFIG: config };
await run(executable, ["init"], { env });
// Exercise settings without provisioning a VM; registration and all proxy APIs remain real.
await fs.writeFile(path.join(profile, "initialization.json"), JSON.stringify({ version: 1, completed: true }), { mode: 0o600 });
const sockets = new Set();
async function upstream(label) {
  const target = http.createServer((_req, res) => res.end(`via-${label}`));
  await new Promise((resolve) => target.listen(0, "127.0.0.1", resolve));
  let requests = 0;
  const proxy = http.createServer();
  proxy.on("connect", (_req, client, head) => {
    requests++;
    const remote = net.connect(target.address().port, "127.0.0.1", () => {
      client.write("HTTP/1.1 200 Connection Established\r\n\r\n");
      if (head.length) remote.write(head);
      remote.pipe(client); client.pipe(remote);
    });
    for (const socket of [remote, client]) { sockets.add(socket); socket.on("close", () => sockets.delete(socket)); socket.on("error", () => { client.destroy(); remote.destroy(); }); }
  });
  await new Promise((resolve) => proxy.listen(0, "127.0.0.1", resolve));
  return { proxy, target, port: proxy.address().port, requests: () => requests };
}
const a = await upstream("A"), b = await upstream("B");
const reserved = net.createServer(); await new Promise((resolve) => reserved.listen(0, "127.0.0.1", resolve));
const port = reserved.address().port; await new Promise((resolve) => reserved.close(resolve));
const log = await fs.open(path.join(profile, "server.log"), "w", 0o600);
const server = spawn(executable, ["serve"], { env, stdio: ["ignore", log.fd, log.fd], windowsHide: true });
let browser, page;
const errors = [];
try {
  let control;
  for (let attempt = 0; attempt < 100; attempt++) {
    try { control = JSON.parse(await fs.readFile(path.join(profile, "control.json"))); break; } catch { await new Promise((r) => setTimeout(r, 100)); }
  }
  assert.ok(control, "control service did not start");
  const origin = `http://localhost:${control.port}`;
  const unauth = await fetch(`${origin}/api/proxy`); assert.equal(unauth.status, 401);
  const launchResponse = await fetch(`${origin}/api/launch`, { method: "POST", headers: { Origin: origin, "Content-Type": "application/json" }, body: JSON.stringify({ token: control.token }) });
  assert.equal(launchResponse.status, 200);
  const entry = await launchResponse.json();
  browser = await chromium.launch({ headless: true, executablePath: process.env.EDEN_TEST_BROWSER_PATH });
  page = await browser.newPage({ viewport: { width: 1280, height: 850 } });
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto(entry.url);
  await page.getByLabel("用户名", { exact: true }).fill("proxy_ui_qa");
  await page.getByLabel("设置密码", { exact: true }).fill("isolated-proxy-qa-password");
  await page.getByLabel("确认密码", { exact: true }).fill("isolated-proxy-qa-password");
  await page.getByRole("button", { name: "创建本地账户", exact: true }).click();
  const recovery = page.getByRole("region", { name: "保存恢复密钥", exact: true });
  const downloadEvent = page.waitForEvent("download");
  await recovery.getByRole("button", { name: "下载密钥文件", exact: true }).click();
  await (await downloadEvent).saveAs(path.join(profile, "recovery.txt"));
  await recovery.getByRole("checkbox", { name: "我已安全保存完整恢复密钥" }).check();
  await recovery.getByRole("button", { name: "继续使用 Claude Eden", exact: true }).click();
  console.log("QA: isolated account and workbench ready");
  await page.getByRole("button", { name: /^设置/ }).click();
  await page.getByRole("button", { name: "网络与代理", exact: true }).click();
  await page.getByRole("heading", { name: "内置代理", exact: true }).waitFor();
  await page.screenshot({ path: path.join(output, "empty.png") });
  const idle = async () => {
    await page.waitForFunction(() => !document.querySelector(".proxy-progress"));
    const job = await page.evaluate(() => fetch("/api/status").then(r => r.json()).then(s => s.job));
    assert.equal(job.error || "", "");
  };
  const add = async (name, upstreamPort) => {
    await page.getByRole("button", { name: "添加节点", exact: true }).click();
    await page.getByLabel("节点名称", { exact: true }).fill(name);
    await chooseInline(page.getByRole("combobox", { name: "协议", exact: true }), "http");
    await page.getByLabel("服务器", { exact: true }).fill("127.0.0.1");
    await page.getByLabel("端口", { exact: true }).fill(String(upstreamPort));
    await page.getByRole("button", { name: "保存并验证", exact: true }).click();
    await page.locator(".proxy-node-editor").waitFor({ state: "hidden" });
    await idle();
  };
  await add("QA A", a.port);
  await add("QA B", b.port);
  await page.getByRole("button", { name: "选择节点 QA A", exact: true }).waitFor();
  await page.getByRole("tab", { name: "诊断", exact: true }).click();
  await page.getByLabel("独立代理端口", { exact: true }).fill(String(port));
  await page.getByRole("tab", { name: "节点", exact: true }).click();
  await page.getByRole("button", { name: "选择节点 QA A", exact: true }).click();
  await page.getByRole("button", { name: "应用代理", exact: true }).click();
  await page.getByText("核心运行中", { exact: false }).waitFor();
  await page.waitForFunction(() => !document.querySelector(".proxy-progress"));
  await page.getByText("没有未保存的修改", { exact: true }).waitFor();
  async function state() { return page.evaluate(() => fetch("/api/proxy").then((r) => r.json())); }
  let p = await state(); assert.equal(p.running, true); assert.equal(p.active, true); assert.equal(p.port, port);
  const firstRevision = p.revision;
  async function through() {
    const stored = JSON.parse(await fs.readFile(path.join(profile, "proxy/nodes.json")));
    const file=path.join(profile,"curl.conf");await fs.writeFile(file,`proxy = "socks5h://eden:${stored.auth}@127.0.0.1:${stored.port}"\n`,{mode:0o600});
    return (await run("curl", ["--config",file,"--noproxy", "", "--fail", "--silent", "--show-error", "--max-time", "10", "http://198.18.0.5/"])).stdout;
  }
  console.log("QA: core selected A"); assert.equal(await through(), "via-A"); assert.ok(a.requests() > 0);
  await page.getByLabel("搜索节点", { exact: true }).fill("QA B");
  assert.equal(await page.getByRole("button", { name: "选择节点 QA A", exact: true }).count(), 0);
  await page.getByLabel("搜索节点", { exact: true }).fill("");
  await page.screenshot({ path: path.join(output, "nodes.png") });
  await page.getByRole("button", { name: "选择节点 QA B", exact: true }).click();
  await page.getByRole("button", { name: "应用代理", exact: true }).click();
  await page.waitForFunction(() => !document.querySelector(".proxy-progress"));
  p = await state(); assert.notEqual(p.revision, firstRevision);
  assert.equal(await through(), "via-B"); assert.ok(b.requests() > 0);
  await assert.rejects(run("curl", ["--noproxy", "", "--fail", "--silent", "--max-time", "5", "--proxy", `socks5h://127.0.0.1:${port}`, "http://198.18.0.5/"]));
  await page.getByRole("button", { name: "编辑 QA B", exact: true }).click();
  await page.getByLabel("节点名称", { exact: true }).fill("QA B renamed");
  await page.getByRole("button", { name: "保存并验证", exact: true }).click();
  await page.locator(".proxy-node-editor").waitFor({ state: "hidden" });
  await idle();
  assert.equal(await through(), "via-B");
  assert.notEqual((await state()).revision, p.revision, "editing the active node must rebuild its applied revision");
  await page.reload();
  await page.getByRole("button", { name: /^设置/ }).click();
  await page.getByRole("button", { name: "网络与代理", exact: true }).click();
  await page.getByRole("tab", { name: "节点", exact: true }).click();
  assert.equal((await state()).selected, p.selected);
  await page.getByRole("tab", { name: "诊断", exact: true }).click();
  await page.getByLabel("独立代理端口", { exact: true }).fill(String(a.port));
  await page.getByRole("button", { name: "应用端口并重启核心", exact: true }).click();
  await page.getByRole("region", { name: "设置", exact: true }).getByRole("alert").filter({ hasText: "Claude Eden 代理端口已被占用，请更换独立端口" }).waitFor();
  assert.equal(a.proxy.listening, true);
  assert.equal((await state()).port, port, "port conflict replaced the saved profile");
  await page.getByRole("button", { name: "应用端口并重启核心", exact: true }).click();
  await page.waitForFunction(() => !document.querySelector(".proxy-progress"));
  assert.equal(await through(), "via-B");
  await page.getByRole("button", { name: "停止核心", exact: true }).click();
  await page.waitForFunction(() => !document.querySelector(".proxy-progress"));
  await assert.rejects(through());
  await page.screenshot({ path: path.join(output, "stopped.png") });
  await page.getByRole("button", { name: "应用端口并重启核心", exact: true }).click();
  await idle();
  assert.equal(await through(), "via-B");
  await page.getByRole("tab", { name: "节点", exact: true }).click();
  await page.getByRole("button", { name: "删除 QA A", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "取消", exact: true }).click();
  assert.equal((await state()).nodes.length, 2);
  await page.getByRole("button", { name: "删除 QA A", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "删除节点", exact: true }).click();
  await idle();
  assert.equal((await state()).nodes.length, 1);
  assert.equal(await through(), "via-B");
  await add("QA replacement", a.port);
  await page.getByRole("button", { name: "删除 QA B renamed", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "删除节点", exact: true }).click();
  await idle();
  p = await state();
  assert.equal(p.selected, ""); assert.equal(p.running, false); assert.equal(p.active, false);
  assert.equal(p.nodes.length, 1, "deleting the active node must preserve the other saved node without applying it");
  await assert.rejects(through());
  await page.getByRole("button", { name: "返回工作台", exact: true }).first().click();
  await page.getByRole("button", { name: "配置网络与代理", exact: true }).waitFor();
  assert.equal(await page.getByRole("button", { name: "启动 Claude Eden 环境", exact: true }).count(), 0);
  assert.deepEqual(errors, []);
  await fs.writeFile(path.join(output, "result.json"), JSON.stringify({ passed: true, realCore: true, realWorkbench: true, browserErrors: errors, upstreamARequests: a.requests(), upstreamBRequests: b.requests(), guestVMTested: false }, null, 2));
  console.log("Proxy UI and real bundled core: PASS; independent upstream switching, authentication, persistence and stop verified.");
} catch (error) {
  if (page && !page.isClosed()) {
    await page.screenshot({ path: path.join(output, "failure.png"), mask: [page.locator("input,textarea")] });
    console.error((await page.locator("body").innerText()).replace(/(?:[A-Fa-f0-9]{8}[-\s]){7}[A-Fa-f0-9]{8}/g, "[recovery secret]"));
  }
  throw error;
} finally {
  if (browser) await browser.close();
  await run(executable, ["prepare-update", "--json"], { env, timeout: 30000, windowsHide: true });
  if (server.exitCode === null) await new Promise(resolve => server.once("exit", resolve));
  await log.close();
  for (const socket of sockets) socket.destroy();
  for (const item of [a, b]) { item.proxy.closeAllConnections(); item.target.closeAllConnections(); item.proxy.close(); item.target.close(); }
  // Keep only this test's private logs on failure for diagnosis. No credentials
  // or control URLs are copied into the repository report.
  console.log(`Private QA profile retained at ${profile}`);
}
