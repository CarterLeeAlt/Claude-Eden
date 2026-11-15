let current = Object.freeze({ mode: "none", connection_id: "" });

export function setClientContext(value) {
  current = Object.freeze({ ...value });
}

export const getClientContext = () => current;

export function assertConnection(connectionID) {
  if (current.connection_id !== connectionID) {
    const error = new Error("本机工作台连接已变化，请重新操作。");
    error.code = "connection_changed";
    throw error;
  }
}

export function connectionHeaders(path, context = current) {
  if (path === "/session" || path.startsWith("/client/")) return {};
  return { "X-Eden-Connection": context.connection_id || "disconnected" };
}

export function runtimeURL(path, context = current) {
  const url = new URL("/api" + path, globalThis.location?.origin || "http://localhost");
  url.searchParams.set("connection", context.connection_id || "disconnected");
  return url.pathname + url.search;
}

export function runtimeSocketURL(path, context = current) {
  return `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}${runtimeURL(path, context)}`;
}
