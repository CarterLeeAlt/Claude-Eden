import { shortcut } from "../lib/platform.ts";
import React, { useEffect, useRef, useState } from "react";
import {
  CircleAlert,
  ExternalLink,
  Folder,
  FolderOpen,
  Globe2,
  Home,
  Monitor,
  Network,
  PanelLeftClose,
  Plus,
  RefreshCw,
  Settings,
  SquareTerminal,
  UsersRound,
} from "lucide-react";
import { EnvironmentShield } from "./environment-shield.tsx";
import logo from "../logo.png";
import claudeLogo from "../claude-logo.svg";
import { projectName, tabDetails } from "../lib/workspace-state.ts";
import { WorkspaceTabs } from "./workspace-tabs.tsx";
import { projectSessionRows, sessionKey } from "../lib/session-order.ts";
import { AccountMenu } from "./account.tsx";
import { GuestSystemIcon } from "./guest-system-icon.tsx";
import type { WorkspaceTabsProps } from "./workspace-tabs.tsx";
import type {
  TabMetadata,
  Session,
  Confirm,
  Notify,
  ProjectGroup,
} from "../lib/types.ts";
import type { networkState } from "../lib/safety-state.ts";

const SESSION_PREVIEW_LIMIT = 5;

export function WorkspaceSidebar({
  workspace,
  guestSystem,
  projects,
  projectGroups = [],
  tabs,
  sessions = [],
  sessionError,
  sessionRefreshing,
  onSessionRefresh,
  onProjectRefresh,
  projectRefreshing,
  onSessionOpen,
  openingSession,
  onCancelSessionOpen,
  sessionOpenError,
  onSessionArchive,
  onSessionPin,
  onSessionRename,
  saved,
  active,
  home,
  busy,
  ready,
  running,
  runtimeLabel,
  network,
  onHome,
  onOpenProject,
  onProjectSelect,
  onSelect,
  onClose,
  onRename,
  onPin,
  onSessionReorder,
  splitDrag,
  onNewTerminal,
  terminalMenuAt,
  onBrowser,
  onTailscale,
  tailscaleOpen,
  onClaudeAccounts,
  claudeAccountsOpen,
  onClaudeWeb,
  onClaudeDesktop,
  desktopOpen,
  onEditor,
  onSettings,
  onAccountConfirm,
  onAccountStatus,
  onSafety,
  onCollapse,
  updateAction,
}: Pick<
  WorkspaceTabsProps,
  | "tabs"
  | "active"
  | "busy"
  | "ready"
  | "onSelect"
  | "onClose"
  | "onRename"
  | "onPin"
  | "splitDrag"
  | "openingSession"
  | "onCancelSessionOpen"
  | "sessionOpenError"
> & {
  workspace: string;
  guestSystem?: string;
  projects: string[];
  projectGroups?: ProjectGroup[];
  sessions?: Session[];
  sessionError: string;
  sessionRefreshing: boolean;
  onSessionRefresh: () => void;
  onProjectRefresh?: () => void;
  projectRefreshing: boolean;
  onSessionOpen: (session: Session) => unknown;
  onSessionArchive: (session: Session, anchor?: HTMLElement) => unknown;
  onSessionPin: (session: Session) => unknown;
  onSessionRename: (session: Session) => unknown;
  saved: TabMetadata;
  home: boolean;
  running: boolean;
  runtimeLabel: string;
  network: ReturnType<typeof networkState>;
  onHome: () => void;
  onOpenProject: () => void;
  onProjectSelect: (path: string) => void;
  onSessionReorder: (path: string, order: string[]) => void;
  onNewTerminal: (path: string, anchor: HTMLButtonElement) => void;
  terminalMenuAt: (path: string) => React.ReactNode;
  onBrowser: () => void;
  onTailscale: () => void;
  tailscaleOpen: boolean;
  onClaudeAccounts: () => void;
  claudeAccountsOpen: boolean;
  onClaudeWeb: () => void;
  onClaudeDesktop: () => void;
  desktopOpen: boolean;
  onEditor: (path: string) => void;
  onSettings: (section: string) => void;
  onAccountConfirm: Confirm;
  onAccountStatus: Notify;
  onSafety: () => void;
  onCollapse: () => void;
  updateAction?: React.ReactNode;
}) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [allSessionsShown, setAllSessionsShown] = useState<
    Record<string, boolean>
  >({});
  const sessionBaseline = useRef(new Map<string, number>());
  const activeFolder = tabs.find((tab) => tab.id === active)?.workspace;
  const activeWorkspace =
    projectGroups.find(
      (group) => activeFolder && group.folders.includes(activeFolder),
    )?.workspace || activeFolder;
  useEffect(() => {
    if (activeWorkspace)
      setCollapsed((previous) => ({ ...previous, [activeWorkspace]: false }));
  }, [active, activeWorkspace]);
  const healthy = ready && network.mode === "open";
  return (
    <aside className="workspace-sidebar" aria-label="项目与终端">
      <div className="sidebar-top">
        <span className="sidebar-brand">
          <img className="brand-logo" src={logo} alt="" />
          Claude Eden
          <GuestSystemIcon />
        </span>
        <button
          className="icon-button"
          onClick={onCollapse}
          aria-label="收起侧栏"
          title="收起侧栏"
        >
          <PanelLeftClose size={17} />
        </button>
      </div>
      <nav className="sidebar-navigation" aria-label="主导航">
        <button
          className={`sidebar-link ${home ? "selected" : ""}`}
          onClick={onHome}
        >
          <Home size={17} />
          工作台
        </button>
        <div
          className={`sidebar-browser-row ${!desktopOpen && !home && tabs.some((tab) => tab.id === active && tab.kind !== "terminal") ? "selected" : ""}`}
        >
          <button
            className="sidebar-link"
            onClick={onBrowser}
            disabled={!ready || busy}
            aria-label="打开浏览器"
            title="打开浏览器"
          >
            <Globe2 size={17} />
            浏览器
          </button>
          <button
            className="icon-button sidebar-claude-shortcut"
            onClick={onClaudeWeb}
            disabled={!ready || busy}
            aria-label="claude.ai"
            title="打开 claude.ai"
          >
            <img src={claudeLogo} width="18" height="18" alt="" />
          </button>
        </div>
        <button
          className={`sidebar-link ${tailscaleOpen ? "selected" : ""}`}
          aria-current={tailscaleOpen ? "page" : undefined}
          onClick={onTailscale}
          aria-label="打开 Tailscale"
        >
          <Network size={17} />
          Tailscale
        </button>
        <button
          className={`sidebar-link ${desktopOpen ? "selected" : ""}`}
          onClick={onClaudeDesktop}
          disabled={!running || busy}
          title="安装或打开 Claude Desktop"
        >
          <Monitor size={17} />
          Claude Desktop
        </button>
        <button
          className={`sidebar-link ${claudeAccountsOpen ? "selected" : ""}`}
          onClick={onClaudeAccounts}
          aria-current={claudeAccountsOpen ? "page" : undefined}
        >
          <UsersRound size={17} />
          Claude 账号管理
        </button>
      </nav>
      <div className="sidebar-scroll">
        <div className="sidebar-section-heading">
          <span>项目</span>
          <span className="sidebar-section-actions">
            {onProjectRefresh && (
              <button
                className="icon-button"
                onClick={onProjectRefresh}
                aria-label="刷新项目文件"
                title="刷新项目文件：让环境立即看到在本机上所做的改动"
                disabled={busy || !running || projectRefreshing}
              >
                <RefreshCw
                  size={14}
                  className={projectRefreshing ? "spin" : undefined}
                />
              </button>
            )}
            <button
              className="icon-button"
              onClick={() => onOpenProject()}
              aria-label="添加项目"
              title="添加项目"
              disabled={busy}
            >
              <Plus size={15} />
            </button>
          </span>
        </div>
        {sessionError && (
          <div className="sidebar-session-error">
            <div role="alert">
              <div className="sidebar-session-error-heading">
                <CircleAlert size={16} aria-hidden="true" />
                <strong>会话列表暂未更新</strong>
              </div>
              <p>{sessionError}</p>
            </div>
            <button
              onClick={onSessionRefresh}
              disabled={sessionRefreshing || busy || !running}
              aria-busy={sessionRefreshing || undefined}
            >
              <RefreshCw size={14} aria-hidden="true" />
              {sessionRefreshing ? "正在读取…" : "重新读取会话"}
            </button>
          </div>
        )}
        {projects.map((path) => {
          const group = projectGroups.find((group) => group.workspace === path);
          const folders = group?.folders || [path];
          const project = projectName(path, projectGroups);
          const rows = projectSessionRows(
            tabs.filter(
              (tab) =>
                tab.kind === "terminal" &&
                tab.workspace !== undefined &&
                folders.includes(tab.workspace),
            ),
            sessions.filter((session) => folders.includes(session.workspace)),
            saved,
            saved.sessionOrder?.[path],
            sessionBaseline.current,
          );
          const rowMetadata = { ...saved };
          for (const row of rows)
            rowMetadata[row.id] = {
              ...tabDetails(saved, row.id),
              pinned: row.sessionPinned,
            };
          const expanded = !collapsed[path];
          const showAllSessions = Boolean(allSessionsShown[path]);
          const visibleRows = showAllSessions
            ? rows
            : rows.slice(0, SESSION_PREVIEW_LIMIT);
          return (
            <section
              className="sidebar-project"
              key={path}
              aria-label={`项目 ${project}`}
            >
              <div
                className={`project-heading ${folders.includes(workspace) ? "current-project" : ""}`}
              >
                <button
                  className="project-disclosure"
                  title={folders.join("\n")}
                  aria-expanded={expanded}
                  onClick={() => {
                    setCollapsed((previous) => ({
                      ...previous,
                      [path]: expanded,
                    }));
                    setAllSessionsShown((previous) => ({
                      ...previous,
                      [path]: false,
                    }));
                    onProjectSelect(path);
                  }}
                >
                  {expanded ? <FolderOpen size={16} /> : <Folder size={16} />}
                  <span>{project}</span>
                </button>
                <button
                  className="icon-button"
                  aria-label={`打开 ${project} 的编辑器`}
                  title="在 code-server 中打开项目目录"
                  disabled={!ready || busy}
                  onClick={() => onEditor(path)}
                >
                  <ExternalLink size={15} />
                </button>
                <button
                  className="icon-button"
                  aria-label={`在 ${project} 新建终端`}
                  title="新建终端"
                  aria-haspopup="menu"
                  data-terminal-location={path}
                  disabled={!running || busy}
                  onClick={(event) => {
                    setCollapsed((previous) => ({
                      ...previous,
                      [path]: false,
                    }));
                    onNewTerminal(path, event.currentTarget);
                  }}
                >
                  <SquareTerminal size={15} />
                </button>
              </div>
              {terminalMenuAt(path)}
              {expanded && folders.length > 1 && (
                <div
                  className="sidebar-project-folders"
                  aria-label={`${project} 的文件夹`}
                >
                  {folders.map((folder, index) => (
                    <button
                      key={folder}
                      title={folder}
                      disabled={!ready || busy}
                      onClick={() => onEditor(folder)}
                      aria-label={`打开文件夹 ${projectName(folder)} 的编辑器`}
                    >
                      <Folder size={13} />
                      <span>{projectName(folder)}</span>
                      {index === 0 && <small>默认</small>}
                    </button>
                  ))}
                </div>
              )}
              <div
                className={`project-children ${expanded ? "expanded" : ""}`}
                inert={!expanded}
              >
                <div>
                  {!!rows.length && (
                    <WorkspaceTabs
                      tabs={visibleRows}
                      saved={rowMetadata}
                      openingSession={openingSession}
                      onCancelSessionOpen={onCancelSessionOpen}
                      sessionOpenError={sessionOpenError}
                      active={home || desktopOpen ? "" : active}
                      label={`${project} 的终端`}
                      busy={busy}
                      ready={ready}
                      onSelect={(id, ...args) => {
                        const row = rows.find((row) => row.id === id);
                        if (row?.historySession)
                          onSessionOpen(row.historySession);
                        else onSelect(id, ...args);
                      }}
                      onClose={(row, anchor) =>
                        row.historySession
                          ? onSessionArchive(row.historySession, anchor)
                          : onClose(row, anchor)
                      }
                      onRename={(row) =>
                        row.historySession
                          ? onSessionRename(row.historySession)
                          : onRename(row)
                      }
                      onPin={(id) => {
                        const row = rows.find((row) => row.id === id);
                        if (row?.historySession)
                          onSessionPin(row.historySession);
                        else onPin(id);
                      }}
                      onReorder={(from, to) => {
                        const targetIndex = rows.findIndex(
                          (row) => row.id === to,
                        );
                        const order = rows
                          .map((row) => row.id)
                          .filter((id) => id !== from);
                        order.splice(targetIndex, 0, from);
                        onSessionReorder(
                          path,
                          order.map((id) =>
                            sessionKey(rows.find((row) => row.id === id)!),
                          ),
                        );
                      }}
                      splitDrag={splitDrag}
                    />
                  )}
                  {rows.length > SESSION_PREVIEW_LIMIT && (
                    <button
                      className="project-session-toggle"
                      aria-expanded={showAllSessions}
                      onClick={() =>
                        setAllSessionsShown((previous) => ({
                          ...previous,
                          [path]: !showAllSessions,
                        }))
                      }
                    >
                      {showAllSessions ? "收起显示" : "展开显示"}
                    </button>
                  )}
                  {!rows.length && !running && (
                    <p className="sidebar-empty">启动后可新建终端</p>
                  )}
                </div>
              </div>
            </section>
          );
        })}
        {!projects.length && (
          <p className="sidebar-empty">打开本地文件夹开始工作</p>
        )}
      </div>
      <div className="sidebar-bottom">
        <button className="sidebar-status" onClick={onSafety}>
          <i className={`status-dot ${network.mode}`} />
          <span>{healthy ? "环境运行正常" : runtimeLabel}</span>
          <EnvironmentShield size={17} healthy={healthy} />
        </button>
        <div className="sidebar-footer-actions">
          <AccountMenu
            onSettings={onSettings}
            onConfirm={onAccountConfirm}
            onStatus={onAccountStatus}
            busy={busy}
          />
          <button
            className="icon-button"
            aria-label="设置"
            title={shortcut("设置 ⌘,")}
            onClick={() => onSettings("keyboard")}
          >
            <Settings size={17} />
          </button>
          {updateAction}
        </div>
      </div>
    </aside>
  );
}
