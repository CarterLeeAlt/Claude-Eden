import { WorkbenchError } from "./errors.ts";
import type { ClientContext } from "./types.ts";
let current: Readonly<ClientContext> = Object.freeze({
  mode: "none",
  connection_id: "",
  client_platform: "windows",
  server_platform: "windows",
  connection: { state: "disconnected" },
});

export function setClientContext(value: ClientContext) {
  current = Object.freeze({ ...value });
}

export const getClientContext = () => current;

export function assertConnection(connectionID: string) {
  if (current.connection_id !== connectionID) {
    throw new WorkbenchError("本机工作台连接已变化，请重新操作。", {
      code: "connection_changed",
    });
  }
}

export function connectionHeaders(
  path: string,
  context = current,
): Record<string, string> {
  if (path === "/session" || path.startsWith("/client/")) return {};
  return { "X-Eden-Connection": context.connection_id || "disconnected" };
}

export function runtimeURL(path: string, context = current) {
  const url = new URL(
    "/api" + path,
    globalThis.location?.origin || "http://localhost",
  );
  url.searchParams.set("connection", context.connection_id || "disconnected");
  return url.pathname + url.search;
}

export function runtimeSocketURL(path: string, context = current) {
  return `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}${runtimeURL(path, context)}`;
}
