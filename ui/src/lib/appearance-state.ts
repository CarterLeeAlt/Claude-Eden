export const uiFontFamily =
  '"Inter", "Noto Sans SC", sans-serif';
export const codeFontFamily =
  '"JetBrains Mono", "Noto Sans Mono CJK SC", monospace';
import { presets } from "./appearance-presets.ts";
import {
  WorkbenchAppearanceThemeSchema,
  WorkbenchAppearanceSchema,
} from "./contracts.generated.ts";
import { isRecord } from "./errors.ts";
import type { Appearance, AppearanceTheme } from "./types.ts";
export {
  presets,
  presetNames,
  presetGroups,
  presetInfo,
  themeCatalog,
  applyThemePreset,
} from "./appearance-presets.ts";
export const defaultAppearance: Appearance = {
  customThemes: [],
  lightTheme: {
    ...presets.eden,
    preset: "eden",
    opaqueWindows: true,
  },
  uiFontSize: 14,
  codeFontSize: 13,
  pointerCursors: false,
  fontSmoothing: true,
  reducedMotion: "system",
};
export function normalizeAppearance(input: unknown = {}): Appearance {
  const value = isRecord(input) ? input : {};
  if (Object.keys(value).some((key) => !Object.hasOwn(defaultAppearance, key)))
    throw new Error("外观设置包含不支持的项目");
  const merged = {
    ...defaultAppearance,
    ...value,
    customThemes: Array.isArray(value.customThemes) ? value.customThemes : [],
    lightTheme: {
      ...defaultAppearance.lightTheme,
      ...(isRecord(value.lightTheme) ? value.lightTheme : {}),
      colors:
        isRecord(value.lightTheme) && isRecord(value.lightTheme.colors)
          ? value.lightTheme.colors
          : isRecord(value.lightTheme) && value.lightTheme.preset !== "eden"
            ? {}
            : defaultAppearance.lightTheme.colors,
    },
  };
  validateTheme(merged.lightTheme);
  for (const saved of merged.customThemes) {
    if (
      !object(saved) ||
      Object.keys(saved).some(
        (key) => !["id", "name", "lightTheme"].includes(key),
      )
    )
      throw new Error("已保存主题包含不支持的设置");
    validateThemeName(saved.name);
    validateTheme(saved.lightTheme);
  }
  const appearance = WorkbenchAppearanceSchema.parse(merged);
  return { ...appearance, customThemes: appearance.customThemes ?? [] };
}
export function mix(a: string, b: string, weight: number) {
  const rgb = (hex: string) =>
    [1, 3, 5].map((index) => parseInt(hex.slice(index, index + 2), 16));
  const left = rgb(a),
    right = rgb(b);
  return (
    "#" +
    left
      .map((value, index) =>
        Math.round(value + (right[index] - value) * weight)
          .toString(16)
          .padStart(2, "0"),
      )
      .join("")
  );
}
function luminance(hex: string) {
  const [r, g, b] = [1, 3, 5]
    .map((index) => parseInt(hex.slice(index, index + 2), 16) / 255)
    .map((v) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return r * 0.2126 + g * 0.7152 + b * 0.0722;
}
export function themeVariables(theme: AppearanceTheme): Record<string, string> {
  const { surface, ink, accent, contrast } = theme;
  const chrome = mix(surface, ink, 0.025 + contrast * 0.00055);
  const line = mix(surface, ink, 0.09 + contrast * 0.0013);
  const variables: Record<string, string> = {
    "--paper": surface,
    "--chrome": chrome,
    "--chrome-dark": mix(surface, ink, 0.1),
    "--ink": ink,
    "--strong": mix(ink, "#000000", 0.15),
    "--muted": mix(ink, surface, 0.15),
    "--accent": accent,
    "--focus": accent,
    "--input": surface,
    "--line": line,
    "--hover": mix(surface, ink, 0.09),
    "--selected": mix(surface, ink, 0.13),
    "--selection": mix(surface, accent, 0.32),
    "--button": ink,
    "--button-ink": surface,
    "--link": accent,
    "--success": "#357c3c",
    "--danger": "#bb3535",
    "--warning": "#af7a22",
    "--inactive": "#a3a3a3",
    "--ui-font-family": uiFontFamily,
    "--code-font-family": codeFontFamily,
    "--accent-ink": luminance(accent) > 0.4 ? "#111111" : "#ffffff",
  };
  for (const [key, value] of Object.entries(theme.colors || {})) {
    if (colorTokens[key]) variables[colorTokens[key]] = value;
  }
  variables["--sidebar"] = theme.colors?.sidebar || variables["--chrome"];
  variables["--sidebar-ink"] = theme.colors?.sidebarInk || ink;
  variables["--toolbar"] = theme.colors?.toolbar || surface;
  variables["--toolbar-ink"] = theme.colors?.toolbarInk || ink;
  // Shared components also use these aliases.
  variables["--border"] = variables["--line"];
  variables["--surface"] = variables["--paper"];
  return variables;
}
// Optional overrides keep unedited colors linked to the three base colors.
export const colorGroups = [
  {
    label: "界面与层级",
    entries: [
      ["chrome", "面板背景"],
      ["sidebar", "侧栏背景"],
      ["sidebarInk", "侧栏文字"],
      ["toolbar", "工具栏背景"],
      ["toolbarInk", "工具栏文字"],
      ["chromeDark", "次级背景"],
      ["strong", "标题文字"],
      ["muted", "辅助文字"],
      ["line", "边框与分隔线"],
      ["input", "输入框背景"],
    ],
  },
  {
    label: "按钮与交互",
    entries: [
      ["button", "主按钮背景"],
      ["buttonInk", "主按钮文字"],
      ["accentInk", "强调色上的文字"],
      ["link", "链接"],
      ["focus", "焦点边框"],
      ["hover", "悬停背景"],
      ["selected", "选中背景"],
      ["selection", "文本选区"],
    ],
  },
  {
    label: "状态提示",
    entries: [
      ["success", "成功状态"],
      ["warning", "警告状态"],
      ["danger", "错误状态"],
      ["inactive", "未启动状态"],
    ],
  },
];
export const colorTokens = Object.fromEntries(
  colorGroups.flatMap((group) =>
    group.entries.map(([key]) => [
      key,
      "--" + key.replace(/[A-Z]/g, (c) => "-" + c.toLowerCase()),
    ]),
  ),
);
export const themeFileLimit = 256 * 1024;
export const customThemeLimit = 50;
const hexColor = /^#[0-9a-fA-F]{6}$/;
const object = isRecord;
export function validateTheme(input: unknown): AppearanceTheme {
  const theme = input;
  if (!object(theme)) throw new Error("主题缺少亮色配置");
  const allowed = [
    "preset",
    "accent",
    "surface",
    "ink",
    "contrast",
    "opaqueWindows",
    "colors",
  ];
  if (Object.keys(theme).some((key) => !allowed.includes(key)))
    throw new Error("主题包含不支持的设置");
  if (
    ![theme.accent, theme.surface, theme.ink].every(
      (value) => typeof value === "string" && hexColor.test(value),
    )
  )
    throw new Error("颜色须为六位十六进制值，例如 #fdf6e3");
  if (
    typeof theme.preset !== "string" ||
    theme.preset.length > 80 ||
    typeof theme.contrast !== "number" ||
    !Number.isInteger(theme.contrast) ||
    theme.contrast < 0 ||
    theme.contrast > 100 ||
    typeof theme.opaqueWindows !== "boolean"
  )
    throw new Error("主题配置无效");
  if (
    theme.colors !== undefined &&
    (!object(theme.colors) ||
      Object.entries(theme.colors).some(
        ([key, value]) =>
          !Object.hasOwn(colorTokens, key) ||
          typeof value !== "string" ||
          !hexColor.test(value),
      ))
  )
    throw new Error("元素颜色无效或包含不支持的颜色项目");
  return WorkbenchAppearanceThemeSchema.parse(theme);
}
export function validateThemeName(name: unknown) {
  if (typeof name !== "string" || !name.trim() || name.trim().length > 80)
    throw new Error("请输入 1–80 个字符的主题名称");
  return name.trim();
}
export function themeDocument(
  name: unknown,
  appearance: { lightTheme: unknown },
) {
  return {
    format: "eden-theme",
    version: 2,
    name: validateThemeName(name),
    lightTheme: validateTheme(appearance.lightTheme),
  };
}
export function parseThemeDocument(text: string) {
  if (new TextEncoder().encode(text).length > themeFileLimit)
    throw new Error("主题文件不能超过 256 KB");
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    throw new Error("无法读取主题，请选择有效的 JSON 文件");
  }
  if (!object(data) || data.format !== "eden-theme" || data.version !== 2)
    throw new Error(
      "不支持的主题格式或版本，请使用 ClaudeEden 导出的主题文件",
    );
  if (
    Object.keys(data).some(
      (key) => !["format", "version", "name", "lightTheme"].includes(key),
    )
  )
    throw new Error("主题文件包含不支持的设置");
  return themeDocument(data.name, { lightTheme: data.lightTheme });
}
export function sameTheme(
  left: { lightTheme: unknown },
  right: { lightTheme: unknown },
) {
  const canonical = (theme: unknown) => {
    const valid = validateTheme(theme);
    return JSON.stringify([
      valid.preset,
      valid.accent,
      valid.surface,
      valid.ink,
      valid.contrast,
      valid.opaqueWindows,
      Object.entries(valid.colors || {}).sort(),
    ]);
  };
  return canonical(left.lightTheme) === canonical(right.lightTheme);
}
