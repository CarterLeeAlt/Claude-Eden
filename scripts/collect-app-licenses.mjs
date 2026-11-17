#!/usr/bin/env node
import { cpSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { resolve, join } from "node:path";

// Collect the licenses of the packages actually shipped in the workbench.
const target = process.argv[2];
if (!target) throw new Error("Usage: node scripts/collect-app-licenses.mjs <output-directory>");
const root = resolve(import.meta.dirname, "..");
mkdirSync(target, { recursive: true });
for (const file of ["LICENSE", "NOTICE", "THIRD_PARTY_LICENSES"]) {
  cpSync(join(root, file), join(target, file));
}
cpSync(join(root, "internal/exitpolicy/data"), join(target, "exit-country-data"), { recursive: true });
mkdirSync(join(target, "fonts"), { recursive: true });
for (const file of readdirSync(join(root, "assets/fonts")).filter(file => /(?:-LICENSE\.txt$|^NOTICE\.txt$)/.test(file))) {
  cpSync(join(root, "assets/fonts", file), join(target, "fonts", file));
}
mkdirSync(join(target, "guestidentity"), { recursive: true });
for (const file of ["LICENSE", "hostname-sources.json"]) {
  cpSync(join(root, "shared/guestidentity", file), join(target, "guestidentity", file));
}
const lock = JSON.parse(readFileSync(join(root, "ui/package-lock.json"), "utf8"));
const wailsSource = JSON.parse(readFileSync(join(root, 'patches/wails/source.json'), 'utf8'));
// These published packages omit the license text; copy the pinned upstream notices.
const npmLicenseSources = new Map([
  ['@wailsio/runtime', {version: wailsSource.version.slice(1),
    files: ['third_party/wails/LICENSE'], source: `${wailsSource.repository}/tree/${wailsSource.commit}`}],
  ['react-remove-scroll-bar', {version: '2.3.8',
    files: ['assets/licenses/react-remove-scroll-bar/LICENSE', 'assets/licenses/react-remove-scroll-bar/NOTICE'],
    source: 'https://github.com/theKashey/react-remove-scroll-bar/blob/7301c160fda44cb8cf2b9fdfde61efad35736196/LICENSE'}],
]);
const notices = [];
for (const [path, entry] of Object.entries(lock.packages)) {
  if (!path || entry.dev) continue;
  const source = join(root, "ui", path);
  const metadata = JSON.parse(readFileSync(join(source, "package.json"), "utf8"));
  const name = metadata.name;
  const dest = join(target, "npm", name);
  const sourceLicense = npmLicenseSources.get(name);
  if (sourceLicense) {
    if (entry.version !== sourceLicense.version || metadata.license !== 'MIT') {
      throw new Error(`${name} version/license differs from the reviewed upstream notice.`);
    }
    mkdirSync(dest, {recursive: true});
    for (const file of sourceLicense.files) cpSync(join(root, file), join(dest, file.split('/').at(-1)));
    notices.push(`${name}@${entry.version}: MIT; ${sourceLicense.source}`);
    continue;
  }
  const files = readdirSync(source).filter((file) => /^(licen[sc]e|copying|notice)(\.|$)/i.test(file) && statSync(join(source, file)).isFile());
  if (!files.length) throw new Error(`Missing license for ${name}@${entry.version}`);
  mkdirSync(dest, { recursive: true });
  for (const file of files) cpSync(join(source, file), join(dest, file));
  notices.push(`${name}@${entry.version}: ${entry.license || "see license files"}`);
}
writeFileSync(join(target, "npm", "NOTICE.txt"), `${notices.join("\n")}\n`);
const novnc = join(root, "ui/vendor/novnc");
for (const file of ["LICENSE.txt", "AUTHORS", "NOTICE.eden", "docs", "vendor/pako/LICENSE"]) {
  mkdirSync(join(target, "novnc", file.includes("/") ? file.slice(0, file.lastIndexOf("/")) : ""), { recursive: true });
  cpSync(join(novnc, file), join(target, "novnc", file), { recursive: true });
}
// MPL-covered source and local modifications travel with the executable.
cpSync(join(root, "ui/.generated/novnc"), join(target, "novnc-source"), { recursive: true });

// Resolve the modules of shipped Windows desktop code, excluding build-only tools.
const desktopModules = execFileSync("go", ["list", "-deps", "-f", "{{if .Module}}{{.Module.Dir}}{{end}}",
  "./cmd/eden-desktop", "./cmd/eden-install"], {
  cwd: root, env: { ...process.env, GOOS: "windows", GOARCH: "amd64", CGO_ENABLED: "0", GOFLAGS: "" }, encoding: "utf8",
});
for (const dir of new Set(desktopModules.split(/\r?\n/).filter(Boolean))) {
  if (resolve(dir) === root) continue;
  const normalized = dir.replaceAll("\\", "/");
  const name = normalized.includes("/pkg/mod/")
    ? normalized.slice(normalized.lastIndexOf("/pkg/mod/") + 9)
    : pathRelativeModule(dir);
  const destination = join(target, "desktop-go", name);
  const files = readdirSync(dir).filter(file => /^(licen[sc]e|copying|notice|patents|authors)(\.|$)/i.test(file) && statSync(join(dir, file)).isFile());
  if (!files.length) throw new Error(`Missing desktop dependency license: ${name}`);
  mkdirSync(destination, { recursive: true });
  for (const file of files) cpSync(join(dir, file), join(destination, file));
  if (name === "github.com/wailsapp/wails/v3" || name.startsWith("github.com/wailsapp/wails/v3@")) {
    for (const component of ["internal/webview2/webviewloader", "internal/go-common-file-dialog"]) {
      mkdirSync(join(destination, component), { recursive: true });
      cpSync(join(dir, component, "LICENSE"), join(destination, component, "LICENSE"));
    }
    cpSync(join(root, 'patches/wails/source.json'), join(destination, 'SOURCE.json'));
  }
}

function pathRelativeModule(directory) {
  const module = readFileSync(join(directory, 'go.mod'), 'utf8').match(/^module\s+(\S+)/m)?.[1];
  if (!module) throw new Error(`Missing module identity: ${directory}`);
  return module;
}

// The embedded Tailscale node is a separate guest executable and module.
const tailnetModules = new Set();
for (const [os, arch] of [["linux", "amd64"], ["linux", "arm64"]]) {
  const dirs = execFileSync("go", ["list", "-deps", "-f", "{{if .Module}}{{.Module.Dir}}{{end}}", "."], {
    cwd: join(root, "tools/tailnet"), env: { ...process.env, GOOS: os, GOARCH: arch, CGO_ENABLED: "0", GOFLAGS: "" }, encoding: "utf8",
  });
  for (const dir of dirs.split("\n").filter(Boolean)) if (resolve(dir) !== join(root, "tools/tailnet")) tailnetModules.add(dir);
}
const tailnetNotices = [];
for (const dir of tailnetModules) {
  const normalized = dir.replaceAll("\\", "/");
  const name = normalized.slice(normalized.lastIndexOf("/pkg/mod/") + 9);
  const dest = join(target, "tailnet-go", name);
  const licenses = readdirSync(dir).filter(file => /^(licen[sc]e|copying|notice|patents|authors)(\.|$)/i.test(file) && statSync(join(dir, file)).isFile());
  if (!licenses.length) throw Error(`Missing guest dependency license: ${name}`);
  mkdirSync(dest, { recursive: true });
  for (const file of licenses) cpSync(join(dir, file), join(dest, file));
  tailnetNotices.push(name);
}
writeFileSync(join(target, "tailnet-go", "NOTICE.txt"), tailnetNotices.sort().join("\n") + "\n");
