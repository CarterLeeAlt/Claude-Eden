#!/usr/bin/env node
// Generate the relocatable cleanup guide first, then hash it with the payload.
import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { lstat, readdir, readFile, writeFile, realpath } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { assertWorkspacePath, prepareWorkspace } from './run-in-workspace.mjs';

const repository = fileURLToPath(new URL('../', import.meta.url));
const templatePath = fileURLToPath(new URL('../packaging/windows/cleanup.md', import.meta.url));
const guideName = 'Eden.cleanup.md';
const manifestName = 'Eden.install-files.json';
const identities = { 'Eden.install-id': 'io.github.carterleealt.claude-eden' };
const canonicalNames = [guideName, manifestName, ...Object.keys(identities)];
const compare = (a, b) => a < b ? -1 : a > b ? 1 : 0;

async function payloadFiles(directory) {
  const files = [];
  const names = new Set();
  async function visit(relative = '') {
    for (const entry of await readdir(path.join(directory, relative), { withFileTypes: true })) {
      const name = relative ? `${relative}/${entry.name}` : entry.name;
      if (/[<>:"\\|?*\x00-\x1f]/.test(entry.name) || /[. ]$/.test(entry.name) || /^(?:con|prn|aux|nul|com[1-9¹²³]|lpt[1-9¹²³])(?:\.|$)/i.test(entry.name)) throw new Error(`Invalid Windows payload name: ${name}`);
      const key = name.toLowerCase();
      if (names.has(key)) throw new Error(`Case-insensitive payload collision: ${name}`);
      names.add(key);
      const info = await lstat(path.join(directory, name));
      if (info.isSymbolicLink()) throw new Error(`Payload links are not allowed: ${name}`);
      const canonical = !relative && canonicalNames.find(value => value.toLowerCase() === entry.name.toLowerCase());
      if (canonical && (entry.name !== canonical || !info.isFile())) throw new Error(`Expected canonical ordinary payload file: ${canonical}`);
      if (!relative && [guideName, manifestName].includes(entry.name) && info.nlink !== 1) throw new Error(`Generated output must not be hard-linked: ${name}`);
      if (info.isDirectory()) await visit(name);
      else if (info.isFile()) files.push(name);
      else throw new Error(`Unsupported payload entry: ${name}`);
    }
  }
  await visit();
  return files.sort(compare);
}

export async function generateWindowsCleanup(appDirectory) {
  const root = await realpath(repository);
  const directory = path.resolve(appDirectory);
  assertWorkspacePath(root, directory);
  const entry = await lstat(directory);
  if (entry.isSymbolicLink() || !entry.isDirectory()) throw new Error('The app directory must be an ordinary build directory.');
  if ((await readdir(directory)).some(name => /^(?:EdenData|Eden-preserved-files|Uninstall\.exe)$/i.test(name))) throw new Error('Generate from a clean build, not an installed application or user-data directory.');
  const files = await payloadFiles(directory);
  if (files.some(name => name === 'Eden.client-id' || name === 'resources/client-only' || name.startsWith('resources/cloud-linux-'))) throw new Error('Unexpected non-local product payload.');
  const markers = Object.keys(identities).filter(name => files.includes(name));
  if (markers.length !== 1 || (await readFile(path.join(directory, markers[0]), 'utf8')).trim() !== identities[markers[0]]) throw new Error('Expected exactly one valid Claude Eden build identity.');
  for (const name of ['Eden.exe', 'resources/eden.exe', 'version.txt']) if (!files.includes(name)) throw new Error(`Incomplete application payload: ${name}`);
  const version = (await readFile(path.join(directory, 'version.txt'), 'utf8')).trim();
  if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version)) throw new Error('Invalid application version.');
  const owned = [...files.filter(name => ![guideName, manifestName].includes(name)), guideName].sort(compare);
  const listed = [...owned, manifestName, 'Uninstall.exe'].sort(compare);
  const lines = listed.map(name => `- ${code(name)}${name === manifestName ? ' — 生成的哈希索引（不哈希自身）' : name === guideName ? ' — 本资源清单，哈希纳入 JSON' : name === 'Uninstall.exe' ? ' — 预期由完整版安装器生成，非构建目录中的文件' : ''}`);
  let guide = await readFile(templatePath, 'utf8');
  for (const [key, value] of Object.entries({ VERSION: version, PACKAGE_KIND: 'Windows 本地版（NSIS 安装包）' })) {
    const placeholder = `{{${key}}}`;
    if (guide.split(placeholder).length !== 2) throw new Error(`Expected one template placeholder: ${placeholder}`);
    guide = guide.replace(placeholder, value);
  }
  guide += `\n## 本包实际文件与安装器生成项\n\n所有路径相对于本文件所在的程序目录，移动安装位置后仍有效。此表由构建目录的全部普通文件生成，不扫描用户电脑。Uninstall.exe 在安装阶段生成，其实际哈希由现有安装助手加入安装后的 manifest。\n\n${lines.join('\n')}\n`;
  await writeFile(path.join(directory, guideName), guide, 'utf8');
  const records = [];
  for (const name of owned) {
    const hash = createHash('sha256');
    for await (const chunk of createReadStream(path.join(directory, name))) hash.update(chunk);
    records.push({ path: name, sha256: hash.digest('hex') });
  }
  await writeFile(path.join(directory, manifestName), JSON.stringify({ version: 1, files: records }, null, 2) + '\n', 'utf8');
  return { edition: 'full', version, files: records.length, guide: path.join(directory, guideName), manifest: path.join(directory, manifestName) };
}

function code(text) {
  const fence = '`'.repeat(Math.max(0, ...[...text.matchAll(/`+/g)].map(match => match[0].length)) + 1);
  return `${fence} ${text} ${fence}`;
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  (async () => {
    if (process.argv.length !== 3) throw new Error('Usage: node scripts/generate-windows-cleanup.mjs <clean-app-directory>');
    prepareWorkspace(repository, process.env, '.', { check: true });
    console.log(JSON.stringify(await generateWindowsCleanup(process.argv[2])));
  })().catch(error => { console.error(`Cleanup inventory: ${error.message}`); process.exitCode = 1; });
}
