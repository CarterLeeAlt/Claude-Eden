import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdtemp, readFile, rm, writeFile, readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { parseArgs, selectAsset, checksumFor, makePlan, downloadPlan, httpsResponse } from '../www/public/agent-bootstrap.mjs';

const tag = 'v0.1.0-test';
const prefix = `https://github.com/CarterLeeAlt/Claude-Eden/releases/download/${tag}/`;
const data = Buffer.from('fake package fixture');
const hash = createHash('sha256').update(data).digest('hex');
function release(unsigned = false) {
  const name = `Eden-${tag.slice(1)}-windows-amd64-setup${unsigned ? '-unsigned' : ''}.exe`;
  return { tag_name: tag, assets: [{ name, size: data.length, browser_download_url: prefix + name }, { name: 'SHA256SUMS', browser_download_url: prefix + 'SHA256SUMS' }] };
}

test('select only exact platform artifacts and explicit unsigned consent', () => {
  for (const unsigned of [false, true]) {
    const options = parseArgs(['plan', '--platform', 'win32-amd64', '--version', tag, ...(unsigned ? ['--allow-unsigned'] : [])]);
    const selected = selectAsset(release(unsigned), options);
    assert.equal(selected.unsigned, unsigned);
    assert.equal(selected.tag, tag);
    assert.equal(selected.asset.name, `Eden-${tag.slice(1)}-windows-amd64-setup${unsigned ? '-unsigned' : ''}.exe`);
  }
  assert.throws(() => selectAsset(release(true), { platform: 'win32-amd64' }), { code: 'unsigned_package_requires_consent', next_action: 'user_decision' });
  const both = release(true);
  both.assets.push(release().assets[0]);
  for (const allowUnsigned of [false, true]) {
    assert.equal(selectAsset(both, { platform: 'win32-amd64', allowUnsigned }).unsigned, false);
  }
  assert.throws(() => parseArgs(['download', '--platform', 'win32-amd64', '--destination', 'relative']), { code: 'invalid_input' });
  assert.throws(() => parseArgs(['--platform', 'win32-amd64', '--version', '../../bad']), { code: 'invalid_input' });
  assert.equal(parseArgs(['--platform', 'win32-amd64', '--version', tag.slice(1)]).version, tag);
  for (const index of [0, 1]) {
    const wrong = release(); wrong.assets[index].browser_download_url = 'https://example.org/package';
    assert.throws(() => selectAsset(wrong, { platform: 'win32-amd64' }), { code: 'invalid_release' });
  }
  assert.throws(() => selectAsset(release(), { platform: 'win32-amd64', version: 'v9.9.9' }), { code: 'invalid_release' });
  assert.throws(() => selectAsset({ ...release(), draft: true }, { platform: 'win32-amd64' }), { code: 'invalid_release' });
  assert.throws(() => selectAsset({ ...release(), assets: [release().assets[0]] }, { platform: 'win32-amd64' }), { code: 'asset_unavailable' });
});

test('reject unsupported desktop platforms at argument and selection boundaries', () => {
  for (const platform of ['darwin-arm64', 'darwin-amd64', 'linux-amd64', 'linux-arm64', 'win32-arm64', 'win32-ia32', 'android-arm64']) {
    assert.throws(() => parseArgs(['plan', '--platform', platform, '--allow-unsigned']), { code: 'unsupported_platform' });
    assert.throws(() => selectAsset(release(), { platform, allowUnsigned: true }), { code: 'unsupported_platform' });
  }
  const wrong = release();
  wrong.assets[0].name = `Eden-${tag.slice(1)}-windows-arm64-setup.exe`;
  wrong.assets[0].browser_download_url = prefix + wrong.assets[0].name;
  assert.throws(() => selectAsset(wrong, { platform: 'win32-amd64' }), { code: 'asset_unavailable' });
});

test('checksum manifest must identify exactly one package', () => {
  const name = release().assets[0].name;
  assert.equal(checksumFor(`${hash}  ${name}\r\n`, name), hash);
  assert.equal(checksumFor(`${hash.toUpperCase()} *${name}\n`, name), hash);
  assert.throws(() => checksumFor(`${hash}  other.exe`, name), { code: 'invalid_checksum_manifest' });
  assert.throws(() => checksumFor(`${hash}  ${name}\n${hash} *${name}`, name), { code: 'invalid_checksum_manifest' });
});

test('plan pins metadata and sums to one release; downloads are verified and exclusive', async t => {
  const options = parseArgs(['plan', '--platform', 'win32-amd64', '--version', tag]);
  const metadata = release();
  const requests = [];
  const fetchFixture = async url => {
    requests.push(String(url));
    if (String(url).startsWith('https://api.github.com/')) return new Response(JSON.stringify(metadata));
    if (String(url).endsWith('SHA256SUMS')) return new Response(`${hash}  ${metadata.assets[0].name}\n`);
    return new Response(data);
  };
  const plan = await makePlan(options, fetchFixture);
  assert.deepEqual(requests, [`https://api.github.com/repos/CarterLeeAlt/Claude-Eden/releases/tags/${tag}`, prefix + 'SHA256SUMS']);
  assert.equal(plan.sha256, hash);
  assert.equal(plan.cli, '<installation_directory>\\resources\\eden.exe');
  assert.equal(plan.unsigned, false);
  assert.deepEqual(plan.installation.argv, ['<downloaded_absolute_path>', '/S']);
  assert.ok(plan.data_policy.includes('~/.eden'));
  const directory = await mkdtemp(join(tmpdir(), 'eden-bootstrap-'));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const result = await downloadPlan(plan, directory, fetchFixture);
  assert.deepEqual(await readFile(result.path), data);
  assert.deepEqual(result.installation.argv, [result.path, '/S']);
  assert.deepEqual(plan.installation.argv, ['<downloaded_absolute_path>', '/S']);
  await writeFile(result.path, 'existing user file');
  await assert.rejects(downloadPlan(plan, directory, fetchFixture), { code: 'EEXIST' });
  assert.equal(await readFile(result.path, 'utf8'), 'existing user file');
  await assert.rejects(downloadPlan({ ...plan, asset: 'bad.exe', sha256: '0'.repeat(64) }, directory, fetchFixture), { code: 'checksum_mismatch' });
  await assert.rejects(downloadPlan({ ...plan, asset: 'large.exe', size: data.length - 1 }, directory, fetchFixture), { code: 'size_mismatch' });
  await assert.rejects(downloadPlan({ ...plan, asset: 'short.exe', size: data.length + 1 }, directory, fetchFixture), { code: 'checksum_mismatch' });
  assert.deepEqual(await readdir(directory), [plan.asset]);
});

test('unsigned plans require consent before fetching checksums', async () => {
  const options = parseArgs(['plan', '--platform', 'win32-amd64', '--version', tag]);
  const metadata = release(true);
  const requests = [];
  const fetchFixture = async url => {
    requests.push(String(url));
    return new Response(String(url).startsWith('https://api.github.com/') ? JSON.stringify(metadata) : `${hash}  ${metadata.assets[0].name}\n`);
  };
  await assert.rejects(makePlan(options, fetchFixture), { code: 'unsigned_package_requires_consent' });
  assert.equal(requests.length, 1);
  const plan = await makePlan({ ...options, allowUnsigned: true }, fetchFixture);
  assert.equal(plan.unsigned, true);
  assert.equal(plan.sha256, hash);
});

test('validate each redirect and redact transport errors', async () => {
  let requests = 0;
  const response = await httpsResponse('https://example.org/start', async (url, options) => {
    assert.equal(options.redirect, 'manual');
    requests++;
    if (requests === 1) return new Response(null, { status: 302, headers: { location: '/package' } });
    assert.equal(String(url), 'https://example.org/package');
    return new Response(data);
  });
  assert.deepEqual(Buffer.from(await response.arrayBuffer()), data);
  assert.equal(requests, 2);
  await assert.rejects(httpsResponse('https://example.org/', async () => new Response(null, { status: 302, headers: { location: 'http://example.org/' } })), { code: 'unsafe_download' });
  await assert.rejects(httpsResponse('https://example.org/', async () => new Response(null, { status: 302, headers: { location: 'https://user:password@example.org/' } })), { code: 'unsafe_download' });
  await assert.rejects(httpsResponse('https://example.org/', async () => new Response(null, { status: 302, headers: { location: '/loop' } })), { code: 'download_failed' });
  await assert.rejects(httpsResponse('https://example.org/', async () => { throw new Error('https://secret.example/private?token=do-not-echo'); }), error => error.code === 'download_connection_failed' && !error.message.includes('do-not-echo'));
});

test('CLI errors are JSON with redacted network and unexpected failure details', () => {
  const moduleURL = new URL('../www/public/agent-bootstrap.mjs', import.meta.url).href;
  for (const [fixture, code] of [
    ["throw new Error('https://secret.example/private?token=do-not-echo')", 'download_connection_failed'],
    ["return new Response('not-json: do-not-echo')", 'invalid_release'],
    [`return new Response(${JSON.stringify(JSON.stringify({ ...release(), assets: 'do-not-echo' }))})`, 'bootstrap_failed'],
  ]) {
    const child = spawnSync(process.execPath, ['--input-type=module', '--eval', `
      import { main } from ${JSON.stringify(moduleURL)};
      globalThis.fetch = async () => { ${fixture}; };
      await main(['plan', '--platform', 'win32-amd64']);
    `], { encoding: 'utf8', timeout: 10000 });
    assert.equal(child.error, undefined);
    assert.equal(child.status, 1);
    assert.equal(child.stderr, '');
    const output = JSON.parse(child.stdout);
    assert.equal(output.status, 'error');
    assert.equal(output.error.code, code);
    assert.ok(!child.stdout.includes('do-not-echo'));
    assert.ok(!child.stdout.includes('secret.example'));
  }
});
