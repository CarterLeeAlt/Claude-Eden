import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { stagePreview, mergeReleaseChecksums } from './publish-platform-preview.mjs';
const version = '0.1.1-alpha';
const dependencies = { mihomoSource: { file: 'mihomo-v1.19.33-source.tar.gz' }, qemuSource: { file: 'qemu-' + 'a'.repeat(40) + '-source.tar' }, qemu: { version: '10.2.0' }, qemuKeycodemap: { version: 'b'.repeat(40) } };
const hash = text => createHash('sha256').update(text).digest('hex');
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'eden-preview-test-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const windows = join(root, 'windows'), output = join(root, 'public');
  for (const [directory, names] of [[windows, [`Eden-${version}-windows-amd64-setup-unsigned.exe`, dependencies.mihomoSource.file, dependencies.qemuSource.file + '.gz', `qemu-keycodemapdb-${dependencies.qemuKeycodemap.version}-source.tar.gz`, `qemu-${dependencies.qemu.version}-eden-changes-source.tar.gz`]]]) {
    mkdirSync(directory);
    for (const name of names) writeFileSync(join(directory, name), name);
    writeFileSync(join(directory, 'SHA256SUMS'), names.map(name => `${hash(name)}  ${name}\n`).join(''));
    writeFileSync(join(directory, 'private-report.json'), 'not a release file');
  }
  return { windows, output };
}
test('preview publication stages verified installers and sources only, with one source copy', t => {
  const f = fixture(t), files = stagePreview(f.windows, f.output, version, dependencies);
  assert.equal(files.size, 6);
  assert.deepEqual(readdirSync(f.output).sort(), [...files.keys()].sort());
  assert.ok(!files.has('private-report.json'));
  for (const [name, file] of files) assert.equal(hash(readFileSync(join(f.output, name))), file.digest);
});
test('corrupt, missing, duplicate and unexpected checksum entries never become release assets', async t => {
  for (const kind of ['corrupt', 'missing', 'duplicate', 'unexpected', 'corrupt-source']) await t.test(kind, t => {
    const f = fixture(t), manifest = join(f.windows, 'SHA256SUMS');
    const name = `Eden-${version}-windows-amd64-setup-unsigned.exe`;
    if (kind === 'corrupt') writeFileSync(join(f.windows, name), 'changed');
    if (kind === 'missing') rmSync(join(f.windows, name));
    if (kind === 'duplicate') writeFileSync(manifest, readFileSync(manifest, 'utf8') + `${hash(name)}  ${name}\n`);
    if (kind === 'unexpected') writeFileSync(manifest, readFileSync(manifest, 'utf8') + `${hash('other')}  other.exe\n`);
    if (kind === 'corrupt-source') {
      writeFileSync(join(f.windows, dependencies.mihomoSource.file), 'changed-source');
    }
    assert.throws(() => stagePreview(f.windows, f.output, version, dependencies));
  });
});

test('consolidation preserves signed Windows checksums and rejects conflicting version assets', () => {
  const hash = 'a'.repeat(64), other = 'b'.repeat(64);
  const signed = 'Eden-0.1.1-alpha-windows-amd64-setup.exe', windows = 'Eden-0.1.1-alpha-windows-amd64-setup-unsigned.exe';
  const release = { tag_name: 'v0.1.1-alpha', draft: false, prerelease: true,
    assets: [{name: signed, state: 'uploaded', digest: `sha256:${hash}`}] };
  const files = new Map([[windows, {digest: other}], ['SHA256SUMS', {digest: other}]]);
  const original = `${hash}  ${signed}\n`;
  const merged = mergeReleaseChecksums(release, original, files, '0.1.1-alpha');
  assert.equal(merged, original + `${other}  ${windows}\n`);
  const partial = {...release, assets:[...release.assets, {name:windows, state:'uploaded', digest:`sha256:${other}`}]};
  assert.equal(mergeReleaseChecksums(partial, original, files, '0.1.1-alpha'), merged);
  assert.equal(mergeReleaseChecksums(partial, merged, files, '0.1.1-alpha'), merged);
  assert.throws(() => mergeReleaseChecksums(release, original + original, files, '0.1.1-alpha'));
  assert.throws(() => mergeReleaseChecksums(release, original, files, '0.1.0-alpha.1'));
  assert.throws(() => mergeReleaseChecksums(release, original, new Map([[signed,{digest:other}]]), '0.1.1-alpha'));
  assert.throws(() => mergeReleaseChecksums({...release,draft:true}, original, files, '0.1.1-alpha'));
  assert.throws(() => mergeReleaseChecksums({...release,prerelease:false}, original, files, '0.1.1-alpha'));
  assert.throws(() => mergeReleaseChecksums(partial, original, new Map(), '0.1.1-alpha'));
});
