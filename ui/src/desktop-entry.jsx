import React, { lazy, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { installDiagnostics } from "./diagnostics.mjs";
import { DiagnosticBoundary } from "./diagnostics.jsx";
import { LocalWorkbenchRoot } from "./local-workbench";
import { native } from "./api";
import "./style.css";
import { IconHints } from "./icon-hints.jsx";

const Workbench = lazy(() => import("./main.jsx"));

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
