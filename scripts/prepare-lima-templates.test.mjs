import assert from 'node:assert/strict';
import { mkdtemp, mkdir, readFile, readdir, rm, symlink, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { limaTemplateSources, prepareLimaTemplates } from './prepare-lima-templates.mjs';
import { prepareWorkspace } from './run-in-workspace.mjs';

const root = fileURLToPath(new URL('../', import.meta.url));
prepareWorkspace(root, process.env, '.', { check: true });
async function fixture(t) {
  const directory = await mkdtemp(path.join(root, '.tmp/lima-templates-'));
  t.after(() => rm(directory, { recursive: true, force: true }));
  return directory;
}

test('Debian 13 template closure exactly matches reviewed sources and can be refreshed', async t => {
  const directory = await fixture(t);
  const expected = ['_default/mounts.yaml', '_images/debian-13.yaml', 'debian-13.yaml'];
  for (let run = 0; run < 2; run++) {
    assert.deepEqual(await prepareLimaTemplates(directory), expected);
    const files = (await readdir(directory, { recursive: true, withFileTypes: true })).filter(entry => entry.isFile()).map(entry => path.relative(directory, path.join(entry.parentPath, entry.name)).replaceAll(path.sep, '/')).sort();
    assert.deepEqual(files, expected);
    for (const file of expected) assert.deepEqual(await readFile(path.join(directory, file)), await readFile(path.join(root, limaTemplateSources[file])));
  }
});

test('unexpected files, directory links and escaping destinations fail without altering source data', async t => {
  const directory = await fixture(t);
  const unexpected = path.join(directory, 'unexpected');
  await mkdir(unexpected);
  await writeFile(path.join(unexpected, 'another-system.yaml'), 'retain evidence');
  await assert.rejects(prepareLimaTemplates(unexpected), /Unexpected generated template/);
  assert.equal(await readFile(path.join(unexpected, 'another-system.yaml'), 'utf8'), 'retain evidence');
  const target = path.join(directory, 'target');
  await mkdir(target);
  const link = path.join(directory, 'linked');
  await symlink(target, link, process.platform === 'win32' ? 'junction' : 'dir');
  await assert.rejects(prepareLimaTemplates(link), /directory link/);
  await assert.rejects(prepareLimaTemplates(path.parse(root).root), /outside the workspace/);
  await assert.rejects(prepareLimaTemplates(root), /dedicated generated template directory/);
});

test('the CLI runs through the workspace entry and rejects missing arguments', async t => {
  const directory = await fixture(t);
  const invoke = args => spawnSync(process.execPath, ['scripts/run-in-workspace.mjs', '--', process.execPath, 'scripts/prepare-lima-templates.mjs', ...args], { cwd: root, encoding: 'utf8', timeout: 15000 });
  const valid = invoke([directory]);
  assert.equal(valid.status, 0, valid.stderr);
  assert.equal(JSON.parse(valid.stdout).templates.length, 3);
  assert.equal(invoke([]).status, 1);
});
