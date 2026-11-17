import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { defaultAppearance, normalizeAppearance, themeVariables, uiFontFamily } from "../lib/appearance-state.ts";

test("controller JSON, frontend default and pre-connection CSS share the full brand palette", async () => {
  const palette = JSON.parse(await fs.readFile(new URL("../../../internal/uitheme/light.json", import.meta.url), "utf8"));
  assert.deepEqual(defaultAppearance.lightTheme, palette);
  assert.deepEqual(normalizeAppearance({}), defaultAppearance);
  const variables = themeVariables(defaultAppearance.lightTheme);
  const css = await fs.readFile(new URL("../styles/tokens.css", import.meta.url), "utf8");
  for (const [name, value] of Object.entries(variables)) {
    if (!/^#[\da-f]{6}$/i.test(value)) continue;
    const declaration = css.match(new RegExp(`${name}:\\s*(#[\\da-f]{6})\\s*;`, "i"));
    if (["--sidebar", "--sidebar-ink", "--toolbar", "--toolbar-ink", "--border", "--surface"].includes(name)) continue;
    assert.equal(declaration?.[1], value, `${name} differs before and after /appearance`);
  }
  assert.match(uiFontFamily, /"Inter", "Noto Sans SC"/);
  for (const token of ["--control-height: 36px", "--control-compact: 32px", "--text-page: 1.4286rem", "--radius-control: 6px", "--radius-panel: 8px"])
    assert.ok(css.includes(token), token);
});

test("normalizing another explicit palette does not silently apply brand element overrides", () => {
  const appearance = normalizeAppearance({ lightTheme: { preset: "custom", accent: "#005fb8", surface: "#ffffff", ink: "#141413" } });
  assert.deepEqual(appearance.lightTheme.colors, {});
  assert.equal(themeVariables(appearance.lightTheme)["--paper"], "#ffffff");
});

test("all UI styles use the shared type scale, never system UI families or page-specific sizes", async () => {
  const directory = new URL("../styles/", import.meta.url);
  for (const name of await fs.readdir(directory)) {
    if (!name.endsWith(".css") || name === "tokens.css") continue;
    const css = await fs.readFile(new URL(name, directory), "utf8");
    assert.doesNotMatch(css, /font-size:\s*\d+(?:\.\d+)?(?:px|rem|em)/, name);
    assert.doesNotMatch(css, /font-family:[^;]*(?:Segoe|Arial|Georgia|Poppins|Lora)/, name);
  }
});
