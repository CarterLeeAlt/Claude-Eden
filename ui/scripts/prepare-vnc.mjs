import { cpSync, mkdirSync } from "node:fs";
import { execFileSync } from "node:child_process";
mkdirSync(".generated", { recursive: true });
cpSync("vendor/novnc", ".generated/novnc", { recursive: true, force: true });
execFileSync(
  process.platform === "win32" ? "python" : "python3",
  ["-B", "../internal/eden/novnc-patch.py", ".generated/novnc"],
  { stdio: "inherit" },
);
