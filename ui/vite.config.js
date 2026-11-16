import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";
export default defineConfig({
  plugins: [react()],
  base: "/",
  build: {
    outDir: "../internal/workbench/web",
    emptyOutDir: true,
  },
  server: {
    fs: {
      allow: [
        fileURLToPath(new URL(".", import.meta.url)),
        fileURLToPath(new URL("../assets/fonts", import.meta.url)),
      ],
    },
    proxy: { "/api": { target: "http://localhost:17777", ws: true } },
  },
});
