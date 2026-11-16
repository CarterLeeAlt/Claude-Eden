import assert from "node:assert/strict";
import { randomBytes } from "node:crypto";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";

const run = promisify(execFile);
const target = process.argv[2];
assert.ok(path.isAbsolute(target || "") && target.toLowerCase().endsWith(".exe"), "supply an absolute Claude Eden Windows executable path");
const eden = target;
const evidence = process.env.EDEN_REMOTE_QA_DIR;
if (evidence) await fs.mkdir(evidence, { recursive: true });
const profile = await fs.mkdtemp(path.join(os.tmpdir(), "eden-packaged-"));
const storageDirectory = profile + "-storage";
const env = { ...process.env, EDEN_CONFIG: path.join(profile, "config.json") };
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.EDEN_PLAYWRIGHT_MODULE || "playwright");
let browser, page;
const errors = [];
const responses = [];
const responseBodies = [];
try {
  const { stdout } = await run(eden, ["desktop", "--json"], { env, windowsHide: true });
  const entry = JSON.parse(stdout);
  assert.equal(entry.status, "ready");
  assert.equal(entry.controller.version, (await fs.readFile(new URL("../../VERSION", import.meta.url), "utf8")).trim());
  browser = await chromium.launch({ headless: true, executablePath: process.env.EDEN_TEST_BROWSER_PATH });
  page = await browser.newPage();
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("response", response => {
    const pathname = new URL(response.url()).pathname;
    if (!["/api/status", "/api/environment/storage", "/api/environment/guest-profile/preview"].includes(pathname)) return;
    const item = { path: pathname, method: response.request().method(), status: response.status() };
    responses.push(item);
    responseBodies.push(response.text().then(body => { item.body = body; }, error => { item.error = error.message; }));
  });
  await page.goto(entry.url);
  await page.getByRole("heading", { name: "注册 Claude Eden 账户", exact: true }).waitFor();
  await page.getByLabel("用户名",{exact:true}).waitFor();
  const initial = await page.evaluate(async () => (await fetch("/api/client/local-status")).json());
  assert.ok(initial.known && initial.service_running && !initial.running, "opening the local home must not boot the VM");
  assert.equal(await page.getByRole("heading",{name:"选择工作环境"}).count(),0);
  if (evidence) await page.screenshot({ path:path.join(evidence,"desktop-first-open.png") });
  const password = randomBytes(24).toString("base64url");
  await page.getByLabel("用户名",{exact:true}).fill("packaged_smoke");
  await page.getByLabel("设置密码",{exact:true}).fill(password);
  await page.getByLabel("确认密码",{exact:true}).fill(password);
  const statusResponse = page.waitForResponse((response) => response.url().endsWith("/api/status") && response.ok());
  await page.getByRole("button",{name:"创建本地账户",exact:true}).click();
  await page.getByRole("heading", { name: "保存你的恢复密钥", exact: true }).waitFor();
  assert.ok((await page.getByLabel("恢复密钥", { exact: true }).inputValue()).length > 20);
  await page.getByRole("checkbox", { name: "我已安全保存完整恢复密钥" }).check();
  await page.getByRole("button", { name: "继续使用 Claude Eden", exact: true }).click();
  const status = await (await statusResponse).json();
  assert.equal(status.vm_running, false);
  await page.getByRole("heading", { name: "为 Claude Eden 准备独立环境", exact: true }).waitFor();
  const nav = page.getByRole("navigation", {name:"主导航"});
  assert.equal(await nav.count(), 0, "fresh profile must not mount workbench navigation");
  assert.equal(new URL(page.url()).hash, "", "launch ticket was not removed");
  const directoryInput = page.getByLabel("存储目录", { exact: true });
  const next = page.getByRole("button", { name: "下一步：配置代理", exact: true });
  const readStorage = () => page.evaluate(async () => {
    const context = await (await fetch("/api/client/context")).json();
    const response = await fetch("/api/environment/storage", { headers: { "X-Eden-Connection": context.connection_id } });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  });
  const rejectedDirectory = path.join(profile, "environment-storage");
  await directoryInput.fill(rejectedDirectory);
  await next.click();
  await page.getByRole("alert").filter({ hasText: "请选择独立空目录，不能与程序、配置、工作区、现有存储或备份重叠" }).waitFor();
  assert.equal(await directoryInput.inputValue(), rejectedDirectory, "rejection must preserve the input for correction");
  const unchanged = await readStorage();
  assert.equal(unchanged.directory, path.join(profile, "state"));
  await assert.rejects(fs.stat(rejectedDirectory), { code: "ENOENT" });
  await directoryInput.fill(storageDirectory);
  await next.click();
  await page.getByRole("heading", { name: "连接你的网络代理", exact: true }).waitFor();
  const saved = await readStorage();
  assert.equal(saved.directory, storageDirectory);
  assert.equal(saved.custom, true);
  assert.equal(saved.has_environment, false);
  assert.equal(JSON.parse(await fs.readFile(path.join(profile, "environment-storage.json"), "utf8")).directory, storageDirectory);
  await page.getByRole("button", { name: "上一步", exact: true }).click();
  assert.equal(await directoryInput.inputValue(), storageDirectory);
  await page.reload();
  await page.getByRole("heading", { name: "为 Claude Eden 准备独立环境", exact: true }).waitFor();
  await page.waitForFunction(directory => document.querySelector("#initialization-directory")?.value === directory, storageDirectory);
  await page.getByRole("button", { name: "下一步：配置代理", exact: true }).click();
  await page.getByRole("heading",{name:"连接你的网络代理",exact:true}).waitFor();
  assert.ok(await page.getByRole("button",{name:"开始安装",exact:true}).isDisabled(), "proxy must be applied before installation");
  await page.getByText("添加节点", { exact: true }).first().waitFor();
  const final = await page.evaluate(async () => (await fetch("/api/client/local-status")).json());
  assert.ok(final.known && final.service_running && !final.running, "initialization must leave the VM stopped");
  await Promise.all(responseBodies);
  assert.equal(responses.filter(item => item.path === "/api/environment/storage" && item.method === "POST").length, 2, "back navigation and reload must not repeat a completed storage move");
  for (const response of responses.filter(item => item.path === "/api/status" && item.status === 200)) {
    assert.equal(JSON.parse(response.body).vm_running, false, "initialization started the VM");
  }
  await page.getByRole("heading", { name: "连接你的网络代理", exact: true }).scrollIntoViewIfNeeded();
  if(evidence) await page.screenshot({ path:path.join(evidence,"initialization-proxy.png") });
  assert.deepEqual(errors, []);
  console.log(JSON.stringify({ checks: ["standalone initialization after account and recovery setup", "VM stopped throughout", "no workbench navigation before initialization", "overlapping storage rejected without changing files", "workspace-scoped storage retry and persistence", "back navigation and reload without repeating the move", "proxy required before installation", "Windows executable bootstrap", "real session and APIs", "embedded production UI loaded", "private profile onboarding", "launch ticket removed"], version: entry.controller.version }));
} catch (error) {
  if (evidence && page && !page.isClosed()) {
    await page.screenshot({ path: path.join(evidence, "initialization-failure.png"), fullPage: true });
    await Promise.all(responseBodies);
    await fs.writeFile(path.join(evidence, "initialization-failure.json"), JSON.stringify({
      error: error.message, errors, responses,
      headings: await page.getByRole("heading").allTextContents(),
      alerts: await page.getByRole("alert").allTextContents(),
    }, null, 2));
  }
  throw error;
} finally {
  await browser?.close();
  await run(eden, ["prepare-update", "--json"], { env, windowsHide: true });
  await fs.rm(profile, { recursive: true, force: true });
  await fs.rm(profile + "-workspace", { recursive: true, force: true });
  await fs.rm(storageDirectory, { recursive: true, force: true });
}
