import type {
  Layout,
  Tab,
  TabMetadata,
  TabDetails,
  ProjectGroup,
  Side,
} from "./types.ts";
import type { Status } from "./contracts.ts";
export function readLocal<T>(key: string, fallback: T): T {
  try {
    return JSON.parse(localStorage.getItem(key) ?? "null") ?? fallback;
  } catch {
    return fallback;
  }
}

export function orderedTabs<T extends Tab>(tabs: T[], saved: TabMetadata) {
  const order = saved.order || [];
  return [...tabs].sort((a, b) => {
    const pinned =
      Number(Boolean(tabDetails(saved, b.id).pinned)) -
      Number(Boolean(tabDetails(saved, a.id).pinned));
    if (pinned) return pinned;
    const index = (id: string) =>
      order.includes(id) ? order.indexOf(id) : order.length;
    return index(a.id) - index(b.id);
  });
}
export function tabDetails(saved: TabMetadata, id: string): TabDetails {
  const entry = saved[id];
  if (!entry || Array.isArray(entry)) return {};
  return {
    title: typeof entry.title === "string" ? entry.title : undefined,
    pinned: typeof entry.pinned === "boolean" ? entry.pinned : undefined,
  };
}

const folderName = (path?: string) =>
  path?.split(/[\\/]/).filter(Boolean).at(-1);

export function projectGroups(
  status: Pick<Status, "project_groups" | "projects"> | null,
) {
  return (
    status?.project_groups ||
    (status?.projects || []).map((path) => ({
      workspace: path,
      name: projectName(path),
      folders: [path],
    }))
  );
}

export function projectName(
  path?: string,
  groups: ProjectGroup[] = [],
): string {
  const group = groups.find(
    (item) =>
      item.workspace === path ||
      (path !== undefined && item.folders.includes(path)),
  );
  if (group?.name) return group.name;
  return folderName(path) || "新项目";
}

export function tabName(tab: Tab, saved: TabMetadata, project = "新项目") {
  return (
    (tab.incognito ? "无痕 · " : "") +
    (tabDetails(saved, tab.id).title ||
      (tab.kind === "editor"
        ? `文件 · ${tab.workspace ? projectName(tab.workspace) : project}`
        : tab.title) ||
      "新标签页")
  );
}

// The focused tab and its neighbour retain their screen positions on focus changes.
export function focusTab(layout: Layout, id: string): Layout {
  if (layout.active === id) return layout;
  if (!id) return { active: "" };
  if (layout.other === id)
    return {
      active: id,
      other: layout.active,
      side: layout.side === "left" ? "right" : "left",
    };
  if (
    layout.other &&
    id.startsWith("terminal:") !== layout.active.startsWith("terminal:") &&
    id.startsWith("terminal:") === layout.other.startsWith("terminal:")
  )
    return { ...focusTab(layout, layout.other), active: id };
  return { ...layout, active: id };
}

export function splitTab(
  layout: Layout,
  tabs: Tab[],
  id: string,
  side: Side,
): Layout {
  if (!tabs.some((tab) => tab.id === id)) return layout;
  const opposite = layout.side === side ? layout.other : layout.active;
  const other = [
    opposite,
    layout.other,
    layout.active,
    ...tabs.map((tab) => tab.id),
  ].find(
    (candidate) => candidate !== id && tabs.some((tab) => tab.id === candidate),
  );
  return other ? { active: id, other, side } : layout;
}
