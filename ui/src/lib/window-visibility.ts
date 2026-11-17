import { useSyncExternalStore } from "react";
import { onDesktopEvent } from "./desktop-runtime.ts";

const visible = () =>
  document.visibilityState !== "hidden" && window.edenWindowVisible !== false;

function subscribe(notify: () => void) {
  document.addEventListener("visibilitychange", notify);
  const remove = onDesktopEvent("eden-window-visibility", (value) => {
    window.edenWindowVisible = value;
    notify();
  });
  return () => {
    document.removeEventListener("visibilitychange", notify);
    remove();
  };
}

export function useWindowVisibility() {
  return useSyncExternalStore(subscribe, visible);
}
