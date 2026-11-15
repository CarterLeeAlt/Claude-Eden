import { useSyncExternalStore } from "react";

const visible = () =>
  document.visibilityState !== "hidden" && window.edenWindowVisible !== false;

function subscribe(notify) {
  document.addEventListener("visibilitychange", notify);
  window.addEventListener("eden-window-visibility", notify);
  return () => {
    document.removeEventListener("visibilitychange", notify);
    window.removeEventListener("eden-window-visibility", notify);
  };
}

export function useWindowVisibility() {
  return useSyncExternalStore(subscribe, visible);
}
