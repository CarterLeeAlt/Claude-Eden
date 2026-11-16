import React, { lazy, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { installDiagnostics } from "./diagnostics.mjs";
import { DiagnosticBoundary } from "./diagnostics.jsx";
import { LocalWorkbenchRoot } from "./local-workbench";
import { native } from "./api";
import "./style.css";
import { IconHints } from "./icon-hints.jsx";

const Workbench = lazy(async () => {
  // xterm measures cells when it opens; load the bundled faces first.
  await Promise.all(
    [
      ['400 14px "Inter"', "Claude Eden 2186"],
      ['600 14px "Inter"', "Claude Eden 2186"],
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
  return import("./main.jsx");
});

installDiagnostics();
// Keep the native menu usable while the workbench is loading or disconnected.
window.edenNativeAction = (command) => {
  if (window.edenWorkbenchAction) return window.edenWorkbenchAction(command);
  if (command === "quit") return native("quitNow");
};
createRoot(document.getElementById("root")).render(
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
