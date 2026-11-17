import "./prepare-ui.mjs";
import { build } from "vite";
import ts from "typescript";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { readFile } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import {
  assertWorkspacePath,
  prepareWorkspace,
} from "../../scripts/run-in-workspace.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
prepareWorkspace(root, process.env, ".", { check: true });
const entry = fileURLToPath(new URL("native-probe.ts", import.meta.url));
const output = resolve(root, "ui/.generated/native-probe.js");
assertWorkspacePath(root, output);
const program = ts.createProgram([entry], {
  target: ts.ScriptTarget.ES2023,
  module: ts.ModuleKind.ESNext,
  moduleResolution: ts.ModuleResolutionKind.Bundler,
  strict: true,
  noUnusedLocals: true,
  noEmit: true,
  skipLibCheck: true,
  types: [],
  lib: ["lib.es2023.d.ts", "lib.dom.d.ts", "lib.dom.iterable.d.ts"],
});
const diagnostics = ts.getPreEmitDiagnostics(program);
if (diagnostics.length)
  throw new Error(
    ts.formatDiagnosticsWithColorAndContext(diagnostics, {
      getCanonicalFileName: (name) => name,
      getCurrentDirectory: () => root,
      getNewLine: () => "\n",
    }),
  );
const bundle = await build({
  configFile: false,
  root: resolve(root, "ui"),
  publicDir: false,
  build: {
    target: "chrome109",
    outDir: resolve(root, "ui/.generated"),
    emptyOutDir: false,
    copyPublicDir: false,
    minify: false,
    lib: { entry, formats: ["es"], fileName: () => "native-probe.js" },
  },
});
const outputs = (Array.isArray(bundle) ? bundle : [bundle]).flatMap(
  (value) => value.output ?? [],
);
const chunks = outputs.filter((value) => value.type === "chunk");
if (
  chunks.length !== 1 ||
  chunks[0].imports.length ||
  chunks[0].dynamicImports.length
)
  throw new Error("Native probe must be one self-contained JS module");
const modules = Object.keys(chunks[0].modules);
for (const pattern of [
  /node_modules\/@wailsio\/runtime\/dist\/calls\.js$/,
  /\.generated\/bindings\/.+\/desktop\/desktopservice\.ts$/,
]) {
  if (!modules.some((name) => pattern.test(name.replaceAll("\\", "/"))))
    throw new Error(
      `Real runtime/binding absent from probe bundle: ${pattern}`,
    );
}
const bytes = await readFile(output);
if (!bytes.length) throw new Error("Native probe build is empty");
const parsed = spawnSync(process.execPath, ["--check", output], {
  encoding: "utf8",
  windowsHide: true,
});
if (parsed.status !== 0) throw parsed.error ?? new Error(parsed.stderr);
console.log(
  JSON.stringify({
    module: output,
    bytes: bytes.length,
    runtime: "@wailsio/runtime",
    binding: "DesktopService",
    execution: "VM only; WebView2 results pending",
  }),
);
