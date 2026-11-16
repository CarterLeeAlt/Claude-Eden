import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { chooseInline, createWorkbench } from "./workbench-fixture.mjs";
const output = fileURLToPath(
  new URL("../../.tmp/Font_Preview/verification/appearance/", import.meta.url),
);
await fs.mkdir(output, { recursive: true });
const { browser, page, state, errors } = await createWorkbench();
const checks = [];
const showAppearance = async () => {
  await page.getByRole("button", { name: /^设置/ }).click();
  await page.getByRole("button", { name: "外观", exact: true }).click();
  await page.getByRole("heading", { name: "外观", exact: true }).waitFor();
};
const saved = () =>
  page
    .getByRole("status")
    .filter({ hasText: /更改自动保存|已同步 Codex 外观/ })
    .waitFor();
const shot = async (name) => {
  await page.locator(".settings-content").evaluate((el) => el.scrollTo(0, 0));
  await page.screenshot({
    path: output + name + ".png",
    animations: "disabled",
  });
};
try {
  await page.emulateMedia({ colorScheme: "light" });
  await showAppearance();
  await page
    .getByRole("button", { name: "从本机 Codex 同步", exact: true })
    .click();
  await page
    .getByRole("status")
    .filter({ hasText: "已同步 Codex 外观" })
    .waitFor();
  assert.equal(state.appearance.lightTheme.preset, "solarized");
  assert.equal(Object.hasOwn(state.appearance, "mode"), false);
  assert.equal(Object.hasOwn(state.appearance, "darkTheme"), false);
  assert.equal(
    await page.getByLabel("界面字号", { exact: true }).inputValue(),
    "16",
  );
  assert.equal(
    await page.getByLabel("代码字号", { exact: true }).inputValue(),
    "13",
  );
  await shot("appearance-light");
  assert.equal(await page.getByRole("group", { name: "主题模式" }).count(), 0);
  await page.emulateMedia({ colorScheme: "dark" });
  assert.equal(
    await page.evaluate(
      () => getComputedStyle(document.documentElement).colorScheme,
    ),
    "light",
  );
  assert.equal(
    await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue("--paper"),
    ),
    "#fdf6e3",
  );
  await page.emulateMedia({ colorScheme: "light" });
  checks.push("导入亮色配色、系统明暗变化保持亮色、16/13 字号");
  await page
    .getByRole("region", { name: "界面配色", exact: true })
    .getByRole("button", { name: "自定义", exact: true })
    .click();
  assert.equal(
    await page.getByText("终端 ANSI 配色", { exact: true }).count(),
    0,
  );
  await page.getByLabel("界面配色半透明侧栏", { exact: true }).uncheck();
  await saved();
  assert.equal(
    await page.evaluate(() => document.documentElement.dataset.translucent),
    "false",
  );
  await page.getByRole("switch", { name: "使用指针光标", exact: true }).check();
  await saved();
  assert.equal(
    await page
      .getByRole("button", { name: "从本机 Codex 同步", exact: true })
      .evaluate((el) => getComputedStyle(el).cursor),
    "pointer",
  );
  await chooseInline(page.getByLabel("减少动态效果", { exact: true }), "on");
  await saved();
  assert.equal(
    await page.evaluate(() => document.documentElement.dataset.reducedMotion),
    "true",
  );
  state.failAppearance = true;
  await page.getByLabel("代码字号", { exact: true }).fill("15");
  await page
    .getByRole("alert")
    .filter({ hasText: "外观设置未能保存" })
    .waitFor();
  assert.equal(
    await page.getByLabel("代码字号", { exact: true }).inputValue(),
    "15",
  );
  assert.equal(state.appearance.codeFontSize, 13);
  state.failAppearance = false;
  await page.getByRole("button", { name: "重试保存", exact: true }).click();
  await saved();
  await page.getByLabel("界面字号", { exact: true }).fill("17");
  await page.getByLabel("界面字号", { exact: true }).fill("18");
  await page.getByLabel("界面字号", { exact: true }).fill("16");
  await saved();
  await page.reload();
  await showAppearance();
  assert.equal(
    await page.getByLabel("代码字号", { exact: true }).inputValue(),
    "15",
  );
  assert.equal(state.appearance.uiFontSize, 16);
  assert.match(
    await page.evaluate(
      () => getComputedStyle(document.documentElement).fontFamily,
    ),
    /Inter.*Noto Sans SC/,
  );
  checks.push(
    "内置字体/半透明/指针/动态效果、失败保留输入与重试、快速调整最后写入胜出、刷新读回",
  );
  await page.setViewportSize({ width: 820, height: 558 });
  await shot("appearance-minimum");
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    ),
    false,
  );
  await page.getByRole("button", { name: "返回工作台", exact: true }).click();
  await page
    .getByRole("button", { name: "启动 Claude Eden 环境", exact: true })
    .click();
  await page
    .getByRole("button", { name: "新建终端", exact: true })
    .first()
    .click();
  await page.getByRole("menuitem", { name: "普通终端", exact: true }).click();
  await page.getByRole("tab", { name: "终端 1", exact: true }).waitFor();
  const count = state.terminalConnections;
  await showAppearance();
  await chooseInline(page.getByLabel("界面配色预设", { exact: true }), "nord");
  await saved();
  await page.getByRole("button", { name: "返回工作台", exact: true }).click();
  assert.equal(state.terminalConnections, count);
  assert.equal(
    await page
      .locator(".xterm-char-measure-element")
      .first()
      .evaluate((el) => getComputedStyle(el).fontSize),
    "15px",
  );
  assert.ok(
    !state.requests.some(
      (request) => request.path === "/settings" && request.method === "POST",
    ),
  );
  await page.setViewportSize({ width: 1280, height: 850 });
  await page.screenshot({
    path: output + "terminal-independent.png",
    animations: "disabled",
  });
  checks.push(
    "820px 设置可滚动且无横向溢出、切换外观保留终端连接、未调用环境设置接口",
  );
  assert.deepEqual(errors, []);
  await fs.writeFile(
    output + "smoke-result.json",
    JSON.stringify(
      {
        passed: true,
        checks,
        scope: "隔离控制接口；源配置导入和写盘另用真实服务验证",
      },
      null,
      2,
    ),
  );
  console.log(JSON.stringify({ passed: true, checks }));
} finally {
  await browser.close();
}
