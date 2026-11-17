import React, {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { ArrowDownToLine, File, FolderOpen, Save, X } from "lucide-react";
import { api, native, nativeAvailable } from "../lib/api.ts";
import {
  assertConnection,
  getClientContext,
  runtimeURL,
} from "../lib/client-context.ts";
import type { Download, Upload } from "../lib/contracts.ts";
import type { WorkbenchSavedDownload } from "../lib/contracts.generated.ts";
import type { Notify } from "../lib/types.ts";
import { errorMessage, errorStatus } from "../lib/errors.ts";

export function useDownloadNotifications(
  downloads: Download[] = [],
  onStatus: Notify,
  uploading = false,
) {
  const previous = useRef(
    new Map<string, Pick<Download, "state" | "openError">>(),
  );
  const mountedAt = useRef(Date.now());
  const observed = useRef(new Set()),
    prompted = useRef(new Set()),
    queue = useRef<Promise<unknown>>(Promise.resolve());
  useEffect(() => {
    for (const item of downloads) {
      if (
        ["downloading", "saving"].includes(item.state) ||
        (item.target === "desktop" &&
          Date.parse(item.startedAt || "") >= mountedAt.current)
      )
        observed.current.add(item.id);
      if (
        item.state === "failed" &&
        previous.current.get(item.id)?.state !== "failed"
      )
        onStatus(
          `“${item.name}”未能保存：${item.error || "请在下载列表中重试"}`,
        );
      if (
        item.target === "desktop" &&
        observed.current.has(item.id) &&
        previous.current.get(item.id)?.state !== item.state
      ) {
        if (item.state === "downloading")
          onStatus(`正在下载“${item.name}”，可在浏览器右上角的下载列表查看`);
        if (item.state === "saved")
          onStatus(`“${item.name}”已保存到下载暂存目录`);
      }
      if (
        item.openError &&
        previous.current.get(item.id)?.openError !== item.openError
      )
        onStatus(`“${item.name}”${item.openError}`);
      if (
        nativeAvailable() &&
        !uploading &&
        observed.current.has(item.id) &&
        !prompted.current.has(item.id) &&
        (item.state === "saved" ||
          (item.target !== "desktop" &&
            item.state === "failed" &&
            item.retryable))
      ) {
        prompted.current.add(item.id);
        queue.current = queue.current
          .then(() => saveDownload(item))
          .catch((error) => onStatus(errorMessage(error)));
      }
    }
    previous.current = new Map(
      downloads.map((item) => [
        item.id,
        { state: item.state, openError: item.openError },
      ]),
    );
  }, [downloads, uploading]);
}

export function useDesktopFiles(
  ready: boolean,
  onStatus: Notify,
  desktopVisible: boolean,
) {
  const connection = getClientContext().connection_id;
  const [files, setFiles] = useState<{
    uploads: Upload[];
    downloads: Download[];
  }>({ uploads: [], downloads: [] });
  const viewVisible = useRef(desktopVisible);
  viewVisible.current = desktopVisible;
  const status = useRef(onStatus);
  status.current = onStatus;
  useEffect(() => {
    if (!ready) {
      setFiles({ uploads: [], downloads: [] });
      return;
    }
    let disposed = false,
      failures = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const handled = new Set();
    const poll = async () => {
      try {
        const value = await api("/input/files/desktop/requests");
        failures = 0;
        if (!disposed)
          setFiles({
            uploads: value.uploads || [],
            downloads: value.downloads || [],
          });
        for (const request of value.uploads || []) {
          if (disposed || !viewVisible.current || !nativeAvailable()) break;
          if (
            request.directory ||
            (request.kind && request.kind !== "chooser") ||
            handled.has(request.id)
          )
            continue;
          const body = { target: "desktop", id: request.id };
          try {
            assertConnection(connection);
            const selection = await native("chooseUploadFiles", {
              multiple: request.multiple,
            });
            assertConnection(connection);
            if (selection.canceled) {
              await api("/input/files/cancel", body);
            } else {
              await native("uploadHostFiles", {
                ...body,
                selection: selection.fileSelection,
              });
              await api("/input/files/finish", body);
              status.current?.("所选文件已交给 Claude Desktop");
            }
          } catch (error) {
            assertConnection(connection);
            await api("/input/files/cancel", body);
            status.current?.(errorMessage(error));
          }
          handled.add(request.id);
        }
        for (const request of value.opens || []) {
          if (disposed || handled.has(request.id)) continue;
          handled.add(request.id);
          try {
            assertConnection(connection);
            if (!nativeAvailable()) {
              await api("/input/files/desktop/open", {
                id: request.id,
                stage: "complete",
                error: "请在 Claude Eden 桌面应用中打开文件",
              });
              throw new Error(
                "请在 Claude Eden 桌面应用中打开文件，或从下载列表保存文件",
              );
            }
            await native("desktopFileAction", { id: request.id });
          } catch (error) {
            if (!disposed) status.current?.(errorMessage(error));
          }
        }
      } catch (error) {
        // Closing Desktop invalidates its outstanding file choosers.
        if (!disposed) {
          setFiles((previous) => ({ ...previous, uploads: [] }));
          if (failures === 0 && errorStatus(error) !== 409)
            status.current?.(errorMessage(error));
          failures++;
        }
      }
      if (!disposed)
        timer = setTimeout(
          poll,
          Math.min(1500 * 2 ** Math.min(failures, 4), 15000),
        );
    };
    void poll();
    return () => {
      disposed = true;
      clearTimeout(timer);
    };
  }, [ready, connection]);
  return files;
}

async function saveDownload(item: Download) {
  if (!nativeAvailable()) {
    const link = document.createElement("a");
    link.href = runtimeURL(
      `/browser/files/downloads/${encodeURIComponent(item.id)}/content`,
    );
    link.download = item.name || "download";
    link.click();
    return;
  }
  const result = await native("browserDownloadAction", {
    operation: "save",
    id: item.id,
    name: item.name,
  });
  if (result?.path)
    window.dispatchEvent(
      new CustomEvent("eden-download-saved", { detail: result }),
    );
  return result;
}

export function BrowserDownloads({
  downloads = [],
  ready,
  onStatus,
  menu = false,
  anchorRef,
}: {
  downloads?: Download[];
  ready: boolean;
  onStatus: Notify;
  menu?: boolean;
  anchorRef?: React.RefObject<HTMLElement | null>;
}) {
  const panelId = useId();
  const trigger = useRef<HTMLButtonElement | null>(null),
    panel = useRef<HTMLElement | null>(null);
  const [open, setOpen] = useState(false),
    [retrying, setRetrying] = useState("");
  const [saved, setSaved] = useState<WorkbenchSavedDownload[]>([]);
  useLayoutEffect(() => {
    if (!open) return;
    const content = panel.current;
    if (!content) return;
    const anchor = anchorRef?.current || trigger.current;
    const restoreFocus = () => {
      setOpen(false);
      anchor?.focus();
    };
    const position = () => {
      const toolbar = anchor?.closest(".navigation-row");
      const bounds = toolbar?.getBoundingClientRect();
      const width = content.getBoundingClientRect().width;
      const top = Math.min(
        bounds ? bounds.bottom + 8 : 56,
        Math.max(12, window.innerHeight - 160),
      );
      content.style.top = `${top}px`;
      content.style.left = `${Math.max(12, Math.min((bounds?.right || window.innerWidth) - width - 12, window.innerWidth - width - 12))}px`;
      content.style.maxHeight = `${Math.max(0, window.innerHeight - top - 12)}px`;
    };
    position();
    content.querySelector("button")?.focus();
    const dismiss = (event: Event) => {
      if (
        !content.contains(event.target as Node | null) &&
        !anchor?.contains(event.target as Node | null)
      )
        setOpen(false);
    };
    const key = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      restoreFocus();
    };
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("focusin", dismiss);
    document.addEventListener("keydown", key, true);
    document.addEventListener("eden-close-modal", restoreFocus);
    window.addEventListener("resize", position);
    return () => {
      document.removeEventListener("pointerdown", dismiss);
      document.removeEventListener("focusin", dismiss);
      document.removeEventListener("keydown", key, true);
      document.removeEventListener("eden-close-modal", restoreFocus);
      window.removeEventListener("resize", position);
    };
  }, [open, anchorRef]);
  useEffect(() => {
    const refresh = () =>
      api("/browser/files/saved")
        .then(setSaved)
        .catch(() => {});
    refresh();
    window.addEventListener("eden-download-saved", refresh);
    return () => window.removeEventListener("eden-download-saved", refresh);
  }, [open]);
  const items: (Download & { localSaved?: boolean })[] = downloads.map(
    (item) => {
      const local = saved.find((entry) => entry.id === item.id);
      return local
        ? {
            ...item,
            ...local,
            localSaved: true,
            state: "saved",
            retryable: false,
            error: "",
          }
        : item;
    },
  );
  for (const item of saved)
    if (!items.some((entry) => entry.id === item.id))
      items.push({ ...item, localSaved: true, state: "saved" });
  const failed = items.some(
    (item) => item.state === "failed" || item.openError,
  );
  const labels: Record<string, string> = {
    downloading: "正在下载",
    saving: "正在保存到本机",
    saved: "已保存到本机",
    failed: "保存失败",
    canceled: "下载已取消",
  };
  return (
    <>
      <button
        ref={trigger}
        className={
          menu
            ? "browser-menu-item browser-download-button"
            : "icon-button browser-download-button"
        }
        role={menu ? "menuitem" : undefined}
        aria-label="浏览器下载"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? panelId : undefined}
        title={failed ? "有下载需要处理，点击查看" : "下载文件"}
        onClick={() => setOpen((value) => !value)}
      >
        <ArrowDownToLine size={16} />
        {failed && !menu && (
          <span className="browser-download-warning" aria-hidden="true" />
        )}
        {menu && <span>下载</span>}
      </button>
      {open &&
        createPortal(
          <section
            ref={panel}
            id={panelId}
            className="browser-download-panel"
            role="dialog"
            aria-label="下载文件"
            data-browser-dialog
          >
            <header className="browser-download-heading">
              <h2>下载</h2>
              <button
                className="icon-button"
                aria-label="关闭面板"
                title="关闭下载"
                onClick={() => {
                  setOpen(false);
                  (anchorRef?.current || trigger.current)?.focus();
                }}
              >
                <X size={17} />
              </button>
            </header>
            <div className="browser-download-list">
              {!items.length && (
                <div className="browser-download-empty">
                  <ArrowDownToLine size={28} />
                  <strong>暂无下载</strong>
                  <p>下载的文件会显示在这里。</p>
                </div>
              )}
              {items.map((item) => (
                <article key={item.id} className="browser-download-item">
                  <File
                    size={22}
                    className="browser-download-file-icon"
                    aria-hidden="true"
                  />
                  <div className="browser-download-content">
                    <strong title={item.name}>{item.name}</strong>
                    <p className="download-status">
                      <span className="download-state">
                        {item.localSaved
                          ? "已保存到本机"
                          : labels[item.state] || "状态待确认"}
                        {item.state === "downloading" &&
                          (item.totalBytes || 0) > 0 &&
                          ` · ${Math.min(100, Math.floor(((item.receivedBytes || 0) / item.totalBytes!) * 100))}%`}
                      </span>
                      {item.path && (
                        <span className="download-path" title={item.path}>
                          {item.path}
                        </span>
                      )}
                    </p>
                    {item.state === "downloading" && (
                      <progress
                        aria-label={`${item.name} 下载进度`}
                        value={
                          (item.totalBytes || 0) > 0
                            ? item.receivedBytes
                            : undefined
                        }
                        max={(item.totalBytes || 0) > 0 ? item.totalBytes : 1}
                      />
                    )}
                    {item.error && <p role="status">{item.error}</p>}
                    {item.openError && <p role="status">{item.openError}</p>}
                    {(item.state === "saved" || item.retryable) && (
                      <div className="download-actions">
                        {(item.state === "saved" ||
                          item.target !== "desktop") && (
                          <button
                            className="download-text-action"
                            disabled={Boolean(retrying)}
                            onClick={async () => {
                              setRetrying(item.id);
                              try {
                                await saveDownload(item);
                              } catch (error) {
                                onStatus(errorMessage(error));
                              } finally {
                                setRetrying("");
                              }
                            }}
                          >
                            <Save size={13} />
                            {retrying === item.id ? "正在保存…" : "另存为…"}
                          </button>
                        )}
                        {nativeAvailable() &&
                          item.state === "saved" &&
                          [
                            ["open", "打开文件"],
                            ["reveal", "在文件夹中显示"],
                          ].map(([operation, label]) => (
                            <button
                              className={
                                operation === "open"
                                  ? "download-text-action"
                                  : "icon-button download-reveal"
                              }
                              key={operation}
                              aria-label={label}
                              title={label}
                              onClick={async () => {
                                try {
                                  await native("browserDownloadAction", {
                                    operation,
                                    id: item.id,
                                  });
                                } catch (error) {
                                  onStatus(errorMessage(error));
                                }
                              }}
                            >
                              {operation === "reveal" ? (
                                <FolderOpen size={16} />
                              ) : (
                                label
                              )}
                            </button>
                          ))}
                        {item.retryable && (
                          <button
                            className="download-text-action"
                            disabled={!ready || Boolean(retrying)}
                            onClick={async () => {
                              setRetrying(item.id);
                              try {
                                await api(
                                  item.target === "desktop"
                                    ? "/input/files/download-retry"
                                    : "/browser/download-retry",
                                  {
                                    id: item.id,
                                    ...(item.target === "desktop"
                                      ? { target: "desktop" }
                                      : {}),
                                  },
                                );
                              } catch (error) {
                                onStatus(errorMessage(error));
                              } finally {
                                setRetrying("");
                              }
                            }}
                          >
                            {retrying === item.id ? "正在重试…" : "重试保存"}
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                </article>
              ))}
            </div>
          </section>,
          document.body,
        )}
    </>
  );
}
