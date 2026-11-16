import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { createWorkbench, defaultGuestProfile } from "./workbench-fixture.mjs";

const output = process.env.EDEN_TEST_CAPTURE;
if (output) await fs.mkdir(output, { recursive: true });
const randomProfile = { ...defaultGuestProfile, mode: "random", hostname: "Test-Random", username: "alex42", browser_username: "sam17", audio_name: "sample_speakers", audio_description: "Sample Speakers", display_width: 1920, display_height: 1080, browser_width: 1280, browser_height: 720, extension_name: "Tab Layout" };
let starts = 0, saved, failRead = true;
const fixture = await createWorkbench({ initialization: { required: true, tracked: true, started: false }, beforeNavigate: async (page, state) => {
  await page.addInitScript(() => localStorage.setItem("eden.audio", JSON.stringify({ muted: true })));
  await page.route("**/api/environment/storage", route => route.fulfill({ json: { directory: "C:/preview/EdenData", has_environment: false } }));
  await page.route("**/api/environment/guest-profile", route => failRead
    ? route.fulfill({ status: 503, contentType: "text/plain; charset=utf-8", body: "创建配置暂不可读" })
    : route.fulfill({ json: { profile: saved || null, defaults: defaultGuestProfile, editable: !saved, audio_models: [] } }));
  await page.route("**/api/environment/guest-profile/preview", route => {
    const body = route.request().postDataJSON();
    if (body.username === "root") return route.fulfill({ status: 400, contentType: "text/plain; charset=utf-8", body: "Linux 账户不能使用系统账户" });
    return route.fulfill({ json: body.mode === "random" && !body.username ? randomProfile : { ...body, hostname: body.hostname || "Test-PC" } });
  });
  await page.route("**/api/start", route => {
    const input = route.request().postDataJSON(); starts++;
    if (starts === 1) {
      assert.equal(input.guest_profile.mode, "manual");
      assert.equal(input.guest_profile.username, "alex");
      assert.equal(input.guest_profile.browser_username, "web");
      assert.equal(input.guest_profile.work_home, "/home/projects");
      assert.equal(input.guest_profile.graphics_backend, "auto");
      assert.equal(input.guest_profile.webgl, "disabled");
      saved = { version: 1, ...input.guest_profile, extension_suffix: "0123456789abcdef" };
    } else assert.deepEqual(input, { guest_system: "debian-13" });
    state.status.job = { id: "profile-install-" + starts, kind: "guest-install", running: true, setup_progress: { phase: "image", system: "debian-13", label: "正在准备镜像" } };
    return route.fulfill({ json: state.status.job });
  });
} });
try {
  const { page, state, errors } = fixture;
  const profile = page.getByRole("region", { name: "客机创建配置" });
  await profile.getByRole("button", { name: "重新读取配置" }).waitFor();
  assert.equal(await page.getByRole("button", { name: "下一步：配置代理" }).isDisabled(), true);
  failRead = false;
  await profile.getByRole("button", { name: "重新读取配置" }).click();
  await profile.getByRole("button", { name: "使用默认配置" }).waitFor();
  await profile.locator("summary").click();
  assert.equal(await profile.getByLabel("工作账户", { exact: true }).inputValue(), "developer");
  assert.equal(await profile.getByLabel("工作账户", { exact: true }).isDisabled(), true);
  await profile.getByRole("button", { name: "随机生成配置" }).click();
  await page.waitForFunction(() => document.querySelector('.guest-profile-options input[maxlength="24"]')?.value === "alex42");
  assert.equal(await profile.getByLabel("浏览器账户", { exact: true }).inputValue(), "sam17");
  await profile.getByRole("button", { name: "手动配置" }).click();
  await profile.getByLabel("工作账户", { exact: true }).fill("root");
  await page.getByRole("button", { name: "下一步：配置代理" }).click();
  await page.getByRole("alert").filter({ hasText: "不能使用系统账户" }).waitFor();
  assert.equal(starts, 0);
  await profile.getByLabel("工作账户", { exact: true }).fill("alex");
  await profile.getByLabel("浏览器账户", { exact: true }).fill("web");
  await profile.getByLabel("工作账户主目录", { exact: true }).fill("/home/projects");
  await profile.getByLabel("浏览器账户主目录", { exact: true }).fill("/home/browsing");
  await profile.getByRole("combobox", { name: "图形渲染后端", exact: true }).click();
  await page.getByRole("option", { name: "ANGLE / OpenGL", exact: true }).click();
  await profile.getByRole("combobox", { name: "WebGL", exact: true }).click();
  await page.getByRole("option", { name: "关闭网页 3D 图形能力", exact: true }).click();
  assert.equal(await profile.getByRole("combobox", { name: "图形渲染后端", exact: true }).isDisabled(), true);
  for (const width of [1280, 600, 390]) {
    await page.setViewportSize({ width, height: 900 });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    if (output) await page.screenshot({ path: path.join(output, `manual-${width}.png`), fullPage: true });
  }
  await page.setViewportSize({ width: 1280, height: 850 });
  await page.getByRole("button", { name: "下一步：配置代理" }).click();
  await page.getByRole("heading", { name: "连接你的网络代理" }).waitFor();
  await page.getByRole("button", { name: "上一步", exact: true }).click();
  await profile.getByLabel("工作账户", { exact: true }).waitFor();
  assert.equal(await profile.getByLabel("工作账户", { exact: true }).inputValue(), "alex");
  await page.getByRole("button", { name: "下一步：配置代理" }).click();
  await page.getByRole("button", { name: "开始安装", exact: true }).click();
  await page.getByText("正在准备镜像", { exact: true }).waitFor();
  assert.equal(starts, 1);
  state.status.job = { id: "stopped-profile", kind: "stop", running: false };
  await page.reload();
  await profile.getByText("创建配置已保存，继续安装将复用以下设置。").waitFor();
  assert.equal(await profile.getByLabel("工作账户", { exact: true }).inputValue(), "alex");
  assert.equal(await profile.getByLabel("工作账户", { exact: true }).isDisabled(), true);
  assert.equal(await profile.getByRole("button", { name: "随机生成配置" }).count(), 0);
  if (output) await page.screenshot({ path: path.join(output, "saved-resume.png"), fullPage: true });
  await page.getByRole("button", { name: "下一步：配置代理" }).click();
  await page.getByRole("button", { name: "开始安装", exact: true }).click();
  await page.getByText("正在准备镜像", { exact: true }).waitFor();
  assert.equal(starts, 2);
  assert.deepEqual(errors, []);
  console.log("PASS guest profile: loading failure/retry, fixed defaults, random preview, manual validation, render options, three widths, back navigation and locked saved resume");
} finally { await fixture.browser.close(); }

{
  const fixture = await createWorkbench({ initialization: { required: true, tracked: true, started: false }, beforeNavigate: async page => {
    await page.route("**/api/environment/storage", route => route.fulfill({ json: { directory: "C:/preview/EdenData", has_environment: true } }));
    await page.route("**/api/environment/guest-profile", route => route.fulfill({ json: { profile: null, defaults: defaultGuestProfile, editable: false, audio_models: [] } }));
  } });
  try {
    await fixture.page.getByText("已有环境继续使用原账户、主目录和设备配置。").waitFor();
    assert.equal(await fixture.page.getByRole("button", { name: "随机生成配置" }).count(), 0);
    assert.deepEqual(fixture.errors, []);
    console.log("PASS existing guest: current accounts and device settings are retained");
  } finally { await fixture.browser.close(); }
}
