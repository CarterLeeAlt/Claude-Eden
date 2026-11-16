import test from "node:test";
import assert from "node:assert/strict";
import {
  defaultAppearance,
  normalizeAppearance,
  themeVariables,
  themeDocument,
  parseThemeDocument,
  sameTheme,
  colorGroups,
} from "./appearance-state.mjs";

test("element overrides stay independent and reset returns to derived colors", () => {
  const appearance = normalizeAppearance(structuredClone(defaultAppearance));
  const theme = appearance.lightTheme;
  const before = themeVariables(theme);
  theme.colors = {
    chrome: "#123456",
    button: "#654321",
    buttonInk: "#abcdef",
    selection: "#aabbcc",
  };
  theme.surface = "#fafafa";
  const vars = themeVariables(theme);
  assert.equal(vars["--chrome"], "#123456");
  assert.equal(vars["--button-ink"], "#abcdef");
  assert.equal(vars["--surface"], "#fafafa");
  assert.equal(vars["--border"], vars["--line"]);
  assert.notEqual(vars["--line"], before["--line"]);
  delete theme.colors.chrome;
  assert.notEqual(themeVariables(theme)["--chrome"], "#123456");
});

test("theme document round trips all editable colors without device preferences or library", () => {
  const appearance = normalizeAppearance(structuredClone(defaultAppearance));
  appearance.lightTheme.colors = Object.fromEntries(
    colorGroups.flatMap((group) =>
      group.entries.map(([key]) => [key, "#123456"]),
    ),
  );
  const document = themeDocument("  自定义主题  ", appearance);
  const imported = parseThemeDocument(JSON.stringify(document));
  assert.equal(imported.name, "自定义主题");
  assert.ok(sameTheme(appearance, imported));
  assert.deepEqual(Object.keys(imported).sort(), [
    "format",
    "lightTheme",
    "name",
    "version",
  ]);
  const copy = structuredClone(imported);
  copy.lightTheme.colors = Object.fromEntries(
    Object.entries(copy.lightTheme.colors).reverse(),
  );
  assert.ok(sameTheme(copy, imported));
  copy.lightTheme.colors.selection = "#000000";
  assert.equal(sameTheme(copy, imported), false);
});

test("invalid theme files are rejected before any mutation", () => {
  const valid = themeDocument("主题", defaultAppearance);
  const reject = (mutate) => {
    const data = structuredClone(valid);
    mutate(data);
    assert.throws(() => parseThemeDocument(JSON.stringify(data)));
  };
  for (const input of [
    "not json",
    "null",
    "[]",
    "{}",
    " ".repeat(256 * 1024 + 1),
  ])
    assert.throws(() => parseThemeDocument(input));
  reject((data) => {
    data.version = 1;
  });
  reject((data) => {
    data.name = "  ";
  });
  reject((data) => {
    delete data.lightTheme;
  });
  reject((data) => {
    data.lightTheme.accent = "#abc";
  });
  reject((data) => {
    data.lightTheme.colors = { chrome: "red" };
  });
  reject((data) => {
    data.lightTheme.colors = { unknown: "#abcdef" };
  });
  reject((data) => {
    data.lightTheme.terminal = { foreground: "#ffffff" };
  });
  reject((data) => {
    data.lightTheme.fonts = { code: "Menlo" };
  });
  reject((data) => {
    data.lightTheme.contrast = 10.5;
  });
  reject((data) => {
    data.mode = "dark";
  });
  reject((data) => {
    data.darkTheme = data.lightTheme;
  });
  assert.throws(() => normalizeAppearance({ mode: "dark" }));
  assert.throws(() => normalizeAppearance({ lightTheme: { terminal: {} } }));
  assert.throws(() => normalizeAppearance({ customThemes: [{ id: "test", name: "主题", lightTheme: defaultAppearance.lightTheme, darkTheme: {} }] }));
  assert.equal(defaultAppearance.lightTheme.accent, "#339cff");
});

test("all built-in palettes produce valid, independently editable theme documents", async () => {
  const { themeCatalog, presets, presetNames, presetGroups, applyThemePreset } =
    await import("./appearance-presets.mjs");
  assert.equal(
    new Set(themeCatalog.map((item) => item.id)).size,
    themeCatalog.length,
  );
  assert.equal(Object.keys(presets).length, 10);
  for (const item of themeCatalog) {
    assert.ok(presetGroups.includes(item.group));
    assert.equal(presetNames[item.id], item.name);
    const current = structuredClone(defaultAppearance.lightTheme);
    current.colors = { danger: "#123456", toolbar: "#123456" };
    const applied = applyThemePreset(current, item.id);
    const appearance = normalizeAppearance({ lightTheme: applied });
    const imported = parseThemeDocument(
      JSON.stringify(themeDocument(item.name, appearance)),
    );
    assert.ok(sameTheme(appearance, imported), item.id);
    assert.notEqual(applied.colors.danger, "#123456");
    assert.ok(
      Object.values(themeVariables(applied)).every(
        (value) => !String(value).includes("NaN"),
      ),
    );
    applied.colors.line = "#123456";
    assert.notEqual(presets[item.id].colors.line, "#123456");
  }
});

function contrastRatio(left, right) {
  const luminance = (color) => {
    const [r, g, b] = [1, 3, 5]
      .map((index) => parseInt(color.slice(index, index + 2), 16) / 255)
      .map((value) =>
        value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4,
      );
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  const a = luminance(left),
    b = luminance(right);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

test("accessible presets keep text, state colors, focus and borders legible", async () => {
  const { applyThemePreset } = await import("./appearance-presets.mjs");
  for (const id of ["color-vision", "high-contrast"]) {
    const theme = applyThemePreset(defaultAppearance.lightTheme, id);
    const vars = themeVariables(theme);
    const threshold = id === "high-contrast" ? 7 : 4.5;
    const check = (foreground, background, minimum, name) =>
      assert.ok(
        contrastRatio(foreground, background) >= minimum,
        `${id}/${name}: ${contrastRatio(foreground, background).toFixed(3)} < ${minimum}`,
      );
    for (const key of [
      "--ink",
      "--strong",
      "--muted",
      "--link",
      "--success",
      "--danger",
      "--warning",
      "--inactive",
    ]) {
      for (const background of [
        "--paper",
        "--chrome",
        "--chrome-dark",
        "--hover",
        "--selected",
        "--input",
      ]) {
        // State messages and links are shown on paper/panel surfaces, not selection rows.
        if (
          ["--hover", "--selected"].includes(background) &&
          !["--ink", "--strong"].includes(key)
        )
          continue;
        check(
          vars[key],
          vars[background],
          threshold,
          `${key} on ${background}`,
        );
      }
    }
    check(vars["--sidebar-ink"], vars["--sidebar"], threshold, "sidebar");
    check(vars["--toolbar-ink"], vars["--toolbar"], threshold, "toolbar");
    check(vars["--button-ink"], vars["--button"], threshold, "button");
    for (const background of ["--paper", "--chrome", "--input"]) {
      check(vars["--line"], vars[background], 3, "border");
      check(vars["--focus"], vars[background], 3, "focus");
    }
    assert.equal(theme.opaqueWindows, true);
    assert.notEqual(vars["--success"], vars["--danger"]);
  }
});
