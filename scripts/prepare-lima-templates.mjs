import { copyFile, lstat, mkdir, readdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { assertWorkspacePath, prepareWorkspace } from './run-in-workspace.mjs';

const root = path.resolve(fileURLToPath(new URL('../', import.meta.url)));
export const limaTemplateSources = {
  'debian-13.yaml': 'third_party/lima/templates/debian-13.yaml',
  '_images/debian-13.yaml': 'internal/hostimage/images/debian-13.yaml',
  '_default/mounts.yaml': 'third_party/lima/templates/_default/mounts.yaml',
};

export async function prepareLimaTemplates(destination) {
  destination = path.resolve(destination);
  assertWorkspacePath(root, destination);
  if (destination === root) throw Error('Choose a dedicated generated template directory.');
  try {
    if ((await lstat(destination)).isSymbolicLink()) throw Error('Generated templates cannot use a directory link.');
    for (const entry of await readdir(destination, { recursive: true, withFileTypes: true })) {
      const file = path.join(entry.parentPath, entry.name);
      assertWorkspacePath(root, file);
      if (entry.isSymbolicLink()) throw Error(`Generated template link: ${file}`);
      const relative = path.relative(destination, file).replaceAll(path.sep, '/');
      if (entry.isDirectory() ? !['_images', '_default'].includes(relative) : !Object.hasOwn(limaTemplateSources, relative)) {
        throw Error(`Unexpected generated template entry: ${relative}`);
      }
    }
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
  }
  for (const [relative, source] of Object.entries(limaTemplateSources)) {
    const input = path.join(root, source), output = path.join(destination, relative);
    assertWorkspacePath(root, input);
    assertWorkspacePath(root, output);
    if (!(await lstat(input)).isFile()) throw Error(`Expected an ordinary template source: ${source}`);
    await mkdir(path.dirname(output), { recursive: true });
    await copyFile(input, output);
  }
  return Object.keys(limaTemplateSources).sort();
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try {
    if (process.argv.length !== 3) throw Error('Usage: node scripts/prepare-lima-templates.mjs <generated-template-directory>');
    prepareWorkspace(root, process.env, '.', { check: true });
    console.log(JSON.stringify({ templates: await prepareLimaTemplates(process.argv[2]) }));
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
