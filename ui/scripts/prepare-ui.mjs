// Generate all contracts and vendored frontend resources before compilation.
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { prepareWorkspace } from "../../scripts/run-in-workspace.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
const ui = fileURLToPath(new URL("../", import.meta.url));
prepareWorkspace(root, process.env, ".", { check: true });
const env = { ...process.env, CGO_ENABLED: "0" };
delete env.GOOS;
delete env.GOARCH;
const run = (command, args, cwd = root) =>
  execFileSync(command, args, {
    cwd,
    env,
    stdio: "inherit",
    windowsHide: true,
  });

run(process.execPath, ["scripts/prepare-desktop.mjs"]);
run("go", ["run", "./ui/scripts/generate-contracts.go"]);
run(process.execPath, ["scripts/prepare-vnc.mjs"], ui);
run(process.execPath, ["scripts/prepare-xterm.mjs"], ui);
