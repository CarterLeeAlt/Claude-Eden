import { z } from "zod";
import {
  WorkbenchError,
  errorMessage,
  errorStatus,
  isRecord,
} from "./errors.ts";
import {
  nativeResults,
  type Native,
  type NativeAction,
} from "./native-contracts.ts";
import type { ClientContext } from "./types.ts";
type PreferenceStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;
interface NativeItem {
  id: string;
  connection_id: string;
  request: Record<string, unknown>;
}
interface NativeDependencies {
  native: Native;
  getContext: () => ClientContext;
  storage: PreferenceStorage;
}
interface ConsumerDependencies extends NativeDependencies {
  api: (
    path: string,
    body?: unknown,
    method?: string,
    headers?: Record<string, string>,
  ) => Promise<unknown>;
  nativeAvailable: () => boolean;
  reload: () => void;
  getOrigin?: () => string;
  schedule?: typeof setTimeout;
  cancel?: typeof clearTimeout;
  interval?: number;
  onError?: (error: unknown) => void;
}
const nativeItemSchema = z.object({
  id: z.string(),
  connection_id: z.string(),
  request: z.record(z.string(), z.unknown()),
});
// External agents use the same native bridge as this window. No script or
// expression is accepted: native actions retain their existing platform checks.

export const agentClientPreferences = Object.freeze([
  {
    key: "eden-active",
    scoped: true,
    format: "string",
    description: "当前标签 ID",
  },
  {
    key: "eden-sidebar-collapsed",
    scoped: true,
    format: "JSON boolean",
    description: "侧栏收起状态",
  },
  {
    key: "eden-browser-tab-layout",
    scoped: true,
    format: 'JSON "horizontal" | "vertical"',
    description: "浏览器标签排列",
  },
  {
    key: "eden-tab-layout",
    scoped: true,
    format: "JSON object",
    description: "按工作区保存的标签名称、固定与顺序",
  },
  {
    key: "eden.audio",
    scoped: false,
    format: "JSON {muted:boolean, volume:number 0..1}",
    description: "当前客户端的声音偏好",
  },
  {
    key: "eden-desktop-zoom",
    scoped: false,
    format: "string number: 0.75, 0.9, 1, 1.1, 1.25, 1.5",
    description: "Claude Desktop 显示比例",
  },
]);

const object = isRecord;
function validPreference(key: string, raw: unknown) {
  if (raw === null) return true;
  if (typeof raw !== "string") return false;
  if (key === "eden-active") return true;
  if (key === "eden-desktop-zoom")
    return [0.75, 0.9, 1, 1.1, 1.25, 1.5].includes(Number(raw));
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return false;
  }
  if (key === "eden-sidebar-collapsed") return typeof value === "boolean";
  if (key === "eden-browser-tab-layout")
    return (
      typeof value === "string" && ["horizontal", "vertical"].includes(value)
    );
  if (key === "eden.audio")
    return (
      object(value) &&
      typeof value.muted === "boolean" &&
      typeof value.volume === "number" &&
      Number.isFinite(value.volume) &&
      value.volume >= 0 &&
      value.volume <= 1 &&
      Object.keys(value).every((field) => ["muted", "volume"].includes(field))
    );
  if (key === "eden-tab-layout") {
    if (!object(value)) return false;
    return Object.values(value).every(
      (workspace) =>
        object(workspace) &&
        Object.entries(workspace).every(([id, metadata]) => {
          if (id === "order")
            return (
              Array.isArray(metadata) &&
              metadata.every((item) => typeof item === "string")
            );
          if (id === "sessionOrder")
            return (
              object(metadata) &&
              Object.values(metadata).every(
                (order) =>
                  Array.isArray(order) &&
                  order.every((item) => typeof item === "string"),
              )
            );
          return (
            object(metadata) &&
            Object.entries(metadata).every(([field, item]) =>
              field === "title"
                ? typeof item === "string"
                : field === "pinned" && typeof item === "boolean",
            )
          );
        }),
    );
  }
  return false;
}

function connectionChanged() {
  return new WorkbenchError(
    "本机工作台连接已变化；请核对原生操作结果后重新操作。",
    { code: "connection_changed" },
  );
}

export function clientPreferences(
  request: Record<string, unknown>,
  context: ClientContext,
  storage: PreferenceStorage,
) {
  if (
    typeof request.operation !== "string" ||
    !["get", "set"].includes(request.operation)
  )
    throw new Error("客户端偏好操作须为 get 或 set。");
  const keys = new Map(agentClientPreferences.map((item) => [item.key, item]));
  if (request.operation === "set") {
    if (!object(request.values) || !Object.keys(request.values).length)
      throw new Error("请提供要修改的客户端偏好 values。");
    // Validate the whole patch before changing any storage entry.
    for (const [key, raw] of Object.entries(request.values)) {
      if (!keys.has(key))
        throw new Error(
          "客户端偏好包含不支持的键。外观请通过 appearance 接口修改。",
        );
      if (!validPreference(key, raw))
        throw new Error(
          "客户端偏好格式无效，请保留对应设置使用的原始字符串格式。",
        );
    }
    const previous = new Map(
      Object.keys(request.values).map((key) => [key, storage.getItem(key)]),
    );
    try {
      for (const [key, raw] of Object.entries(request.values)) {
        if (raw === null) storage.removeItem(key);
        else storage.setItem(key, raw as string);
      }
    } catch (error) {
      for (const [key, raw] of previous) {
        try {
          if (raw === null) storage.removeItem(key);
          else storage.setItem(key, raw);
        } catch {
          /* Keep the original storage failure. */
        }
      }
      throw error;
    }
  }
  const available = agentClientPreferences;
  return {
    connection_id: context.connection_id,
    values: Object.fromEntries(
      available.map((item) => [item.key, storage.getItem(item.key)]),
    ),
    storage_keys: Object.fromEntries(
      available.map((item) => [item.key, item.key]),
    ),
    fields: available,
    reload_required: request.operation === "set",
  };
}

export async function executeAgentNative(
  item: NativeItem,
  { native, getContext, storage }: NativeDependencies,
): Promise<unknown> {
  const context = getContext();
  if (!item.connection_id || item.connection_id !== context.connection_id)
    throw connectionChanged();
  const request = item.request;
  if (!object(request) || typeof request.action !== "string")
    throw new Error("原生请求格式无效。");
  let result;
  if (request.action === "clientPreferences")
    result = clientPreferences(request, context, storage);
  else {
    const { action, connection_id: ignoredConnection, ...extra } = request;
    void ignoredConnection;
    if (!Object.hasOwn(nativeResults, action))
      throw new Error("原生请求包含不支持的操作。");
    result = await native(action as NativeAction, extra);
    // Native session tickets belong to the transport. An agent can request a
    // refresh, but never receives the launch URL or its secret fragment.
    if (action === "refreshSession") result = { refreshed: true };
  }
  if (getContext().connection_id !== item.connection_id)
    throw connectionChanged();
  return result === undefined ? null : result;
}

export function createAgentNativeConsumer({
  api,
  native,
  nativeAvailable,
  getContext,
  storage,
  reload,
  getOrigin = () => globalThis.location?.origin,
  schedule = setTimeout,
  cancel = clearTimeout,
  interval = 750,
  onError = () => {},
}: ConsumerDependencies) {
  let stopped = true;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let running = false;
  let completion:
    | {
        id: string;
        body: { result?: unknown; error?: string };
        reload?: boolean;
      }
    | undefined;
  let proof: string | undefined;
  const registerWindow = async () => {
    const entry = await native("refreshSession");
    let url;
    try {
      url = new URL(entry?.url);
    } catch {
      throw new Error("无法验证原生窗口入口，请重新连接 Claude Eden。");
    }
    if (url.origin !== getOrigin() || !url.hash.slice(1))
      throw new Error(
        "原生窗口入口已改变；未把验证票据发送到其他地址，请重新连接 Claude Eden。",
      );
    const value = z
      .object({ window_proof: z.string() })
      .parse(await api("/agent/native/window", { ticket: url.hash.slice(1) }));
    if (typeof value?.window_proof !== "string" || !value.window_proof)
      throw new Error("原生窗口认证未完成，请重新连接 Claude Eden。");
    proof = value.window_proof;
  };
  const poll = async () => {
    if (stopped || running) return;
    if (!nativeAvailable()) {
      timer = schedule(poll, interval);
      return;
    }
    running = true;
    try {
      if (!proof) await registerWindow();
      if (!completion) {
        if (stopped) return;
        const raw = await api(
          "/agent/native?consumer=window",
          undefined,
          undefined,
          { "X-Eden-Agent-Window": proof! },
        );
        const item = raw === null ? null : nativeItemSchema.parse(raw);
        if (item?.id) {
          if (stopped) {
            completion = {
              id: item.id,
              body: { error: "桌面窗口已关闭，原生操作未执行。" },
            };
          } else {
            try {
              const result = await executeAgentNative(item, {
                native,
                getContext,
                storage,
              });
              completion = {
                id: item.id,
                body: { result },
                reload: isRecord(result) && result.reload_required === true,
              };
            } catch (error) {
              completion = {
                id: item.id,
                body: {
                  error:
                    errorMessage(error) || "原生操作未完成，请核对当前窗口。",
                },
              };
            }
          }
        }
      }
      if (completion) {
        const pending = completion;
        await api(
          `/agent/native/${encodeURIComponent(pending.id)}/complete`,
          pending.body,
          undefined,
          { "X-Eden-Agent-Window": proof! },
        );
        completion = undefined;
        if (pending.reload && !stopped) {
          stopped = true;
          reload();
        }
      }
    } catch (error) {
      // Only a completion receipt is retried. An accepted native action is
      // never re-executed after a lost connection or lost response.
      if (errorStatus(error) === 401 || errorStatus(error) === 403)
        proof = undefined;
      onError(error);
    } finally {
      running = false;
      if (!stopped) timer = schedule(poll, interval);
    }
  };
  return {
    start() {
      if (!stopped) return;
      stopped = false;
      void poll();
    },
    stop() {
      stopped = true;
      if (timer !== undefined) cancel(timer);
    },
  };
}

let installedConsumer: ReturnType<typeof createAgentNativeConsumer> | undefined;
export function installAgentNativeConsumer(
  dependencies: Pick<
    ConsumerDependencies,
    "api" | "native" | "nativeAvailable" | "getContext"
  >,
  environment = globalThis,
) {
  if (installedConsumer || !environment.document || !environment.window) return;
  const consumer = createAgentNativeConsumer({
    ...dependencies,
    storage: {
      getItem: (key) => environment.localStorage.getItem(key),
      setItem: (key, value) => environment.localStorage.setItem(key, value),
      removeItem: (key) => environment.localStorage.removeItem(key),
    },
    reload: () => environment.location.reload(),
    getOrigin: () => environment.location.origin,
  });
  installedConsumer = consumer;
  const start = () => {
    if (dependencies.nativeAvailable()) consumer.start();
  };
  environment.window.addEventListener("eden-session-ready", start);
  environment.window.addEventListener("pagehide", () => consumer.stop());
  environment.window.addEventListener("pageshow", start);
  // A fresh launch URL must first be exchanged by ClientRoot/AccountProvider.
  // Existing windows without a fragment already hold their session cookie.
  if (!environment.location?.hash) start();
}
