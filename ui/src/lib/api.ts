import { reportDiagnostic } from "./diagnostics.ts";
import {
  httpErrorMessage,
  connectionErrorMessage,
  operationName,
  nativeFailureMessage,
} from "./error-message.ts";
import {
  assertConnection,
  connectionHeaders,
  getClientContext,
} from "./client-context.ts";
import { installAgentNativeConsumer } from "./agent-native.ts";
import { z } from "zod";
import { WorkbenchError, errorMessage, errorStatus } from "./errors.ts";
import {
  parseResponse,
  responseSchemaFor,
  terminalSchema,
  cookieValueSchema,
  extensionPreviewSchema,
  type ExtensionPreview,
  type ApiResponse,
  type KnownPath,
  type MutationPath,
  type MutationResponse,
} from "./contracts.ts";
import {
  nativeResults,
  type NativeAction,
  type NativeResult,
} from "./native-contracts.ts";
import type { ClientContext } from "./types.ts";
import { desktopService, nativeAvailable } from "./desktop-runtime.ts";
export { nativeAvailable } from "./desktop-runtime.ts";
let sessionRenewal: Promise<void> | null = null;
class WorkbenchUnreachable extends Error {}
const pageRequests = new WeakMap<Window, { controller: AbortController }>();
function requestSignal(caller?: AbortSignal) {
  if (
    typeof window === "undefined" ||
    typeof window.addEventListener !== "function"
  )
    return caller;
  let page = pageRequests.get(window);
  if (!page) {
    const lifetime = { controller: new AbortController() };
    pageRequests.set(window, lifetime);
    const leave = () =>
      lifetime.controller.abort(
        new DOMException("当前页面已离开，操作等待已取消。", "AbortError"),
      );
    window.addEventListener("beforeunload", leave);
    window.addEventListener("pagehide", leave);
    window.addEventListener("pageshow", () => {
      if (lifetime.controller.signal.aborted)
        lifetime.controller = new AbortController();
    });
    page = lifetime;
  }
  return caller
    ? AbortSignal.any([caller, page.controller.signal])
    : page.controller.signal;
}
async function responseError(response: Response, path: string, method = "GET") {
  const error = new WorkbenchError(
    httpErrorMessage(
      path,
      response.status,
      await response.text(),
      response.headers.get("Content-Type") || "",
      method,
    ),
  );
  error.status = response.status;
  error.recovery = response.headers.get("X-Eden-Recovery") || "";
  return error;
}
async function renewSession() {
  if (!sessionRenewal) {
    sessionRenewal = (async () => {
      const { url } = await native("refreshSession");
      const entry = new URL(url);
      if (entry.origin !== location.origin) {
        await native("reconnect");
        throw new Error("工作台地址已更新，连接恢复后请重试刚才的操作。");
      }
      const response = await fetch("/api/session", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: entry.hash.slice(1) }),
        signal: requestSignal(),
      });
      if (!response.ok) throw await responseError(response, "/session");
    })().finally(() => {
      sessionRenewal = null;
    });
  }
  return sessionRenewal;
}
interface RequestOptions<T = unknown> {
  signal?: AbortSignal;
  schema?: z.ZodType<T>;
}
export function api<T>(
  path: string,
  body: unknown,
  method: string | undefined,
  headers: Record<string, string>,
  options: RequestOptions<T> & { schema: z.ZodType<T> },
): Promise<T>;
export function api<P extends MutationPath>(
  path: P,
  body: object,
  method?: string,
  headers?: Record<string, string>,
  options?: RequestOptions,
): Promise<MutationResponse<P>>;
export function api(
  path: "/browser/cookies",
  body: { mode: "details"; id: string; [key: string]: unknown },
): Promise<z.infer<typeof cookieValueSchema>>;
export function api<P extends KnownPath>(
  path: P,
  body?: unknown,
  method?: string,
  headers?: Record<string, string>,
  options?: RequestOptions,
): Promise<ApiResponse<P>>;
export function api(
  path: "/terminals",
  body?: undefined,
  method?: string,
  headers?: Record<string, string>,
  options?: RequestOptions,
): Promise<z.infer<typeof terminalSchema>[]>;
export function api(
  path: "/terminals",
  body: unknown,
  method?: string,
  headers?: Record<string, string>,
  options?: RequestOptions,
): Promise<z.infer<typeof terminalSchema>>;
export function api(
  path: string,
  body?: unknown,
  method?: string,
  headers?: Record<string, string>,
  options?: RequestOptions,
): Promise<unknown>;
export async function api(
  path: string,
  body?: unknown,
  method?: string,
  headers: Record<string, string> = {},
  { signal: callerSignal, schema }: RequestOptions = {},
): Promise<unknown> {
  const endpointSchema =
    schema ??
    responseSchemaFor(
      path,
      method || (body === undefined ? "GET" : "POST"),
      body,
    );
  if (!endpointSchema) throw new Error(`接口 ${path} 缺少响应数据契约。`);
  const signal = requestSignal(callerSignal);
  const connection = getClientContext().connection_id;
  const options = {
    method: method || (body === undefined ? "GET" : "POST"),
    headers: {
      ...connectionHeaders(path),
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      ...headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  };
  const send = async () => {
    try {
      return await fetch("/api" + path, options);
    } catch (error) {
      if (signal?.aborted) throw signal.reason;
      reportDiagnostic("frontend", error);
      const message = connectionErrorMessage(path, options.method);
      if (error instanceof TypeError)
        throw new WorkbenchUnreachable(message, { cause: error });
      throw new Error(message, { cause: error });
    }
  };
  let response = await send();
  // Authentication fails before a handler runs, so retrying this rejected
  // request once does not repeat an accepted mutation.
  if (response.status === 401 && path !== "/session" && nativeAvailable()) {
    await renewSession();
    response = await send();
  }
  if (response.status === 423 && !path.startsWith("/client/"))
    window.dispatchEvent(new Event("eden-account-locked"));
  if (!response.ok) throw await responseError(response, path, options.method);
  let value: unknown;
  try {
    value = await response.json();
  } catch (error) {
    if (signal?.aborted) throw signal.reason;
    reportDiagnostic("frontend", error);
    throw new Error(
      `${operationName(path, options.method)}失败：后台返回的结果格式无效。请重新连接；仍出现时在设置中导出诊断包。`,
      { cause: error },
    );
  }
  if (
    path !== "/session" &&
    !path.startsWith("/agent/native") &&
    !path.startsWith("/client/")
  )
    assertConnection(connection);
  return parseResponse(endpointSchema, value, path);
}
interface UploadOptions {
  signal?: AbortSignal;
  onProgress?: (loaded: number, total: number) => void;
}
export function uploadFile(
  path: "/browser/extensions/upload",
  file: File,
  options?: UploadOptions,
): Promise<ExtensionPreview>;
export function uploadFile(
  path: string,
  file: File,
  options?: UploadOptions,
): Promise<{ path?: string; name?: string }>;
export async function uploadFile(
  path: string,
  file: File,
  { signal: callerSignal, onProgress }: UploadOptions = {},
): Promise<unknown> {
  const signal = requestSignal(callerSignal);
  const headers = connectionHeaders(path);
  const connection = getClientContext().connection_id;
  const send = () =>
    new Promise<unknown>((resolve, reject) => {
      if (signal?.aborted) {
        reject(new DOMException("上传已取消", "AbortError"));
        return;
      }
      const request = new XMLHttpRequest();
      const abort = () => request.abort();
      const cleanup = () => signal?.removeEventListener("abort", abort);
      request.open("POST", "/api" + path);
      request.setRequestHeader("Content-Type", "application/octet-stream");
      for (const [name, value] of Object.entries(headers))
        request.setRequestHeader(name, value);
      request.upload.onprogress = (event) =>
        onProgress?.(event.loaded, event.total || file.size);
      request.onerror = () => {
        cleanup();
        reject(
          new Error(
            "上传文件失败：与 ClaudeEden 后台的连接中断。请恢复连接后核对目标是否已收到文件，再重新选择文件。",
          ),
        );
      };
      request.onabort = () => {
        cleanup();
        reject(new DOMException("上传已取消", "AbortError"));
      };
      request.onload = () => {
        cleanup();
        if (request.status === 423)
          window.dispatchEvent(new Event("eden-account-locked"));
        if (request.status !== 200) {
          const error = new WorkbenchError(
            httpErrorMessage(
              path,
              request.status,
              request.responseText,
              request.getResponseHeader("Content-Type") || "",
            ),
          );
          error.status = request.status;
          reject(error);
          return;
        }
        try {
          assertConnection(connection);
        } catch (error) {
          reject(error);
          return;
        }
        try {
          const value: unknown = JSON.parse(request.responseText);
          resolve(
            path === "/browser/extensions/upload"
              ? parseResponse(extensionPreviewSchema, value, path)
              : parseResponse(
                  z.object({
                    path: z.string().optional(),
                    name: z.string().optional(),
                  }),
                  value,
                  path,
                ),
          );
        } catch {
          reject(
            new Error(
              "上传文件后未收到有效确认。请先核对目标是否已收到文件，避免重复上传；仍失败时导出诊断包。",
            ),
          );
        }
      };
      signal?.addEventListener("abort", abort, { once: true });
      request.send(file);
    });
  try {
    return await send();
  } catch (error) {
    if (errorStatus(error) !== 401 || !nativeAvailable() || signal?.aborted)
      throw error;
    await renewSession();
    return send();
  }
}
const lostLocalView = (context: ClientContext) =>
  context.desktop_gateway &&
  context.mode === "local" &&
  context.connection?.state === "failed" &&
  context.connection.error_code === "connection_lost";

async function suspendBeforeQuit(context: ClientContext) {
  if (context.mode !== "local" || lostLocalView(context)) return;
  try {
    await api("/account/suspend", {});
  } catch (error) {
    if (errorStatus(error) === 423) return;
    // The first request after CLI shutdown can fail before the next context poll.
    if (
      context.desktop_gateway &&
      [409, 502].includes(errorStatus(error) ?? 0)
    ) {
      const latest = await api("/client/context");
      if (
        latest.connection_id === context.connection_id &&
        lostLocalView(latest)
      )
        return;
    }
    throw error;
  }
}
let remoteInputOrder = Promise.resolve();
let clipboardCancellation: Promise<void> | undefined;

export function native<A extends NativeAction>(
  action: A,
  extra: Record<string, unknown> = {},
): Promise<NativeResult<A>> {
  const context = getClientContext();
  if (action === "remoteInput") {
    // Reserve the order before loading the binding. Go dispatches calls concurrently,
    // so ignoring an old reply cannot stop it from replacing the current owner.
    const result = remoteInputOrder.then(() =>
      callNative(action, extra, context),
    );
    remoteInputOrder = result.then(
      () => {},
      () => {},
    );
    return result;
  }
  if (action === "clipboardSyncCancel") {
    // Cancellation must interrupt an old read/write, not queue behind it. Only
    // cancellations are serialized; a successful one establishes the next epoch.
    const result = (clipboardCancellation ?? Promise.resolve())
      .catch(() => {})
      .then(() => callNative(action, extra, context));
    clipboardCancellation = result.then(() => {});
    void clipboardCancellation.catch(() => {});
    return result;
  }
  return callNative(action, extra, context, clipboardCancellation);
}

async function callNative<A extends NativeAction>(
  action: A,
  extra: Record<string, unknown>,
  context: Readonly<ClientContext>,
  cancellation?: Promise<void>,
): Promise<NativeResult<A>> {
  const connection = context.connection_id;
  if (action === "quitNow") {
    try {
      await suspendBeforeQuit(context);
    } catch (error) {
      if (!(error instanceof WorkbenchUnreachable)) throw error;
    }
    assertConnection(connection);
  }
  try {
    const service = await desktopService();
    if (action === "clipboardSyncRead" || action === "clipboardSyncWrite") {
      await cancellation;
      if (cancellation !== clipboardCancellation)
        throw new Error("剪贴板连接已变化，请重新操作。");
    }
    assertConnection(connection);
    const text = (key: string) => z.string().parse(extra[key]);
    const request = { connection_id: connection };
    let result: unknown;
    switch (action) {
      case "refreshSession":
        result = await service.RefreshSession();
        break;
      case "reconnect":
        result = await service.Reconnect();
        break;
      case "quitNow":
        result = await service.QuitNow();
        break;
      case "workbenchReady":
        result = await service.WorkbenchReady();
        break;
      case "workbenchFailed":
        result = await service.WorkbenchFailed(text("message"));
        break;
      case "clipboardRead":
        result = await service.ClipboardRead();
        break;
      case "clipboardReadContent":
        result = await service.ClipboardReadContent();
        break;
      case "clipboardRevision":
        result = await service.ClipboardRevision();
        break;
      case "clipboardSyncCancel":
        result = await service.ClipboardSyncCancel();
        if (result === false)
          throw new Error("本机未能取消旧剪贴板操作，剪贴板共享已暂停。");
        break;
      case "clipboardSyncRead":
        result = await service.ClipboardSyncRead(request);
        break;
      case "clipboardSyncRelease":
        result = await service.ClipboardSyncRelease({
          ...request,
          files: text("files"),
        });
        break;
      case "clipboardSyncWrite":
        result = await service.ClipboardSyncWrite({
          ...request,
          ...z
            .object({
              revision: z.number().optional(),
              text: z.string().optional(),
              png: z.string().optional(),
              files: z.string().optional(),
            })
            .parse(extra),
        });
        break;
      case "clipboardWrite":
        result = await service.ClipboardWrite(text("text"));
        break;
      case "importTheme":
        result = await service.ImportTheme();
        break;
      case "exportTheme":
        result = await service.ExportTheme({
          text: text("text"),
          filename: text("filename"),
        });
        break;
      case "saveRecoveryKey":
        result = await service.SaveRecoveryKey(text("key"));
        break;
      case "exportDiagnostics":
        result = await service.ExportDiagnostics(request);
        break;
      case "chooseFolder":
        result = await service.ChooseFolder(
          z.string().optional().parse(extra.purpose) ?? "project",
        );
        break;
      case "chooseUploadFiles":
        result = await service.ChooseUploadFiles(
          z.boolean().optional().parse(extra.multiple) ?? false,
        );
        break;
      case "uploadHostFiles":
        result = await service.UploadHostFiles({
          ...request,
          target: text("target"),
          id: text("id"),
          selection: text("selection"),
        });
        break;
      case "desktopFileAction":
        result = await service.DesktopFileAction({
          ...request,
          id: text("id"),
        });
        break;
      case "browserDownloadAction":
        result = await service.BrowserDownloadAction({
          ...request,
          id: text("id"),
          operation: text("operation"),
          name: z.string().optional().parse(extra.name) ?? "",
        });
        break;
      case "appReference":
        result = await service.AppReference(text("kind"));
        break;
      case "accountDevice":
        result = await service.AccountDevice(text("operation"));
        break;
      case "appUpdate":
        result = await service.AppUpdate(text("operation"));
        break;
      case "appearance":
        result = await service.Appearance({
          background: text("background"),
          foreground: text("foreground"),
        });
        break;
      case "remoteInput":
        result = await service.RemoteInput({
          ...z
            .object({ owner: z.string(), operation: z.string() })
            .parse(extra),
          x: z.number().optional().parse(extra.x) ?? 0,
          y: z.number().optional().parse(extra.y) ?? 0,
          height: z.number().optional().parse(extra.height) ?? 0,
          returnFocus: z.boolean().optional().parse(extra.returnFocus) ?? false,
        });
        break;
    }
    assertConnection(connection);
    const parsed = nativeResults[action].safeParse(result);
    if (!parsed.success)
      throw new Error(`本机操作 ${action} 返回的数据结构无效。`, {
        cause: parsed.error,
      });
    return parsed.data as NativeResult<A>;
  } catch (error) {
    reportDiagnostic("frontend", error);
    const original = errorMessage(error);
    const message = nativeFailureMessage(action, original);
    if (message === original) throw error;
    throw new Error(message, { cause: error });
  }
}
export async function reportStartupFailure(message: string) {
  if (!(await native("workbenchFailed", { message })))
    throw new Error("原生窗口未接受启动失败报告。");
}
export const readClipboard = () =>
  nativeAvailable() ? native("clipboardRead") : navigator.clipboard.readText();
export const writeClipboard = (text: string) =>
  nativeAvailable()
    ? native("clipboardWrite", { text })
    : navigator.clipboard.writeText(text);
export function normalizeURL(text: string) {
  const value = text.trim();
  if (!value || value === "about:blank") return "about:blank";
  if (/^https?:\/\//i.test(value)) return new URL(value).href;
  if (/^[a-z][a-z\d+.-]*:/i.test(value) && !/^localhost:\d+/i.test(value))
    throw new Error("地址仅支持 http 或 https");
  if (
    !/\s/.test(value) &&
    (value.includes(".") || /^localhost(?::\d+)?(?:\/|$)/i.test(value))
  )
    return new URL(
      (/^localhost|^127\./.test(value) ? "http://" : "https://") + value,
    ).href;
  return "https://www.google.com/search?q=" + encodeURIComponent(value);
}

installAgentNativeConsumer({
  api,
  native,
  nativeAvailable,
  getContext: getClientContext,
});
