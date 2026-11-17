import { lazy, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { installDiagnostics } from "../lib/diagnostics.ts";
import { DiagnosticBoundary } from "../features/diagnostics.tsx";
import { LocalWorkbenchRoot } from "../features/local-workbench.tsx";
import { native } from "../lib/api.ts";
import "../styles/style.css";
import { IconHints } from "../components/ui/icon-hints.tsx";
import { onDesktopEvent } from "../lib/desktop-runtime.ts";

const Workbench = lazy(async () => {
  // xterm measures cells when it opens; load the bundled faces first.
  await Promise.all(
    [
      ['400 14px "Noto Sans SC"', "工作台"],
      ['600 14px "Noto Sans SC"', "工作台"],
      ['400 13px "JetBrains Mono"', "Claude 2186"],
      ['700 13px "JetBrains Mono"', "Claude 2186"],
      ['italic 400 13px "JetBrains Mono"', "Claude 2186"],
      ['italic 700 13px "JetBrains Mono"', "Claude 2186"],
      ['400 13px "Noto Sans Mono CJK SC"', "终端，。한국어"],
      ['700 13px "Noto Sans Mono CJK SC"', "终端，。한국어"],
    ].map(async ([font, text]) => {
      const faces = await document.fonts.load(font, text);
      if (!faces.length)
        throw new Error("内置字体未能加载，请重新安装 Claude Eden。");
    }),
  );
  return import("./App.tsx");
});

installDiagnostics();
// Keep the native menu usable while the workbench is loading or disconnected.
onDesktopEvent("eden-native-action", (command) => {
  if (window.edenWorkbenchAction) return window.edenWorkbenchAction(command);
  if (command === "quit") return native("quitNow");
});
const root = document.getElementById("root");
if (!root) throw new Error("工作台挂载节点缺失。");
createRoot(root).render(
  <DiagnosticBoundary>
    <IconHints />
    <LocalWorkbenchRoot>
      <Suspense
        fallback={
          <div className="client-loading" role="status">
            正在打开工作台…
          </div>
        }
      >
        <Workbench />
      </Suspense>
    </LocalWorkbenchRoot>
  </DiagnosticBoundary>,
);

window.addEventListener("hashchange", () => {
  if (location.hash) location.reload();
});
