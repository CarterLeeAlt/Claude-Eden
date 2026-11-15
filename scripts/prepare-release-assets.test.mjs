import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { lstat, mkdir, mkdtemp, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { prepareReleaseAssets } from './prepare-release-assets.mjs';
import { checksumFor } from '../www/scripts/sync-release.mjs';

const version = (await readFile(new URL('../VERSION', import.meta.url), 'utf8')).trim();
const dependencies = JSON.parse(await readFile(new URL('../packaging/windows/dependencies.json', import.meta.url), 'utf8'));
const names = {
  windows: `Eden-${version}-windows-amd64-setup.exe`,
  windowsSource: `mihomo-${dependencies.mihomo.version}-source.tar.gz`,
  qemu: `qemu-${dependencies.qemu.version}-windows-full-source.zip`,
};
const digest = data => createHash('sha256').update(data).digest('hex');

async function fixture(t) {
  const root = await mkdtemp(join(tmpdir(), 'eden-release-test-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  return { root, windows: join(root, 'windows'), output: join(root, 'publish') };
}

async function platform(directory, assets) {
  await mkdir(directory, { recursive: true });
  for (const [name, data] of Object.entries(assets)) await writeFile(join(directory, name), data);
  await writeFile(join(directory, 'SHA256SUMS'), Object.entries(assets).map(([name, data]) => `${digest(data)}  ${name}\n`).join(''));
}

const windowsAssets = () => ({ [names.windows]: 'Windows package fixture', [names.windowsSource]: 'Mihomo source fixture', [names.qemu]: 'complete QEMU source fixture' });

test('deduplicates matching Windows inputs and keeps website checksum lookup valid', async t => {
  const f = await fixture(t);
  const extraSource = `${dependencies.qemuSource.file}.gz`;
  const packagedSources = {
    [extraSource]: 'QEMU repository source fixture',
    [`qemu-keycodemapdb-${dependencies.qemuKeycodemap.version}-source.tar.gz`]: 'keymap source fixture',
    [`qemu-${dependencies.qemu.version}-eden-changes-source.tar.gz`]: 'Eden QEMU patch fixture',
  };
  const second = join(f.root, 'second-windows');
  await platform(second, windowsAssets());
  await platform(f.windows, { ...windowsAssets(), ...packagedSources });
  const result = await prepareReleaseAssets({ inputs: [second, f.windows], output: f.output });
  const expected = new Map(Object.entries({ ...windowsAssets(), ...packagedSources }));
  assert.deepEqual(new Set(await readdir(f.output)), new Set([...expected.keys(), 'SHA256SUMS']));
  assert.equal(result.version, version);
  const sums = await readFile(join(f.output, 'SHA256SUMS'), 'utf8');
  for (const [name, data] of expected) {
    assert.equal(await readFile(join(f.output, name), 'utf8'), data);
    assert.equal(checksumFor(sums, { name }), digest(data));
  }
  assert.equal(sums.trimEnd().split('\n').length, expected.size);
});

test('supports a Windows package with its corresponding sources', async t => {
  const f = await fixture(t);
  await platform(f.windows, windowsAssets());
  assert.equal((await prepareReleaseAssets({ inputs: [f.windows], output: f.output })).assets.length, 3);
});

test('rejects corrupted bytes before creating the output', async t => {
  const f = await fixture(t);
  await platform(f.windows, windowsAssets());
  await writeFile(join(f.windows, names.windows), 'changed after packaging');
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows], output: f.output }), /SHA-256 mismatch/);
  await assert.rejects(lstat(f.output), { code: 'ENOENT' });
});

test('rejects conflicting contents under the same asset name', async t => {
  const f = await fixture(t);
  const second = join(f.root, 'second-windows');
  await platform(f.windows, windowsAssets());
  await platform(second, { ...windowsAssets(), [names.windowsSource]: 'different source archive' });
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows, second], output: f.output }), /Conflicting same-name/);
});

test('checks duplicate input bytes even when their manifests claim identical digests', async t => {
  const f = await fixture(t);
  const second = join(f.root, 'second-windows');
  await platform(f.windows, windowsAssets());
  await platform(second, windowsAssets());
  await writeFile(join(second, names.windowsSource), 'corrupted duplicate');
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows, second], output: f.output }), /SHA-256 mismatch/);
});

test('rejects unsigned development installers including unlisted leftovers', async t => {
  const f = await fixture(t);
  const unsigned = names.windows.replace(/\.exe$/, '-unsigned.exe');
  const assets = windowsAssets();
  assets[unsigned] = assets[names.windows];
  delete assets[names.windows];
  await platform(f.windows, assets);
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows], output: f.output }), /Unsigned Windows development/);
  await platform(f.windows, windowsAssets());
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows], output: f.output }), /Unsigned Windows development/);
});

test('requires Windows complete QEMU source in the same verified manifest', async t => {
  const f = await fixture(t);
  const assets = windowsAssets();
  delete assets[names.qemu];
  await platform(f.windows, assets);
  await writeFile(join(f.windows, names.qemu), 'unlisted source is not verified');
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows], output: f.output }), /complete QEMU source/);
});

test('requires Windows Mihomo source in the same verified manifest', async t => {
  const f = await fixture(t);
  const assets = windowsAssets();
  delete assets[names.windowsSource];
  await platform(f.windows, assets);
  await writeFile(join(f.windows, names.windowsSource), 'unlisted source is not verified');
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows], output: f.output }), /Mihomo and complete QEMU source/);
});

test('rejects wrong versions and manifest paths outside the release directory', async t => {
  const f = await fixture(t);
  await platform(f.windows, { ...windowsAssets(), 'Eden-99.0.0-windows-amd64-setup.exe': 'wrong version' });
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows], output: f.output }), /Unsupported asset or version/);
  await writeFile(join(f.windows, 'SHA256SUMS'), `${digest('outside')}  ../escape.exe\n`);
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows], output: f.output }), /Invalid or duplicate/);
});

test('never overwrites an existing output, including an empty one', async t => {
  const f = await fixture(t);
  await platform(f.windows, windowsAssets());
  await mkdir(f.output);
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows], output: f.output }), /Output must be a new directory/);
  await writeFile(join(f.output, 'published-asset'), 'keep');
  await assert.rejects(prepareReleaseAssets({ inputs: [f.windows], output: f.output }), /Output must be a new directory/);
  assert.equal(await readFile(join(f.output, 'published-asset'), 'utf8'), 'keep');
});
