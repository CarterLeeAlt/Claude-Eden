import React from "react";
import { createRoot } from "react-dom/client";
import { AppearanceProvider } from "../src/features/appearance.tsx";
import { TerminalView } from "../src/features/views.tsx";
import "../src/styles/style.css";

const root = createRoot(document.getElementById("terminal"));
window.showTestTerminal = (visible = true) =>
  root.render(
    <AppearanceProvider>
      <TerminalView
        session={{ id: "image-test" }}
        visible={visible}
        onStatus={(text) => {
          document.getElementById("status").textContent = text;
        }}
      />
    </AppearanceProvider>,
  );
window.showTestTerminal();
