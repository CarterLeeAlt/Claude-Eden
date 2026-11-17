import type { ReactNode } from "react";
import type * as Wire from "./contracts.generated.ts";
export type Settings = Wire.WorkbenchSettings;
export type Job = Wire.WorkbenchJob;
export type ClientContext = Wire.WorkbenchClientContext;
export type AppearanceTheme = Wire.WorkbenchAppearanceTheme;
export type Appearance = Omit<Wire.WorkbenchAppearance, "customThemes"> & {
  customThemes: Wire.WorkbenchSavedAppearanceTheme[];
};
export type SavedTheme = Wire.WorkbenchSavedAppearanceTheme;
export type ProjectGroup = Wire.WorkbenchProjectGroup;
export type Session = Wire.WorkbenchCliSessionSummary & {
  claudeAccountNumber?: number;
};
export type ClaudeAccount = Wire.WorkbenchClaudeAccount;
export type Notify = (message: string) => void;
export type DirtyChange = (dirty: boolean) => void;
export type Side = "left" | "right";
export interface Layout {
  active: string;
  other?: string;
  side?: Side;
}
export interface ConfirmOptions {
  title: string;
  body: ReactNode;
  confirm: string;
  secondary?: string;
  pendingLabel?: string;
  anchor?: HTMLElement | null;
  submit?: () => Promise<unknown>;
}
export type ConfirmResult = boolean | "secondary";
export type Confirm = (options: ConfirmOptions) => Promise<ConfirmResult>;
export interface Confirmation extends ConfirmOptions {
  resolve: (value: ConfirmResult) => unknown;
  pending?: boolean;
  error?: string;
}
export interface TerminalSession {
  id: string;
  title: string;
  workspace: string;
  kind: string;
  closed: boolean;
  claude_account_id?: string;
  claude_account_source?: string;
  cli_session_id?: string;
  cli_starting?: boolean;
  metadata_set?: boolean;
  pinned?: boolean;
  custom_title?: string;
}
export interface Tab {
  id: string;
  kind: string;
  title?: string;
  url?: string;
  workspace?: string;
  incognito?: boolean;
  loading?: boolean;
  canGoBack?: boolean;
  canGoForward?: boolean;
  closed?: boolean;
  sessionId?: string;
  terminalKind?: string;
  cli_session_id?: string;
  claude_account_id?: string;
  metadata_set?: boolean;
  custom_title?: string;
  pinned?: boolean;
  native_window?: boolean;
  login?: boolean;
  claudeLabel?: string;
  claudeAccountNumber?: number;
  historySession?: Session;
  claude_account_source?: string;
  document?: string;
  favicon?: string;
  sessionActive?: boolean;
  sessionRunningElsewhere?: boolean;
  sessionClaudeDesktop?: boolean;
  sessionPinned?: boolean;
}
export interface TabDetails {
  title?: string;
  pinned?: boolean;
}
export type TabMetadata = {
  [key: string]: TabDetails | string[] | Record<string, string[]> | undefined;
  order?: string[];
  sessionOrder?: Record<string, string[]>;
};
export type SavedTabMetadata = Record<string, TabMetadata>;
export interface SplitDrag {
  clear: () => void;
  hover: (id: string, x: number, y: number) => Side | null;
  drop: (id: string, x: number, y: number) => boolean;
}
export interface ClipboardContent {
  text?: string;
  png?: string;
  files?: File[];
  fileSelection?: string;
  fileCount?: number;
  revision?: number;
  kind?: string;
  x?: number;
  y?: number;
}
export type InputTarget = "browser" | "desktop" | "terminal";
export interface WindowTarget {
  pid?: number;
  window?: string;
  surface?: string;
}
export interface RemoteControls {
  focus: () => void;
  edit: (action: string) => boolean | Promise<unknown>;
  copy: () => unknown;
  receiveCopy: (value: CopyResult) => unknown;
  paste: () => unknown;
  pasteText: (text: string) => unknown;
  transfer: () => unknown;
}
export interface TerminalControls {
  edit: (action: string) => boolean;
}
export interface CopyResult {
  id?: string;
  text?: string;
  clipboard?: { id: string; mime: string; size: number };
  context?: string;
  document?: string;
  error?: string;
}
