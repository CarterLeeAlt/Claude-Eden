import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { chooseInline, createWorkbench } from "./workbench-fixture.mjs";

const output = path.resolve(import.meta.dirname, "../../.tmp/Language_Simplification/ui");
await fs.mkdir(output, { recursive: true });
const { browser, page, state, errors } = await createWorkbench();
const languages = [
  ["zh_CN.UTF-8", "zh-CN"], ["zh_TW.UTF-8", "zh-TW"], ["en_US.UTF-8", "en-US"],
  ["ja_JP.UTF-8", "ja-JP"], ["ko_KR.UTF-8", "ko-KR"], ["de_DE.UTF-8", "de-DE"], ["fr_FR.UTF-8", "fr-FR"],
];
const system = () => page.getByRole("combobox", { name: "系统语言", exact: true });
const web = () => page.getByRole("combobox", { name: "网页首选语言", exact: true });
const open = async () => {
  await page.getByRole("button", { name: /^设置/ }).click();
  await page.getByRole("button", { name: "时区与语言", exact: true }).click();
};
const save = async () => {
  await page.getByRole("button", { name: "保存设置", exact: true }).click();
  await page.getByRole("region", { name: "设置", exact: true }).waitFor({ state: "hidden" });
};
try {
  state.settings.auto_language = true;
  await page.reload();
  await page.route("**/api/settings/probe", (route) => route.fulfill({
    status: 400,
    contentType: "text/plain; charset=utf-8",
    body: "已测得出口 IP，但位置查询均失败；未使用直连回退",
  }));
  const policy = structuredClone(state.settings.exit_policy);
  await page.getByRole("button", { name: /^设置/ }).click();
  await page.getByRole("button", { name: "时区与语言", exact: true }).click();
  await page.getByRole("button", { name: "现在检测", exact: true }).first().click();
  await page.getByRole("alert").filter({ hasText: "位置查询均失败" }).waitFor();
  await page.getByRole("button", { name: "手动设置时区和语言", exact: true }).click();
  assert.equal(await page.getByRole("switch").nth(0).isChecked(), false);
  assert.equal(await page.getByRole("switch").nth(1).isChecked(), false);
  await chooseInline(page.getByRole("combobox", { name: "系统时区", exact: true }), "Asia/Tokyo");
  await chooseInline(page.getByRole("combobox", { name: "系统语言", exact: true }), "ja_JP.UTF-8");
  await chooseInline(page.getByRole("combobox", { name: "网页首选语言", exact: true }), "ja-JP");
  await page.getByText("所有出口仍须查询国家或地区", { exact: false }).waitFor();
  await page.screenshot({ path: path.join(output, "manual-settings.png") });
  await page.getByRole("button", { name: "保存设置", exact: true }).click();
  await page.getByRole("heading", { name: "时区与语言", exact: true }).waitFor({ state: "hidden" });
  assert.equal(state.settings.auto_timezone, false);
  assert.equal(state.settings.auto_language, false);
  assert.equal(state.settings.timezone, "Asia/Tokyo");
  assert.equal(state.settings.locale, "ja_JP.UTF-8");
  assert.equal(state.settings.browser_language, "ja-JP");
  assert.deepEqual(state.settings.exit_policy, policy);
  await page.reload();
  await page.getByRole("button", { name: /^设置/ }).click();
  await page.getByRole("button", { name: "时区与语言", exact: true }).click();
  assert.equal(await page.getByLabel("系统时区", { exact: true }).inputValue(), "Asia/Tokyo");
  assert.equal(await page.getByRole("combobox", { name: "系统语言", exact: true }).getAttribute("value"), "ja_JP.UTF-8");
  assert.equal(await page.getByRole("combobox", { name: "网页首选语言", exact: true }).getAttribute("value"), "ja-JP");
  assert.equal(await page.getByRole("switch").nth(0).isChecked(), false);
  assert.equal(await page.getByRole("switch").nth(1).isChecked(), false);
  await system().click();
  assert.deepEqual(await page.getByRole("option").evaluateAll(options => options.map(option => option.dataset.value)), languages.map(pair => pair[0]));
  await system().press("Escape");
  await web().click();
  assert.deepEqual(await page.getByRole("option").evaluateAll(options => options.map(option => option.dataset.value)), languages.map(pair => pair[1]));
  await web().press("Escape");
  for (const [locale, browserLanguage] of languages) {
    await chooseInline(system(), locale);
    await chooseInline(web(), browserLanguage);
    await save();
    assert.equal(state.settings.locale, locale);
    assert.equal(state.settings.browser_language, browserLanguage);
    assert.deepEqual(state.settings.exit_policy, policy);
    await page.reload(); await open();
    assert.equal(await system().getAttribute("value"), locale);
    assert.equal(await web().getAttribute("value"), browserLanguage);
  }
  await chooseInline(system(), "ko_KR.UTF-8");
  await chooseInline(web(), "ko-KR");
  state.failSave = true;
  await page.getByRole("button", { name: "保存设置", exact: true }).click();
  await page.getByRole("alert").filter({ hasText: "保存失败" }).waitFor();
  assert.equal(state.settings.locale, "fr_FR.UTF-8");
  assert.equal(await system().getAttribute("value"), "ko_KR.UTF-8");
  state.failSave = false;
  await save(); await open();
  let suggestion = { ip: "8.8.8.8", country: "Spain", country_code: "ES", region: "", city: "", timezone: "Europe/Madrid", checked_at: new Date().toISOString() };
  await page.route("**/api/settings/probe", route => route.fulfill({ json: suggestion }));
  await page.getByRole("switch").nth(0).check();
  await page.getByRole("switch").nth(1).check();
  await page.getByRole("button", { name: "现在检测", exact: true }).first().click();
  await page.getByText("出口建议时区：Europe/Madrid，保存时应用。", { exact: true }).waitFor();
  assert.equal(await system().getAttribute("value"), "ko_KR.UTF-8", "missing suggestion must preserve the selected language");
  await save(); await open();
  assert.equal(state.settings.timezone, "Europe/Madrid");
  suggestion = { ip: "8.8.8.8", country: "Canada", country_code: "CA", region: "", city: "", timezone: "America/Toronto", checked_at: new Date().toISOString(), suggested_locale: "en_CA.UTF-8", suggested_browser_language: "en-CA" };
  await page.getByRole("button", { name: "现在检测", exact: true }).first().click();
  await page.getByText("地区建议：en_CA.UTF-8 / en-CA", { exact: true }).waitFor();
  assert.equal(await system().getAttribute("value"), "en_CA.UTF-8");
  assert.equal(await web().getAttribute("value"), "en-CA");
  assert.equal(await system().isDisabled(), true);
  assert.equal(await web().isDisabled(), true);
  await save(); await page.reload(); await open();
  assert.equal(await system().getAttribute("value"), "en_CA.UTF-8");
  assert.equal(await web().getAttribute("value"), "en-CA");
  await page.getByRole("switch").nth(1).uncheck();
  await chooseInline(system(), "de_DE.UTF-8");
  await page.getByRole("button", { name: "返回工作台", exact: true }).first().click();
  const confirm = page.getByRole("dialog", { name: "放弃尚未保存的修改？", exact: true });
  await confirm.getByRole("button", { name: "取消", exact: true }).click();
  assert.equal(await system().getAttribute("value"), "de_DE.UTF-8");
  await page.getByRole("button", { name: "返回工作台", exact: true }).first().click();
  await confirm.getByRole("button", { name: "放弃修改", exact: true }).click();
  await open();
  assert.equal(await system().getAttribute("value"), "en_CA.UTF-8");
  await page.setViewportSize({ width: 560, height: 900 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  await page.screenshot({ path: path.join(output, "region-560.png") });
  await page.getByRole("button", { name: "返回工作台", exact: true }).first().click();
  assert.deepEqual(errors, []);
  console.log(JSON.stringify({ passed: true, checks: ["seven manual languages saved/reloaded", "failed lookup/manual recovery", "save failure/retry", "unsupported region preserves language", "supported regional variants", "cancel/discard", "compact layout", "exit policy preserved"] }));
} catch (error) {
  await page.screenshot({ path: path.join(output, "failure.png") });
  throw error;
} finally {
  await browser.close();
}
