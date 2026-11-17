#!/usr/bin/env node
// Generate the Windows desktop contract on any build host using the pinned Wails tool.
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, realpathSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { assertWorkspacePath, prepareWorkspace } from './run-in-workspace.mjs';

const root = realpathSync(fileURLToPath(new URL('../', import.meta.url)));
prepareWorkspace(root, process.env, '.', { check: true });
const args = process.argv.slice(2);
if (args.length > 1 || args.length === 1 && args[0] !== '--resources') {
  throw new Error('Usage: node scripts/prepare-desktop.mjs [--resources]');
}
const tools = path.join(root, '.tmp/tools/wails');
assertWorkspacePath(root, tools);
mkdirSync(tools, { recursive: true });
const tool = path.join(tools, process.platform === 'win32' ? 'wails3.exe' : 'wails3');
const native = { ...process.env, CGO_ENABLED: '0' };
delete native.GOOS;
delete native.GOARCH;
const run = (command, arguments_, env = native) => execFileSync(command, arguments_, {
  cwd: root, env, stdio: 'inherit', windowsHide: true,
});
run('go', ['build', '-p', '2', '-trimpath', '-o', tool, 'github.com/wailsapp/wails/v3/cmd/wails3']);

if (args[0] === '--resources') {
  const version = readFileSync(path.join(root, 'VERSION'), 'utf8').trim();
  const match = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$/.exec(version);
  if (!match || match.slice(1).some(value => Number(value) > 65535)) {
    throw new Error('VERSION cannot be represented in Windows file metadata.');
  }
  const numeric = `${match[1]}.${match[2]}.${match[3]}.0`;
  for (const [entry, filename, description] of [
    ['eden-desktop', 'Eden.exe', 'Claude Eden desktop workbench'],
    ['eden-install', 'eden-install.exe', 'Claude Eden installation helper'],
  ]) {
    const info = path.join(tools, `${entry}.json`);
    writeFileSync(info, JSON.stringify({
      fixed: { file_version: numeric, product_version: numeric },
      info: { '0409': {
        FileVersion: version, ProductVersion: version,
        CompanyName: 'Claude Eden contributors', ProductName: 'Claude Eden',
        FileDescription: description, OriginalFilename: filename,
      } },
    }, null, 2) + '\n');
    const output = path.join(root, 'cmd', entry, 'rsrc_windows_amd64.syso');
    assertWorkspacePath(root, output);
    run(tool, ['generate', 'syso', '-arch', 'amd64',
      '-manifest', 'packaging/windows/app.manifest', '-icon', 'packaging/windows/Eden.ico',
      '-info', info, '-out', output]);
  }
} else {
  run(tool, ['generate', 'bindings', '-ts', '-i', '-d', 'ui/.generated/bindings', './cmd/eden-desktop'],
    { ...native, GOOS: 'windows', GOARCH: 'amd64' });
}
