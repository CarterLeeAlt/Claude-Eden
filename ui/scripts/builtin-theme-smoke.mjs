import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { chooseInline, createWorkbench } from "./workbench-fixture.mjs";
import {
  presets,
  themeCatalog,
  parseThemeDocument,
  sameTheme,
} from "../src/lib/appearance-state.ts";
const output = fileURLToPath(
  new URL("../../.tmp/Font_Preview/verification/presets/", import.meta.url),
);
await fs.mkdir(output, { recursive: true });
let exported;
const { browser, page, state, errors } = await createWorkbench({
  nativeRequest: (body) => {
    if (body.action === "exportTheme") { exported = body; return true; }
    return undefined;
  },
});
const checks = [];
const open = async () => {
  await page.getByRole("button", { name: /^设置/ }).click();
  await page.getByRole("button", { name: "外观", exact: true }).click();
  await page.getByRole("heading", { name: "外观", exact: true }).waitFor();
};
const saved = () =>
  page
    .locator(".appearance-save-state")
    .filter({ hasText: "更改自动保存，即时生效" })
    .waitFor();
const select = async (id) => {
  await chooseInline(
    page.getByLabel("界面配色预设", {
      exact: true,
    }),
    id,
  );
  await saved();
};
const shot = async (name) => {
  await page.locator(".settings-content").evaluate((el) => el.scrollTo(0, 0));
  await page.screenshot({
    path: output + name + ".png",
    animations: "disabled",
  });
};
try {
  await page.emulateMedia({ colorScheme: "light" });
  await open();
  const picker = page.getByLabel("界面配色预设", { exact: true });
  await picker.click();
  const list = page.getByRole("listbox");
  assert.equal(await list.locator(".inline-select-group").count(), 4);
  assert.equal(
    await list.getByRole("option").count(),
    Object.keys(presets).length,
  );
  await picker.press("Escape");
  // Every palette goes through the real picker, not just the model helper.
  for (const id of Object.keys(presets)) {
    await select(id);
    const theme = state.appearance.lightTheme;
    assert.equal(theme.preset, id);
    assert.deepEqual(theme.colors, presets[id].colors);
  }
  await select("vscode-modern");
  assert.equal(
    await page.evaluate(
      () =>
        getComputedStyle(document.querySelector(".settings-sidebar"))
          .backgroundColor,
    ),
    "rgb(248, 248, 248)",
  );
  await shot("vscode-modern-light");
  await select("sepia");
  await shot("reading-sepia");
  checks.push(
    `真实下拉选取全部 ${Object.keys(presets).length} 套亮色配色，四组分类与界面配色完整应用`,
  );
  await select("color-vision");
  assert.equal(
    await page.evaluate(() => document.documentElement.dataset.translucent),
    "false",
  );
  const preview = page.getByLabel("界面配色状态配色预览", { exact: true });
  for (const text of ["成功", "警告", "错误"])
    await preview.getByText(text, { exact: true }).waitFor();
  assert.equal(await preview.locator("svg").count(), 3);
  assert.equal(
    await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue("--success"),
    ),
    "#005a9c",
  );
  assert.equal(
    await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue("--danger"),
    ),
    "#a63b00",
  );
  await shot("color-vision-light");
  await select("high-contrast");
  await shot("high-contrast-light");
  // Switching away must clear accessibility colors as well as earlier user overrides.
  await select("paper");
  assert.equal(state.appearance.lightTheme.colors.success, undefined);
  await select("color-vision");
  await page.getByLabel("主题名称", { exact: true }).fill("我的蓝橙搭配");
  await page.getByRole("button", { name: "另存为主题", exact: true }).click();
  await page.getByRole("status").filter({ hasText: "主题已保存" }).waitFor();
  const snapshot = structuredClone(state.appearance);
  await page.getByRole("button", { name: "导出主题", exact: true }).click();
  await page.getByRole("status").filter({ hasText: "主题已导出" }).waitFor();
  assert.ok(sameTheme(parseThemeDocument(exported.text), snapshot));
  await fs.writeFile(
    output + "accessible-export.eden-theme.json",
    exported.text,
  );
  await page.reload();
  await open();
  assert.equal(
    await page
      .getByLabel("界面配色预设", { exact: true })
      .getAttribute("value"),
    "color-vision",
  );
  assert.equal(
    await page
      .getByLabel("已保存的主题", { exact: true })
      .getAttribute("value"),
    state.appearance.customThemes[0].id,
  );
  assert.ok(sameTheme(state.appearance, snapshot));
  checks.push(
    "蓝橙状态图标与文字、高对比不透明窗口、跨预设清除旧覆盖、命名保存/导出/刷新读回",
  );
  await page.setViewportSize({ width: 820, height: 558 });
  await shot("builtin-minimum");
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
  await page.waitForFunction(
    () =>
      getComputedStyle(document.querySelector(".xterm-scrollable-element"))
        .backgroundColor === "rgb(0, 0, 0)",
  );
  const connections = state.terminalConnections;
  await open();
  await select("high-contrast");
  await page.getByRole("button", { name: "返回工作台", exact: true }).click();
  await page.waitForFunction(
    () =>
      getComputedStyle(document.querySelector(".xterm-scrollable-element"))
        .backgroundColor === "rgb(0, 0, 0)",
  );
  assert.equal(state.terminalConnections, connections);
  await page.setViewportSize({ width: 1280, height: 850 });
  await page.screenshot({
    path: output + "terminal-color-vision.png",
    animations: "disabled",
  });
  checks.push("820px 无横向溢出；界面配色变化保留终端默认色与连接");
  assert.deepEqual(errors, []);
  assert.ok(
    !state.requests.some(
      (request) => request.path === "/settings" && request.method === "POST",
    ),
  );
  const result = {
    passed: true,
    families: themeCatalog.length,
    palettes: Object.keys(presets).length,
    checks,
    scope: "真实前端、xterm 和隔离 API；不连接日常 VM",
  };
  await fs.writeFile(output + "result.json", JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result));
} finally {
  await browser.close();
}
