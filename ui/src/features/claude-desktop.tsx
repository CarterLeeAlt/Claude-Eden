import { InlineSelect } from "../components/ui/inline-select.tsx";
import React, { useEffect, useRef, useState } from "react";
import {
  Download,
  ChevronDown,
  ChevronUp,
  Monitor,
  PanelLeftOpen,
  Power,
  RefreshCw,
} from "lucide-react";
import { BrowserUploads } from "./browser-uploads.tsx";
import { ProjectDialog } from "./project-dialog.tsx";
import { api, nativeAvailable } from "../lib/api.ts";
import { SafetyMessage } from "./safety-message.tsx";
import { DeviceCheckButton } from "./device-checks.tsx";
import { RemoteView } from "./views.tsx";
import { useWindowVisibility } from "../lib/window-visibility.ts";
import "../styles/claude-desktop.css";
import {
  WorkbenchJobSchema,
  type WorkbenchClaudeDesktopState,
} from "../lib/contracts.generated.ts";
import type { Status, Upload } from "../lib/contracts.ts";
import type { Confirm, Notify, RemoteControls } from "../lib/types.ts";
import { errorMessage } from "../lib/errors.ts";

const desktopZooms = [0.75, 0.9, 1, 1.1, 1.25, 1.5];

export function useDesktopZoom() {
  const [zoom, setZoom] = useState(() => {
    try {
      const stored = localStorage.getItem("eden-desktop-zoom");
      const value = Number(stored);
      return desktopZooms.includes(value) ? value : 1;
    } catch {
      return 1;
    }
  });
  return [zoom, setZoom] as const;
}

export function DesktopToolbar({
  desktop,
  platform,
  title = "Claude Desktop",
  zoom,
  setZoom,
  onConfirm,
  onStop,
  busy,
  refresh,
  sidebarCollapsed,
  onExpand,
  children,
}: React.PropsWithChildren<{
  desktop: WorkbenchClaudeDesktopState | null;
  platform: string;
  title?: string;
  zoom: number;
  setZoom: (value: number) => void;
  onConfirm: Confirm;
  onStop?: () => unknown;
  busy: boolean;
  refresh: () => unknown;
  sidebarCollapsed: boolean;
  onExpand: () => void;
}>) {
  const [hidden, setHidden] = useState(() => {
    try {
      return localStorage.getItem("eden-desktop-toolbar-hidden") === "true";
    } catch {
      return false;
    }
  });
  const toggleToolbar = (value: boolean) => {
    setHidden(value);
    try {
      localStorage.setItem("eden-desktop-toolbar-hidden", String(value));
    } catch {
      /* Keep the preference for this session. */
    }
  };
  if (hidden && desktop?.running) {
    return (
      <div className="desktop-toolbar-reveal">
        {sidebarCollapsed && (
          <button
            className="icon-button"
            aria-label="展开侧栏"
            title="展开侧栏"
            onClick={onExpand}
          >
            <PanelLeftOpen size={15} />
          </button>
        )}
        <button
          className="icon-button"
          aria-label="显示 Desktop 工具栏"
          title="显示 Desktop 工具栏"
          onClick={() => toggleToolbar(false)}
        >
          <ChevronDown size={15} />
        </button>
      </div>
    );
  }
  return (
    <header className="claude-desktop-toolbar">
      {sidebarCollapsed && (
        <button
          className="icon-button"
          aria-label="展开侧栏"
          onClick={onExpand}
        >
          <PanelLeftOpen size={17} />
        </button>
      )}
      <Monitor size={18} />
      <strong>{title}</strong>
      <span className="desktop-version">
        {desktop?.version ? `v${desktop.version}` : platform}
      </span>
      <span className="desktop-toolbar-spacer" />
      {children}
      <DeviceCheckButton disabled={busy || !desktop?.running} />
      {desktop?.running && (
        <label className="desktop-display-size">
          显示大小
          <InlineSelect
            aria-label="Desktop 显示大小"
            value={zoom}
            onValueChange={(next) => {
              const value = Number(next);
              setZoom(value);
              try {
                localStorage.setItem("eden-desktop-zoom", String(value));
              } catch {
                /* Session-only preference. */
              }
            }}
          >
            {desktopZooms.map((value) => (
              <option key={value} value={value}>
                {Math.round(value * 100)}%{value === 1 ? "（默认）" : ""}
              </option>
            ))}
          </InlineSelect>
        </label>
      )}
      {desktop?.running && onStop && (
        <button
          className="quiet-button"
          disabled={busy}
          onClick={async () => {
            if (
              await onConfirm({
                title: "关闭 Claude Desktop？",
                body: "请先保存应用内的工作。浏览器和 Claude Eden 终端会继续运行。",
                confirm: "关闭应用",
              })
            )
              onStop();
          }}
        >
          <Power size={15} />
          关闭应用
        </button>
      )}
      <button
        className="icon-button"
        aria-label={"刷新 " + title + " 状态"}
        disabled={busy}
        onClick={refresh}
      >
        <RefreshCw size={15} />
      </button>
      {desktop?.running && (
        <button
          className="icon-button"
          aria-label="隐藏 Desktop 工具栏"
          title="隐藏 Desktop 工具栏"
          onClick={() => toggleToolbar(true)}
        >
          <ChevronUp size={15} />
        </button>
      )}
    </header>
  );
}

export function ClaudeDesktop({
  uploads = [],
  status,
  onProjectOpened,
  visible,
  selected = visible,
  remoteRef,
  onStatus,
  onFallback,
  onConfirm,
  onDetails,
  onExpand,
  sidebarCollapsed,
}: {
  uploads?: Upload[];
  status: Status;
  onProjectOpened: (workspace: string) => Promise<unknown>;
  visible: boolean;
  selected?: boolean;
  remoteRef: React.Ref<RemoteControls>;
  onStatus: Notify;
  onFallback: (text: string, message: string) => void;
  onConfirm: Confirm;
  onDetails: () => void;
  onExpand: () => void;
  sidebarCollapsed: boolean;
}) {
  const [closedChooser, setClosedChooser] = useState("");
  const directoryRequest = uploads.find(
    (item) => item.directory && item.id !== closedChooser,
  );
  const [zoom, setZoom] = useDesktopZoom();
  const windowVisible = useWindowVisibility();
  const screenVisible = visible && windowVisible;
  const [desktop, setDesktop] = useState<WorkbenchClaudeDesktopState | null>(
    null,
  );
  const [error, setError] = useState("");
  const [statusError, setStatusError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [jobID, setJobID] = useState("");
  const opened = useRef(false);
  useEffect(() => {
    if (!selected) opened.current = false;
  }, [selected]);
  const statusRequest = useRef<Promise<WorkbenchClaudeDesktopState> | null>(
    null,
  );
  const statusRevision = useRef(0);
  const ownJob = status.job?.kind?.startsWith("claude-desktop-");
  const busy = submitting || Boolean(status.job?.running) || Boolean(jobID);
  const refresh = async (valid = () => true) => {
    const revision = statusRevision.current;
    const request = (statusRequest.current ??= api("/claude-desktop"));
    try {
      const value = await request;
      if (!valid() || revision !== statusRevision.current) return null;
      setDesktop(value);
      setStatusError("");
      return value;
    } catch (e) {
      if (valid() && revision === statusRevision.current)
        setStatusError(errorMessage(e));
      return null;
    } finally {
      if (statusRequest.current === request) statusRequest.current = null;
    }
  };
  const run = async (action: string) => {
    statusRevision.current++;
    statusRequest.current = null;
    setSubmitting(true);
    setError("");
    try {
      const job = await api(
        `/claude-desktop/${action}`,
        {},
        undefined,
        {},
        { schema: WorkbenchJobSchema },
      );
      setJobID(job.id);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setSubmitting(false);
    }
  };
  useEffect(() => {
    if (!screenVisible || !status.vm_running || status.job?.running) return;
    let disposed = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const poll = async () => {
      const value = await refresh(() => !disposed);
      if (disposed) return;
      if (value && !opened.current) {
        opened.current = true;
        if (value.installed && !value.running && !value.failed && !jobID)
          await run("start");
      }
      if (!disposed) timer = setTimeout(poll, 5000);
    };
    poll();
    return () => {
      disposed = true;
      clearTimeout(timer);
    };
  }, [screenVisible, status.vm_running, status.job?.running]);
  useEffect(() => {
    if (!jobID || status.job?.id !== jobID || status.job.running) return;
    setJobID("");
    if (status.job.error) {
      setError(status.job.error);
      return;
    }
    if (
      ["claude-desktop-install", "claude-desktop-update"].includes(
        status.job.kind,
      )
    )
      run("start");
    else refresh();
  }, [status.job]);
  return (
    <section
      className="claude-desktop-view"
      aria-label="Claude Desktop"
      hidden={!visible}
    >
      {!nativeAvailable() && (
        <BrowserUploads requests={uploads} onStatus={onStatus} />
      )}
      {directoryRequest && (
        <ProjectDialog
          key={directoryRequest.id}
          status={status}
          activate={false}
          onClose={() => {
            setClosedChooser(directoryRequest.id);
            void api("/input/files/cancel", {
              target: "desktop",
              id: directoryRequest.id,
            }).catch((e) => onStatus(errorMessage(e)));
          }}
          onOpened={async (workspace) => {
            await api("/input/files/directory", {
              target: "desktop",
              id: directoryRequest.id,
              workspace,
            });
            setClosedChooser(directoryRequest.id);
            await onProjectOpened(workspace);
          }}
        />
      )}
      <DesktopToolbar
        desktop={desktop}
        platform="Linux"
        zoom={zoom}
        setZoom={setZoom}
        onConfirm={onConfirm}
        onStop={() => run("stop")}
        busy={busy}
        refresh={() => refresh()}
        sidebarCollapsed={sidebarCollapsed}
        onExpand={onExpand}
      >
        {desktop?.installed && (
          <button
            className="quiet-button"
            disabled={busy}
            onClick={async () => {
              if (
                !desktop.running ||
                (await onConfirm({
                  title: "更新 Claude Desktop？",
                  body: "请先保存应用内的工作。更新会关闭 Claude Desktop，从官方软件源安装可用版本，再重新打开；登录资料继续保留。",
                  confirm: "更新应用",
                }))
              )
                run("update");
            }}
          >
            更新应用
          </button>
        )}
      </DesktopToolbar>
      {(error || statusError || desktop?.failed) && (
        <div className="connection-banner" role="alert">
          <SafetyMessage
            message={
              error ||
              statusError ||
              (desktop?.failure_reason === "oom-kill"
                ? "环境内存不足，Claude Desktop 已被系统关闭。请在设置中检查计算资源后重新打开。"
                : "Claude Desktop 未能启动，可重试打开或修复安装。")
            }
          />
          <button onClick={onDetails}>操作详情</button>
        </div>
      )}
      {desktop?.running && desktop.managed === false && (
        <div className="connection-banner" role="status">
          当前应用尚未接入 Claude Eden
          账户解锁。请先保存并关闭应用，再重新打开。
        </div>
      )}
      {busy && ownJob && (
        <div className="desktop-progress" role="status">
          <RefreshCw size={16} className="spin" />
          {status.job.message}
          <button className="text-button" onClick={onDetails}>
            查看进度
          </button>
        </div>
      )}
      <div className="desktop-screen">
        {desktop?.running && status.vm_running ? (
          <RemoteView
            ref={remoteRef}
            desktop
            zoom={zoom}
            visible={visible}
            keyboard={status.keyboard}
            onStatus={onStatus}
            onFallback={onFallback}
          />
        ) : (
          <div className="desktop-empty">
            <Monitor size={36} />
            <h2>
              {desktop?.installed
                ? "Claude Desktop"
                : "在 Claude Eden 中使用 Claude Desktop"}
            </h2>
            <p>
              Chat 和 Code 共用一个官方应用。在 Claude Eden
              内安装、登录并使用，项目文件来自已打开的文件夹。
            </p>
            {!desktop?.installed && (
              <p>
                首次使用时从 Anthropic 官方源下载，不增加 Claude Eden
                默认安装包体积。
              </p>
            )}
            <div className="desktop-actions">
              {desktop && (
                <button
                  className="primary"
                  disabled={busy || !status.vm_running}
                  onClick={() => run(desktop.installed ? "start" : "install")}
                >
                  {desktop.installed ? (
                    <Monitor size={16} />
                  ) : (
                    <Download size={16} />
                  )}
                  {desktop.installed ? "打开 Claude Desktop" : "下载并安装"}
                </button>
              )}
              {desktop?.installed && (
                <button
                  className="quiet-button"
                  disabled={busy}
                  onClick={() => run("install")}
                >
                  更新 / 修复安装
                </button>
              )}
              {!desktop && (
                <button
                  className="quiet-button"
                  disabled={busy}
                  onClick={() => refresh()}
                >
                  重新读取状态
                </button>
              )}
            </div>
            {desktop?.installed && (
              <p>
                Linux 版通过系统软件源更新。可点击上方“更新应用”，或在 Claude
                Eden 终端运行 sudo apt update && sudo apt install --only-upgrade
                claude-desktop。
              </p>
            )}
            <p className="desktop-note">
              关闭应用后释放图形会话资源，登录状态保留在 Claude Eden
              环境中。需要额外虚拟机的 Cowork 功能暂未适配。
            </p>
          </div>
        )}
      </div>
    </section>
  );
}
