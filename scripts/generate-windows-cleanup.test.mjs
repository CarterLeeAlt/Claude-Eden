import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { copyFileSync, existsSync, linkSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, renameSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { test } from 'node:test';
import { generateWindowsCleanup } from './generate-windows-cleanup.mjs';
import { prepareWorkspace } from './run-in-workspace.mjs';

const root = fileURLToPath(new URL('../', import.meta.url));
prepareWorkspace(root, process.env, '.', { check: true });
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const guideName = 'Eden.cleanup.md';
const manifestName = 'Eden.install-files.json';
const helperShells = process.platform === 'win32' ? [
  'pwsh',
  path.join(process.env.SystemRoot, 'System32/WindowsPowerShell/v1.0/powershell.exe'),
  path.join(process.env.SystemRoot, 'SysWOW64/WindowsPowerShell/v1.0/powershell.exe'),
] : [];

function fixture(t) {
  const directory = mkdtempSync(path.join(root, '.tmp', 'windows-cleanup-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const app = path.join(directory, 'app 中文');
  mkdirSync(path.join(app, 'resources', 'licenses'), { recursive: true });
  for (const [name, text] of Object.entries({
    'Eden.exe': 'fixture desktop, never executed',
    'resources/eden.exe': 'fixture backend, never executed',
    'resources/licenses/LICENSE.txt': 'third-party attribution',
    'resources/资料 _ [beta] `file`.txt': 'Unicode and Markdown punctuation',
    'version.txt': '1.2.3-test.1',
    'Eden.install-id': 'io.github.carterleealt.claude-eden',
  })) writeFileSync(path.join(app, name), text);
  return { directory, app };
}

function assertManifest(app) {
  const manifest = JSON.parse(readFileSync(path.join(app, manifestName), 'utf8'));
  assert.equal(manifest.version, 1);
  const actual = readdirSync(app, { recursive: true, withFileTypes: true }).filter(entry => entry.isFile())
    .map(entry => path.relative(app, path.join(entry.parentPath, entry.name)).replaceAll(path.sep, '/')).filter(name => name !== manifestName).sort();
  assert.deepEqual(manifest.files.map(file => file.path), actual);
  for (const file of manifest.files) assert.equal(file.sha256, hash(readFileSync(path.join(app, file.path))), file.path);
  assert.ok(manifest.files.some(file => file.path === guideName));
  return manifest;
}

test('full package covers every actual file, generated items and optional runtime locations without leaking build paths', async t => {
  const { app, directory } = fixture(t);
  const result = await generateWindowsCleanup(app);
  assert.equal(result.edition, 'full');
  const manifest = assertManifest(app);
  const guide = readFileSync(path.join(app, guideName), 'utf8');
  const appendix = guide.split('## 本包实际文件与安装器生成项')[1];
  assert.ok(appendix);
  for (const file of manifest.files) assert.ok(appendix.includes(file.path), file.path);
  assert.ok(appendix.includes(manifestName));
  assert.ok(appendix.includes('Uninstall.exe'));
  assert.ok(!existsSync(path.join(app, 'Uninstall.exe')));
  assert.ok(!manifest.files.some(file => file.path === 'Uninstall.exe'));
  for (const marker of ['clipboard-ownership', 'trusted-local', 'DOTNET_BUNDLE_EXTRACT_BASE_DIR', 'Eden-preserved-files', 'controller-instance.json', 'cli-sessions', 'TMPDIR', 'Server.Dir', 'workbench-zram-device', '.cache-maintenance-recovery', 'host-system-source.json']) assert.ok(guide.includes(marker), marker);
  assert.doesNotMatch(guide, /Android|Ubuntu|mobile|remote-hosts|remote-downloads|cloud-host|container-engine|Eden\.client-id/);
  assert.doesNotMatch(guide, /Windows 客[户]?机|HyperVManager|Virtual Machines Cache|ComponentMaintenance|PacketBridge|GuestBridge|windows-guest|setupadmin|DefaultPassword|scripts\/guest-os/);
  assert.ok(!guide.includes(directory));
  assert.ok(!guide.includes(root));
  assert.ok(!guide.includes('{{'));
  assert.ok(guide.includes('`` resources/资料 _ [beta] `file`.txt ``'));
});

test('local guest and Tailscale inventory retains dynamic locations and ownership boundaries', async t => {
  const { app } = fixture(t);
  await generateWindowsCleanup(app);
  const guide = readFileSync(path.join(app, guideName), 'utf8');
  for (const marker of ['Debian 13', 'developer', 'browser', '18961', 'node.lock', 'CorpDNS=false', 'RouteAll=false', 'mktemp -t agent-vm-proxy.XXXXXX', 'mktemp -t workbench-tailnet.XXXXXX']) assert.ok(guide.includes(marker), marker);
  assert.ok(!guide.includes('/tmp/agent-vm-proxy.sudoers'));
  assert.ok(!guide.includes('/tmp/workbench-tailnet.XXXXXX'));
});

test('only the local Windows payload is accepted', async t => {
  for (const name of ['Eden.client-id', 'resources/client-only', 'resources/cloud-linux-amd64.tar']) {
    const { app } = fixture(t);
    writeFileSync(path.join(app, name), 'unexpected product artifact');
    await assert.rejects(generateWindowsCleanup(app), /non-local product payload/);
    assert.ok(!existsSync(path.join(app, guideName)));
  }
});

test('regeneration is deterministic and picks up added, removed and changed payload files before hashing the guide', async t => {
  const { app } = fixture(t);
  await generateWindowsCleanup(app);
  const before = [guideName, manifestName].map(name => readFileSync(path.join(app, name), 'utf8'));
  await generateWindowsCleanup(app);
  assert.deepEqual([guideName, manifestName].map(name => readFileSync(path.join(app, name), 'utf8')), before);
  writeFileSync(path.join(app, 'resources/new.txt'), 'new payload');
  writeFileSync(path.join(app, 'Eden.exe'), 'updated fixture binary');
  rmSync(path.join(app, 'resources/licenses/LICENSE.txt'));
  await generateWindowsCleanup(app);
  const manifest = assertManifest(app);
  const appendix = readFileSync(path.join(app, guideName), 'utf8').split('## 本包实际文件与安装器生成项')[1];
  assert.ok(appendix.includes('resources/new.txt'));
  assert.ok(!appendix.includes('resources/licenses/LICENSE.txt'));
  assert.notEqual(manifest.files.find(file => file.path === guideName).sha256, hash(Buffer.from(before[0])));
});

test('moving a payload does not change the guide or manifest bytes', async t => {
  const { app, directory } = fixture(t);
  await generateWindowsCleanup(app);
  const before = [guideName, manifestName].map(name => readFileSync(path.join(app, name)));
  const moved = path.join(directory, 'relocated');
  renameSync(app, moved);
  await generateWindowsCleanup(moved);
  assert.deepEqual([guideName, manifestName].map(name => readFileSync(path.join(moved, name))), before);
  assertManifest(moved);
});

test('incomplete, ambiguous, invalid-version and installed/data-bearing input fails before modifying generated outputs', async t => {
  for (const mutate of [
    app => rmSync(path.join(app, 'Eden.exe')),
    app => rmSync(path.join(app, 'Eden.install-id')),
    app => writeFileSync(path.join(app, 'Eden.install-id'), 'another application'),
    app => writeFileSync(path.join(app, 'Eden.client-id'), 'io.github.carterleealt.claude-eden-client'),
    app => writeFileSync(path.join(app, 'version.txt'), 'not a version'),
    app => mkdirSync(path.join(app, 'EdenData')),
    app => mkdirSync(path.join(app, 'Eden-preserved-files')),
    app => writeFileSync(path.join(app, 'Uninstall.exe'), 'installed fixture'),
  ]) {
    const { app } = fixture(t);
    writeFileSync(path.join(app, guideName), 'retain prior guide');
    writeFileSync(path.join(app, manifestName), 'retain prior manifest');
    mutate(app);
    await assert.rejects(generateWindowsCleanup(app));
    assert.equal(readFileSync(path.join(app, guideName), 'utf8'), 'retain prior guide');
    assert.equal(readFileSync(path.join(app, manifestName), 'utf8'), 'retain prior manifest');
  }
});

test('canonical generated files and identities cannot be directories or differently cased names', async t => {
  for (const name of [guideName, manifestName, 'Eden.install-id']) {
    for (const asDirectory of [false, true]) {
      const { app } = fixture(t);
      if (existsSync(path.join(app, name))) rmSync(path.join(app, name));
      if (asDirectory) mkdirSync(path.join(app, name));
      else writeFileSync(path.join(app, name.toLowerCase()), 'noncanonical input');
      await assert.rejects(generateWindowsCleanup(app), /canonical ordinary payload file/);
      if (!asDirectory) assert.equal(readFileSync(path.join(app, name.toLowerCase()), 'utf8'), 'noncanonical input');
    }
  }
});

test('payload junctions and linked output names are rejected without reading or modifying their targets', async t => {
  for (const name of ['resources/linked', guideName, manifestName]) {
    const { app, directory } = fixture(t);
    const outside = path.join(directory, 'outside');
    mkdirSync(outside);
    writeFileSync(path.join(outside, 'sentinel'), 'untouched');
    symlinkSync(outside, path.join(app, name), process.platform === 'win32' ? 'junction' : 'dir');
    await assert.rejects(generateWindowsCleanup(app), /links are not allowed/);
    assert.deepEqual(readdirSync(outside), ['sentinel']);
    assert.equal(readFileSync(path.join(outside, 'sentinel'), 'utf8'), 'untouched');
  }
  const { app, directory } = fixture(t);
  const linked = path.join(directory, 'linked-app');
  symlinkSync(app, linked, process.platform === 'win32' ? 'junction' : 'dir');
  await assert.rejects(generateWindowsCleanup(linked), /ordinary build directory/);
  const escaping = path.join(directory, 'escaping-parent');
  symlinkSync(path.dirname(root.replace(/[\\/]$/, '')), escaping, process.platform === 'win32' ? 'junction' : 'dir');
  await assert.rejects(generateWindowsCleanup(path.join(escaping, 'never-created')), /outside the workspace/);
  await assert.rejects(generateWindowsCleanup(path.parse(root).root), /outside the workspace/);
});

test('shipped CLI generates the documents through the workspace entry', t => {
  const { app } = fixture(t);
  const result = spawnSync(process.execPath, ['scripts/run-in-workspace.mjs', '--', process.execPath, 'scripts/generate-windows-cleanup.mjs', app], { cwd: root, encoding: 'utf8', timeout: 15_000 });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(JSON.parse(result.stdout).guide, path.join(app, guideName));
  assertManifest(app);
  const invalid = spawnSync(process.execPath, ['scripts/run-in-workspace.mjs', '--', process.execPath, 'scripts/generate-windows-cleanup.mjs'], { cwd: root, encoding: 'utf8', timeout: 15_000 });
  assert.equal(invalid.status, 1);
  assert.match(invalid.stderr, /Usage:/);
});

test('hard-linked generated outputs are rejected before their shared bytes can be changed', async t => {
  for (const name of [guideName, manifestName]) {
    const { app, directory } = fixture(t);
    const original = path.join(directory, 'shared-original');
    writeFileSync(original, 'shared bytes must survive');
    linkSync(original, path.join(app, name));
    await assert.rejects(generateWindowsCleanup(app), /must not be hard-linked/);
    assert.equal(readFileSync(original, 'utf8'), 'shared bytes must survive');
  }
});

test('installation helper adds the actual uninstaller hash without changing the guide', { skip: process.platform !== 'win32', timeout: 50_000 }, async t => {
  for (const shell of helperShells) {
    const { app, directory } = fixture(t);
    await generateWindowsCleanup(app);
    const guide = readFileSync(path.join(app, guideName));
    writeFileSync(path.join(app, 'Uninstall.exe'), 'fixture uninstaller, never executed');
    const result = spawnSync(shell, ['-NoLogo', '-NoProfile', '-NonInteractive', '-File', path.join(root, 'packaging/windows/installation-files.ps1'), '-Operation', 'prepare', '-Target', app, '-Receipt', path.join(directory, 'installation.json')], { cwd: root, encoding: 'utf8', timeout: 15_000 });
    assert.equal(result.status, 0, `${shell}: ${result.stderr}`);
    assert.deepEqual(readFileSync(path.join(app, guideName)), guide);
    const manifest = JSON.parse(readFileSync(path.join(app, manifestName), 'utf8').replace(/^\uFEFF/, ''));
    for (const name of [guideName, 'Uninstall.exe']) assert.equal(manifest.files.find(file => file.path === name).sha256, hash(readFileSync(path.join(app, name))));
  }
});

test('existing uninstall helper removes the hash-matching guide and retains a modified guide and user files', { skip: process.platform !== 'win32', timeout: 100_000 }, async t => {
  for (const [shell, modified] of helperShells.flatMap(shell => [[shell, false], [shell, true]])) {
    const { app, directory } = fixture(t);
    await generateWindowsCleanup(app);
    const embedded = path.join(directory, 'embedded-manifest.json');
    copyFileSync(path.join(app, manifestName), embedded);
    if (modified) writeFileSync(path.join(app, guideName), 'user-maintained cleanup notes');
    writeFileSync(path.join(app, 'user-project.txt'), 'keep user data');
    const result = spawnSync(shell, ['-NoLogo', '-NoProfile', '-NonInteractive', '-File', path.join(root, 'packaging/windows/installation-files.ps1'), '-Operation', 'uninstall', '-Target', app, '-Manifest', embedded], { cwd: root, encoding: 'utf8', timeout: 15_000 });
    assert.equal(result.status, 0, `${shell}: ${result.stderr}`);
    assert.equal(existsSync(path.join(app, guideName)), modified);
    assert.equal(readFileSync(path.join(app, 'user-project.txt'), 'utf8'), 'keep user data');
    assert.equal(existsSync(path.join(app, 'Eden.exe')), false);
  }
});
