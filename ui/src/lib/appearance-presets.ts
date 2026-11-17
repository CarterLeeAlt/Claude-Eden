// Eden adaptations of these palettes; no third-party theme files are loaded at runtime.
// VS Code: https://github.com/microsoft/vscode/tree/main/extensions/theme-defaults/themes
// Typora: https://github.com/typora/typora-default-themes
// Nord: https://www.nordtheme.com/docs/colors-and-palettes/
// Color Universal Design: https://jfly.uni-koeln.de/color/
import type { AppearanceTheme } from "./types.ts";
type Palette = Omit<AppearanceTheme, "preset" | "opaqueWindows"> & {
  opaqueWindows?: boolean;
};
const light = (
  accent: string,
  surface: string,
  ink: string,
  colors: Record<string, string> = {},
): Palette => ({
  accent,
  surface,
  ink,
  contrast: 45,
  colors,
});

export const themeCatalog = [
  {
    id: "eden",
    name: "Claude Eden",
    group: "经典",
    description: "暖米白画布、深灰文字与克制的陶土强调。",
    light: light("#a64b32", "#faf9f5", "#141413", {
      chrome: "#f3f1ea",
      sidebar: "#f3f1ea",
      toolbar: "#faf9f5",
      line: "#e8e6dc",
      muted: "#6b6a64",
      strong: "#141413",
      input: "#faf9f5",
      button: "#a64b32",
      buttonInk: "#faf9f5",
      link: "#a64b32",
      focus: "#a64b32",
      hover: "#eeece4",
      selected: "#e9e5d9",
      selection: "#edd4c7",
    }),
  },
  {
    id: "codex",
    name: "Codex",
    group: "经典",
    description: "中性背景与蓝色强调，保持 Claude Eden 的默认外观。",
    light: light("#339cff", "#ffffff", "#1a1c1f"),
  },
  {
    id: "solarized",
    name: "Solarized",
    group: "经典",
    description: "暖米白背景与青灰文字，使用低饱和度的阅读配色。",
    light: light("#b58900", "#fdf6e3", "#657b83"),
  },
  {
    id: "vscode-modern",
    name: "VS Code Modern",
    group: "编辑器",
    description: "参考 VS Code Modern，区分工作区、侧栏和输入框的层级。",
    light: light("#005fb8", "#ffffff", "#3b3b3b", {
      chrome: "#f8f8f8",
      sidebar: "#f8f8f8",
      toolbar: "#f8f8f8",
      line: "#cecece",
      strong: "#1f1f1f",
      muted: "#616161",
      hover: "#f2f2f2",
      selected: "#e8e8e8",
      selection: "#add6ff",
      button: "#005fb8",
      buttonInk: "#ffffff",
    }),
  },
  {
    id: "vscode-classic",
    name: "VS Code Classic",
    group: "编辑器",
    description: "参考经典 Light+，蓝色强调与清楚的面板分区。",
    light: light("#0066b8", "#ffffff", "#202020", {
      chrome: "#f3f3f3",
      sidebar: "#f3f3f3",
      toolbar: "#f3f3f3",
      line: "#b8b8b8",
      muted: "#5a5a5a",
      selected: "#d6ebff",
      selection: "#add6ff",
      button: "#0066b8",
      buttonInk: "#ffffff",
    }),
  },
  {
    id: "github",
    name: "GitHub",
    group: "编辑器",
    description: "清爽的白底与蓝灰配色。",
    light: light("#0969da", "#ffffff", "#24292f", {
      chrome: "#f6f8fa",
      sidebar: "#f6f8fa",
      toolbar: "#f6f8fa",
      line: "#d0d7de",
      muted: "#57606a",
      selected: "#ddf4ff",
      button: "#1a7f37",
      buttonInk: "#ffffff",
      success: "#1a7f37",
      danger: "#cf222e",
      warning: "#9a6700",
    }),
  },
  {
    id: "nord",
    name: "Nord",
    group: "编辑器",
    description: "冷调蓝灰与冰蓝强调，参考 Nord 的极地色板。",
    light: light("#375f88", "#eceff4", "#2e3440", {
      chrome: "#e5e9f0",
      sidebar: "#e5e9f0",
      toolbar: "#e5e9f0",
      line: "#9aa7b8",
      muted: "#4c566a",
      selected: "#cbd9e8",
      button: "#375f88",
      buttonInk: "#ffffff",
    }),
  },
  {
    id: "paper",
    name: "纸白 · Typora 风格",
    group: "阅读",
    description:
      "参考 Typora 的 GitHub 阅读主题，白纸、深灰文字和克制的蓝色链接。",
    light: light("#336da0", "#ffffff", "#333333", {
      chrome: "#f7f7f7",
      sidebar: "#f7f7f7",
      toolbar: "#fafafa",
      line: "#c8c8c8",
      muted: "#666666",
      strong: "#202020",
      button: "#333333",
      buttonInk: "#ffffff",
    }),
  },
  {
    id: "sepia",
    name: "暖纸 · Sepia",
    group: "阅读",
    description: "参考 Typora 的暖纸阅读风格，米色背景、棕色强调与柔和分区。",
    light: light("#80552f", "#f8f0e3", "#514437", {
      chrome: "#eee3d1",
      sidebar: "#eee3d1",
      toolbar: "#f3e9da",
      line: "#b1a18b",
      muted: "#6d5a48",
      selected: "#e1d2bb",
      button: "#80552f",
      buttonInk: "#fffaf1",
      link: "#76502e",
    }),
  },
  {
    id: "color-vision",
    name: "色觉友好 · 蓝橙",
    group: "无障碍",
    description:
      "用蓝色和橙色区分成功与错误，减少红绿依赖；状态仍有图标和文字。",
    light: {
      ...light("#005a9c", "#ffffff", "#17212b", {
        chrome: "#f2f5f8",
        sidebar: "#f2f5f8",
        toolbar: "#f2f5f8",
        chromeDark: "#e8edf2",
        strong: "#0c1722",
        muted: "#46596b",
        line: "#6b7785",
        input: "#ffffff",
        hover: "#e5edf5",
        selected: "#c9dff2",
        selection: "#c9dff2",
        button: "#005a9c",
        buttonInk: "#ffffff",
        accentInk: "#ffffff",
        link: "#005a9c",
        focus: "#005a9c",
        success: "#005a9c",
        danger: "#a63b00",
        warning: "#735700",
        inactive: "#546477",
      }),
      opaqueWindows: true,
    },
  },
  {
    id: "high-contrast",
    name: "高对比",
    group: "无障碍",
    description: "白底黑字，提高文字、边框和焦点的对比度，使用不透明侧栏。",
    light: {
      ...light("#002b66", "#ffffff", "#000000", {
        chrome: "#ffffff",
        sidebar: "#ffffff",
        toolbar: "#ffffff",
        chromeDark: "#f0f0f0",
        strong: "#000000",
        muted: "#333333",
        line: "#595959",
        input: "#ffffff",
        hover: "#ededed",
        selected: "#e0e0e0",
        selection: "#d4d4d4",
        button: "#000000",
        buttonInk: "#ffffff",
        accentInk: "#ffffff",
        link: "#002b66",
        focus: "#000000",
        success: "#003b73",
        danger: "#8a2800",
        warning: "#554000",
        inactive: "#3f3f3f",
      }),
      opaqueWindows: true,
    },
  },
];

export const presets = Object.fromEntries(
  themeCatalog.map((item) => [item.id, item.light]),
);
export const presetNames = Object.fromEntries(
  themeCatalog.map((item) => [item.id, item.name]),
);
export const presetGroups = ["经典", "编辑器", "阅读", "无障碍"];
export const presetInfo = Object.fromEntries(
  themeCatalog.map((item) => [item.id, item]),
);

export function applyThemePreset(
  current: AppearanceTheme,
  id: string,
): AppearanceTheme {
  const preset = presets[id];
  if (!preset) throw new Error("所选主题不存在");
  return {
    ...current,
    ...structuredClone(preset),
    preset: id,
    colors: structuredClone(preset.colors),
  };
}
