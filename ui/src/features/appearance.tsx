import React, {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import type { Appearance } from "../lib/types.ts";
import { errorMessage } from "../lib/errors.ts";
import { api, native, nativeAvailable } from "../lib/api.ts";
import {
  defaultAppearance,
  normalizeAppearance,
  themeVariables,
} from "../lib/appearance-state.ts";

interface AppearanceContextValue {
  appearance: Appearance;
  loading: boolean;
  saving: boolean;
  error: string;
  update: (
    next: Appearance | ((current: Appearance) => Appearance),
  ) => Promise<boolean | undefined>;
  retry: () => Promise<boolean | undefined>;
}
const AppearanceContext = createContext<AppearanceContextValue | null>(null);
export const useAppearance = () => {
  const value = useContext(AppearanceContext);
  if (!value) throw new Error("外观设置需要 AppearanceProvider。");
  return value;
};
export function AppearanceProvider({ children }: React.PropsWithChildren) {
  const [appearance, setAppearance] = useState(() => {
    try {
      return normalizeAppearance(
        JSON.parse(localStorage.getItem("eden-appearance") ?? "null") ||
          defaultAppearance,
      );
    } catch {
      return defaultAppearance;
    }
  });
  const [systemReduced, setSystemReduced] = useState(
    matchMedia("(prefers-reduced-motion: reduce)").matches,
  );
  const [loading, setLoading] = useState(true),
    [saving, setSaving] = useState(false),
    [error, setError] = useState("");
  const revision = useRef(0),
    queue = useRef<Promise<boolean | undefined>>(Promise.resolve(undefined)),
    current = useRef(appearance);
  const theme = appearance.lightTheme;
  current.current = appearance;
  useEffect(() => {
    let live = true;
    const load = () =>
      api("/appearance")
        .then((value) => {
          if (live) {
            setAppearance(normalizeAppearance(value));
            localStorage.setItem("eden-appearance", JSON.stringify(value));
          }
        })
        .catch((e) => {
          if (live) setError(errorMessage(e));
        })
        .finally(() => {
          if (live) setLoading(false);
        });
    if (location.hash)
      window.addEventListener("eden-session-ready", load, { once: true });
    else load();
    const reduced = matchMedia("(prefers-reduced-motion: reduce)");
    const onReduced = () => setSystemReduced(reduced.matches);
    reduced.addEventListener("change", onReduced);
    return () => {
      live = false;
      window.removeEventListener("eden-session-ready", load);
      reduced.removeEventListener("change", onReduced);
    };
  }, []);
  useEffect(() => {
    const root = document.documentElement;
    const variables = themeVariables(theme);
    for (const [key, value] of Object.entries(variables))
      root.style.setProperty(key, value);
    root.style.setProperty("--ui-font-size", appearance.uiFontSize + "px");
    root.style.setProperty("--code-font-size", appearance.codeFontSize + "px");
    root.style.setProperty(
      "--control-cursor",
      appearance.pointerCursors ? "pointer" : "default",
    );
    root.dataset.translucent = String(!theme.opaqueWindows);
    root.dataset.fontSmoothing = String(appearance.fontSmoothing);
    root.dataset.reducedMotion = String(
      appearance.reducedMotion === "on" ||
        (appearance.reducedMotion === "system" && systemReduced),
    );
    if (nativeAvailable())
      native("appearance", {
        background: variables["--sidebar"],
        foreground: variables["--sidebar-ink"],
        translucent: !theme.opaqueWindows,
      }).catch(() =>
        setError(
          "外观已应用到工作台，原生窗口未能同步，请重新打开 Claude Eden。",
        ),
      );
  }, [appearance, systemReduced]);
  const update = (next: Appearance | ((current: Appearance) => Appearance)) => {
    next = normalizeAppearance(
      typeof next === "function" ? next(current.current) : next,
    );
    current.current = next;
    setAppearance(next);
    setSaving(true);
    setError("");
    const id = ++revision.current;
    queue.current = queue.current
      .catch(() => {})
      .then(async () => {
        if (id !== revision.current) return;
        try {
          await api("/appearance", next);
          localStorage.setItem("eden-appearance", JSON.stringify(next));
          if (id === revision.current) setSaving(false);
          return true;
        } catch (e) {
          if (id === revision.current) {
            setError(errorMessage(e));
            setSaving(false);
          }
          return false;
        }
      });
    return queue.current;
  };
  return (
    <AppearanceContext.Provider
      value={{
        appearance,
        loading,
        saving,
        error,
        update,
        retry: () => update(current.current),
      }}
    >
      {children}
    </AppearanceContext.Provider>
  );
}
