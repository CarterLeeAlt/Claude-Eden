// Explicit preview publication only. Signed release/update policies are unchanged.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { appendFileSync, copyFileSync, lstatSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const repo = 'CarterLeeAlt/Claude-Eden';
const gh = args => execFileSync('gh', args, { encoding: 'utf8', maxBuffer: 8 * 1024 * 1024 });
const api = (path, args = []) => JSON.parse(gh(['api', `repos/${repo}/${path}`, ...args]));
const digest = data => createHash('sha256').update(data).digest('hex');

export function stagePreview(windows, destination, version, dependencies) {
  assert.match(version, /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/);
  const expected = [
    [windows, [`Eden-${version}-windows-amd64-setup-unsigned.exe`, dependencies.mihomoSource.file, `${dependencies.qemuSource.file}.gz`,
      `qemu-keycodemapdb-${dependencies.qemuKeycodemap.version}-source.tar.gz`, `qemu-${dependencies.qemu.version}-eden-changes-source.tar.gz`]],
  ];
  const verified = new Map();
  for (const [directory, names] of expected) {
    const manifest = readFileSync(join(directory, 'SHA256SUMS'), 'utf8').replace(/^\uFEFF/, '');
    assert.ok(Buffer.byteLength(manifest) < 65536);
    const entries = new Map();
    for (const line of manifest.split(/\r?\n/).filter(Boolean)) {
      const match = /^([a-f0-9]{64}) [ *]([A-Za-z0-9][A-Za-z0-9._-]*)$/i.exec(line);
      assert.ok(match && !entries.has(match[2]), 'Invalid or duplicate platform checksum');
      entries.set(match[2], match[1].toLowerCase());
    }
    assert.deepEqual([...entries.keys()].sort(), [...names].sort(), 'Unexpected platform assets');
    for (const name of names) {
      const path = join(directory, name), stat = lstatSync(path);
      assert.ok(stat.isFile() && stat.size > 0 && stat.size <= 512 * 1024 * 1024);
      const actual = digest(readFileSync(path));
      assert.equal(actual, entries.get(name), `Corrupt artifact: ${name}`);
      if (verified.has(name)) assert.equal(verified.get(name).digest, actual, 'Conflicting source archives');
      else verified.set(name, { path, digest: actual, size: stat.size });
    }
  }
  // Do not copy reports or other artifact leftovers into a public release.
  mkdirSync(destination);
  for (const [name, file] of verified) copyFileSync(file.path, join(destination, name));
  const sums = [...verified].map(([name, file]) => `${file.digest}  ${name}\n`).join('');
  writeFileSync(join(destination, 'SHA256SUMS'), sums);
  verified.set('SHA256SUMS', { digest: digest(sums), size: Buffer.byteLength(sums) });
  return verified;
}

export function mergeReleaseChecksums(release, text, files, version) {
  assert.equal(release.tag_name, `v${version}`);
  assert.equal(release.draft, false);
  assert.equal(release.prerelease, true, 'Unsigned packages require a prerelease');
  const entries = new Map();
  for (const line of text.trim().split(/\r?\n/)) {
    const match = /^([a-f0-9]{64}) [ *]([A-Za-z0-9][A-Za-z0-9._-]*)$/i.exec(line);
    assert.ok(match && !entries.has(match[2]), 'Invalid or duplicate existing checksum');
    entries.set(match[2], match[1].toLowerCase());
  }
  const assets = release.assets.filter(asset => asset.name !== 'SHA256SUMS');
  for (const name of entries.keys()) assert.ok(assets.some(asset => asset.name === name), 'Existing release is incomplete');
  assert.ok(entries.has(`Eden-${version}-windows-amd64-setup.exe`) || entries.has(`Eden-${version}-windows-amd64-setup-unsigned.exe`), 'Existing release must contain its Windows installer');
  for (const asset of assets) {
    assert.equal(asset.state, 'uploaded');
    // A retry may find uploaded platform files whose merged manifest is still pending.
    const expected = entries.get(asset.name) || files.get(asset.name)?.digest;
    assert.ok(expected, `Unlisted existing asset: ${asset.name}`);
    assert.equal(asset.digest, `sha256:${expected}`, `Existing checksum differs: ${asset.name}`);
  }
  for (const [name, file] of files) {
    if (name === 'SHA256SUMS') continue;
    if (entries.has(name)) assert.equal(entries.get(name), file.digest, `Conflicting release asset: ${name}`);
    entries.set(name, file.digest);
  }
  return [...entries].map(([name, hash]) => `${hash}  ${name}\n`).join('');
}

async function main() {
  assert.equal(process.env.GITHUB_EVENT_NAME, 'workflow_dispatch');
  assert.equal(process.env.GITHUB_REPOSITORY, repo);
  assert.equal(process.env.GITHUB_REF, 'refs/heads/main');
  const runID = process.env.EDEN_PREVIEW_RUN;
  assert.match(runID || '', /^\d+$/);
  const run = api(`actions/runs/${runID}`);
  assert.equal(run.conclusion, 'success');
  assert.equal(run.path, '.github/workflows/ci.yml');
  assert.equal(run.head_repository.full_name, repo);
  assert.match(run.head_sha, /^[a-f0-9]{40}$/);
  const sourceFile = path => Buffer.from(api(`contents/${path}?ref=${run.head_sha}`).content, 'base64').toString('utf8');
  const version = sourceFile('VERSION').trim();
  assert.match(version, /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/);
  const dependencies = JSON.parse(sourceFile('packaging/windows/dependencies.json'));
  const root = '_output/platform-publication';
  const destination = join(root, 'public');
  const files = stagePreview(join(root, 'windows'), destination, version, dependencies);
  const tag = `v${version}`;
  let release = api(`releases/tags/${tag}`);
  const originalChecksums = release.assets.find(asset => asset.name === 'SHA256SUMS');
  assert.ok(originalChecksums && originalChecksums.size <= 65536);
  const response = await fetch(originalChecksums.browser_download_url);
  assert.ok(response.ok, 'Cannot read public checksum manifest');
  const originalText = await response.text();
  assert.equal(digest(originalText), originalChecksums.digest.replace('sha256:', ''));
  const merged = mergeReleaseChecksums(release, originalText, files, version);
  writeFileSync(join(destination, 'SHA256SUMS'), merged);
  files.set('SHA256SUMS', { digest: digest(merged), size: Buffer.byteLength(merged) });
  for (const [name, file] of files) {
    if (name === 'SHA256SUMS') continue;
    const existing = release.assets.find(asset => asset.name === name);
    if (existing) {
      assert.equal(existing.digest, `sha256:${file.digest}`, `Existing asset differs: ${name}`);
      assert.equal(existing.size, file.size);
    } else gh(['release', 'upload', tag, join(destination, name), '--repo', repo]);
  }
  // Upload the merged manifest last. Existing installation archives are never replaced.
  const uploaded = api(`releases/${release.id}`);
  for (const [name, file] of files) {
    if (name === 'SHA256SUMS') continue;
    const asset = uploaded.assets.find(asset => asset.name === name);
    assert.ok(asset && asset.state === 'uploaded');
    assert.equal(asset.digest, `sha256:${file.digest}`);
    assert.equal(asset.size, file.size);
  }
  assert.equal(uploaded.assets.find(asset => asset.name === 'SHA256SUMS').digest, originalChecksums.digest, 'Checksum manifest changed concurrently');
  if (originalChecksums.digest !== `sha256:${digest(merged)}`) {
    gh(['release', 'upload', tag, join(destination, 'SHA256SUMS'), '--repo', repo, '--clobber']);
  }
  release = api(`releases/${release.id}`);
  assert.equal(release.assets.find(asset => asset.name === 'SHA256SUMS').digest, `sha256:${digest(merged)}`);
  mergeReleaseChecksums(release, merged, files, version);
  const marker = `<!-- eden-platform-packages source=${run.head_sha} ci=${runID} -->`;
  if (!release.body.includes(marker)) {
    const notes = release.body + `\n\n## Windows 预览版\n\n${marker}\n\nWindows 安装包来自源码 [${run.head_sha.slice(0, 7)}](https://github.com/${repo}/commit/${run.head_sha})，文件摘要列于 SHA256SUMS。\n\n- **Windows：未签名 x64 预览版**，文件名含 -setup-unsigned.exe，需手动安装和更新，系统可能提示未知发布者；不进入签名自动更新通道。\n- 应用随附 Linux 云端部署包，通过 Windows 客户端连接 Linux 服务器。\n- [构建与检查记录](https://github.com/${repo}/actions/runs/${runID})；托管 CI 不代替 Windows 10 真机及硬件虚拟化验收。\n- 随附 Mihomo、QEMU 主程序、键码映射和项目补丁源码；升级前保存文件、停止环境并退出 Claude Eden。\n\n[项目发布页](https://github.com/${repo}/releases/tag/${tag})\n`;
    const notesPath = join(root, 'notes.md'); writeFileSync(notesPath, notes);
    gh(['release', 'edit', tag, '--repo', repo, '--notes-file', notesPath]);
  }
  appendFileSync(process.env.GITHUB_OUTPUT, `tag=${tag}\nurl=${release.html_url}\n`);
  console.log(`Verified platform packages in version release: ${release.html_url}`);
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) main().catch(error => { console.error(error.message); process.exitCode = 1; });
