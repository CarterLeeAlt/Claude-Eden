import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtempSync, readFileSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { test } from 'node:test';
import { releaseAssets, checksumFor, releaseMetadata, platformMetadata, resolvePlatforms, request, publicManifestRelease } from './sync-release.mjs';

test('exhausted version API falls back to anonymous public manifest and attachment checks', async () => {
  const calls = [];
  const tag = 'v0.2.0';
  const manifest = `${'a'.repeat(64)}  Eden-0.2.0-windows-amd64-setup.exe\n${'b'.repeat(64)}  mihomo-v1.19.33-source.tar.gz\n`;
  const fetchImpl = async (url, options) => {
    calls.push({ url, ...options });
    assert.equal(options.headers.Authorization, undefined);
    if (url.startsWith('https://api.github.com/')) return new Response('', { status: 403, headers: { 'x-ratelimit-remaining': '0' } });
    if (url.endsWith('/SHA256SUMS')) return new Response(manifest);
    assert.equal(options.method, 'HEAD');
    return new Response('', { headers: { 'content-length': '100' } });
  };
  const response = await request(`https://api.github.com/repos/CarterLeeAlt/Claude-Eden/releases/tags/${tag}`, 1000, { fetchImpl });
  assert.equal(releaseAssets(await response.json()).version, '0.2.0');
  assert.equal(calls.length, 4);
});

test('ordinary API errors, unversioned lists and other origins do not use a fallback', async () => {
  for (const [url, status, headers] of [
    ['https://api.github.com/repos/CarterLeeAlt/Claude-Eden/releases/tags/v0.2.0', 404, {}],
    ['https://api.github.com/repos/CarterLeeAlt/Claude-Eden/releases/tags/v0.2.0', 403, {}],
    ['https://api.github.com/repos/CarterLeeAlt/Claude-Eden/releases', 403, { 'x-ratelimit-remaining': '0' }],
    ['https://github.com/CarterLeeAlt/Claude-Eden/releases/download/v0.2.0/SHA256SUMS', 403, { 'x-ratelimit-remaining': '0' }],
    ['https://example.test/repos/CarterLeeAlt/Claude-Eden/releases', 403, { 'x-ratelimit-remaining': '0' }],
  ]) {
    let count = 0;
    await assert.rejects(request(url, 1000, { fetchImpl: async (_url, options) => {
      count++; assert.equal(options.headers.Authorization, undefined);
      return new Response('', { status, headers });
    } }));
    assert.equal(count, 1);
  }
});

test('manifest fallback rejects missing bytes, duplicate digests, traversal and wrong versions', async () => {
  const valid = `${'a'.repeat(64)}  Eden-0.2.0-windows-amd64-setup.exe\n${'b'.repeat(64)}  mihomo-v1.19.33-source.tar.gz\n`;
  for (const [text, missing] of [[valid, true], [valid + valid, false], [valid.replace('Eden-', '../Eden-'), false], [valid.replace('0.2.0-windows', '0.1.0-windows'), false], ['', false]]) {
    await assert.rejects(publicManifestRelease('v0.2.0', async (url) => {
      if (url.endsWith('/SHA256SUMS')) return new Response(text);
      return new Response('', { status: missing ? 404 : 200, headers: { 'content-length': '100' } });
    }));
  }
});

const hash = 'a'.repeat(64);
function fixture() {
  return { tag_name: 'v0.2.0-alpha.10', draft: false, prerelease: true, assets:
    ['Eden-0.2.0-alpha.10-windows-amd64-setup.exe', 'SHA256SUMS', 'mihomo-v1.19.33-source.tar.gz'].map(name => ({
      name, state: 'uploaded', size: 100, digest: `sha256:${hash}`,
      browser_download_url: `https://github.com/CarterLeeAlt/Claude-Eden/releases/download/v0.2.0-alpha.10/${name}`,
    })),
  };
}
test('accept complete Alpha release; reject drafts, incomplete uploads and cross-version assets', () => {
  assert.equal(releaseAssets(fixture()).version, '0.2.0-alpha.10');
  for (const mutate of [
    r => { r.draft = true; }, r => { r.assets.pop(); },
    r => { r.assets[0].state = 'new'; }, r => { r.assets[0].size = 0; },
    r => { r.assets[0].browser_download_url = r.assets[0].browser_download_url.replace('/v0.2.0-alpha.10/', '/v0.1.0/'); },
  ]) {
    const release = fixture(); mutate(release);
    assert.throws(() => releaseAssets(release));
  }
});
test('checksum must uniquely match the archive and GitHub digest', () => {
  const archive = fixture().assets[0];
  const line = `${hash}  ${archive.name}\n`;
  assert.equal(checksumFor(line, archive), hash);
  for (const text of ['', line + line, `${'b'.repeat(64)}  ${archive.name}`, `${hash}  another.exe`]) {
    assert.throws(() => checksumFor(text, archive));
  }
});

const sums = release => release.assets.filter(asset => asset.name !== 'SHA256SUMS').map(asset => `${hash}  ${asset.name}\n`).join('');

test('Windows is the primary release and metadata uses per-platform entries', () => {
  const release = fixture();
  assert.deepEqual(releaseMetadata(release, sums(release)), {
    version: '0.2.0-alpha.10',
    releaseURL: 'https://github.com/CarterLeeAlt/Claude-Eden/releases/tag/v0.2.0-alpha.10',
    platforms: { windows: {
      version: '0.2.0-alpha.10',
      releaseURL: 'https://github.com/CarterLeeAlt/Claude-Eden/releases/tag/v0.2.0-alpha.10',
      downloadURL: release.assets[0].browser_download_url,
    } },
  });
  assert.throws(() => releaseAssets({ ...release, assets: release.assets.slice(1) }));
});

test('incomplete, oversized, duplicate and cross-version Windows assets reject the primary release', () => {
  for (const mutate of [
    asset => { asset.state = 'new'; }, asset => { asset.size = 0; },
    asset => { asset.size = 512 * 1024 * 1024 + 1; },
    asset => { asset.browser_download_url = asset.browser_download_url.replace('/v0.2.0-alpha.10/', '/v0.1.0/'); },
  ]) {
    const release = fixture(); mutate(release.assets[0]);
    assert.throws(() => releaseMetadata(release, sums(release)));
    assert.deepEqual(platformMetadata(release, sums(release)), {});
  }
  const duplicate = fixture(); duplicate.assets.push({ ...duplicate.assets[0] });
  assert.throws(() => releaseMetadata(duplicate, sums(duplicate)));
});

test('missing, duplicate or mismatching Windows and source checksums reject the primary release', () => {
  const release = fixture();
  const all = sums(release);
  for (const asset of [release.assets[0], release.assets[2]]) {
    for (const text of [all.replace(`${hash}  ${asset.name}\n`, ''), all + `${hash}  ${asset.name}\n`, all.replace(`${hash}  ${asset.name}`, `${'b'.repeat(64)}  ${asset.name}`)]) {
      assert.throws(() => releaseMetadata(release, text));
      assert.deepEqual(platformMetadata(release, text), {});
    }
  }
});

function androidFixture() {
  const release = fixture();
  const name = `Eden-${release.tag_name.slice(1)}-android-arm64.apk`;
  release.assets.push({ ...release.assets[0], name,
    browser_download_url: `https://github.com/CarterLeeAlt/Claude-Eden/releases/download/${release.tag_name}/${name}` });
  return release;
}

function versionedFixture(version, platforms) {
  const release = { tag_name: `v${version}`, prerelease: version.includes('-'), draft: false, assets: [] };
  for (const suffix of ['windows-amd64-setup.exe', 'android-arm64.apk'].filter((_, index) => platforms.includes(index))) {
    const name = `Eden-${version}-${suffix}`;
    release.assets.push({ name, state: 'uploaded', size: 100, digest: `sha256:${hash}`,
      browser_download_url: `https://github.com/CarterLeeAlt/Claude-Eden/releases/download/${release.tag_name}/${name}` });
  }
  if (platforms.includes(0)) release.assets.push({ ...fixture().assets[2], browser_download_url: `https://github.com/CarterLeeAlt/Claude-Eden/releases/download/${release.tag_name}/mihomo-v1.19.33-source.tar.gz` });
  return release;
}

test('each platform retains its latest verified version and release notes independently', async () => {
  const latest = versionedFixture('0.3.0-rc', [0]);
  const older = versionedFixture('0.2.8', [0, 1]);
  const platforms = await resolvePlatforms([latest, older], async release => sums(release));
  for (const [platform, version] of Object.entries({ windows: '0.3.0-rc', android: '0.2.8' })) {
    assert.equal(platforms[platform].version, version);
    assert.equal(platforms[platform].releaseURL, `https://github.com/CarterLeeAlt/Claude-Eden/releases/tag/v${version}`);
    assert.ok(platforms[platform].downloadURL.includes(`/v${version}/Eden-${version}-`));
  }
});

test('new Android package replaces the fallback without relabeling other platforms', async () => {
  const latest = versionedFixture('0.3.0-rc', [0, 1]);
  const platforms = await resolvePlatforms([latest, versionedFixture('0.2.9', [1])], async release => sums(release));
  assert.deepEqual(platforms, platformMetadata(latest, sums(latest)));
  assert.equal(platforms.android.version, '0.3.0-rc');
});

test('drafts, invalid assets, missing checksums and failed manifests preserve the older package', async () => {
  const older = versionedFixture('0.2.0', [0, 1]);
  for (const mutate of [
    r => { r.draft = true; }, r => { r.assets.at(-1).state = 'new'; },
    r => { r.assets.at(-1).browser_download_url = older.assets[1].browser_download_url; },
    r => { r.assets.push({ ...r.assets.at(-1) }); },
    r => { r.assets.at(-1).digest = `sha256:${'b'.repeat(64)}`; },
  ]) {
    const invalid = versionedFixture('0.3.0', [1]); mutate(invalid);
    const platforms = await resolvePlatforms([invalid, older], async release => sums(release));
    assert.equal(platforms.android.version, '0.2.0');
  }
  for (const fail of [false, true]) {
    const latest = versionedFixture('0.3.0', [1]);
    const platforms = await resolvePlatforms([latest, older], async release => {
      if (release === latest) { if (fail) throw new Error('manifest unavailable'); return ''; }
      return sums(release);
    });
    assert.equal(platforms.android.version, '0.2.0');
  }
});

test('unsigned Windows fallback retains its version and unsigned status', async () => {
  const unsigned = unsignedFixture();
  const platforms = await resolvePlatforms([versionedFixture('0.3.0', [1]), unsigned], async release => sums(release));
  assert.equal(platforms.windows.version, '0.2.0-alpha.10');
  assert.equal(platforms.windows.unsigned, true);
});

test('website CLI discovers paginated releases and pins fallbacks to the requested release or older', () => {
  const directory = mkdtempSync(join(tmpdir(), 'eden-platform-fallback-'));
  const latest = versionedFixture('0.3.0', [0]); latest.published_at = '2026-11-14T00:00:00Z';
  const older = versionedFixture('0.2.0', [0, 1]); older.published_at = '2026-11-13T00:00:00Z';
  for (const release of [latest, older]) {
    const text = sums(release);
    release.assets.push({ name: 'SHA256SUMS', state: 'uploaded', size: Buffer.byteLength(text), browser_download_url: `https://github.com/CarterLeeAlt/Claude-Eden/releases/download/${release.tag_name}/SHA256SUMS` });
  }
  const mock = join(directory, 'fetch-fixture.mjs');
  writeFileSync(mock, `
    const releases = ${JSON.stringify([latest, older])};
    const sums = ${JSON.stringify(Object.fromEntries([latest, older].map(release => [release.tag_name, sums(release)])))};
    globalThis.fetch = async (url, options) => {
      if (options.headers.Authorization) throw new Error('Public download used authentication');
      if (url.endsWith('releases?per_page=100')) return new Response(JSON.stringify([releases[0]]), { headers: { link: '<https://api.github.com/repos/CarterLeeAlt/Claude-Eden/releases?page=2>; rel="next"' } });
      if (url.endsWith('releases?page=2')) return new Response(JSON.stringify([releases[1]]));
      for (const release of releases) {
        if (url.endsWith('/tags/' + release.tag_name)) return new Response(JSON.stringify(release));
        if (url.endsWith('/' + release.tag_name + '/SHA256SUMS')) return new Response(sums[release.tag_name]);
      }
      throw new Error('Unexpected request: ' + url);
    };
  `);
  try {
    for (const tag of [undefined, latest.tag_name, older.tag_name]) {
      const output = join(directory, 'release.json');
      const result = spawnSync(process.execPath, ['--import', pathToFileURL(mock).href, fileURLToPath(new URL('./sync-release.mjs', import.meta.url)), '--output', output, ...(tag ? ['--tag', tag] : [])], { encoding: 'utf8' });
      assert.equal(result.status, 0, result.stderr);
      const metadata = JSON.parse(readFileSync(output, 'utf8'));
      assert.equal(metadata.platforms.android.version, '0.2.0');
      assert.equal(metadata.platforms.windows.version, tag === older.tag_name ? '0.2.0' : '0.3.0');
    }
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

test('Android APK is optional and requires a unique same-version asset and verified checksum', () => {
  const release = androidFixture();
  const apk = release.assets.at(-1);
  assert.equal(releaseAssets(release).android.name, apk.name);
  assert.equal(releaseMetadata(release, sums(release)).platforms.android.downloadURL, apk.browser_download_url);
  assert.ok(!releaseMetadata(fixture(), sums(fixture())).platforms.android);
  for (const mutate of [
    asset => { asset.name = 'eden-mobile-debug.apk'; },
    asset => { asset.name = asset.name.replace('arm64', 'amd64'); },
    asset => { asset.state = 'new'; }, asset => { asset.size = 0; },
    asset => { asset.size = 512 * 1024 * 1024 + 1; },
    asset => { asset.browser_download_url = asset.browser_download_url.replace(release.tag_name, 'v0.1.0'); },
  ]) {
    const invalid = structuredClone(release); mutate(invalid.assets.at(-1));
    const metadata = releaseMetadata(invalid, sums(invalid));
    assert.ok(!metadata.platforms.android);
    assert.equal(metadata.platforms.windows.downloadURL, release.assets[0].browser_download_url);
  }
  const duplicate = structuredClone(release); duplicate.assets.push({ ...apk });
  assert.ok(!releaseMetadata(duplicate, sums(duplicate)).platforms.android);
  for (const text of [sums(fixture()), sums(release) + `${hash}  ${apk.name}\n`, sums(release).replace(`${hash}  ${apk.name}`, `${'b'.repeat(64)}  ${apk.name}`)]) {
    assert.ok(!releaseMetadata(release, text).platforms.android);
  }
});

test('archive-directory downloads the verified Android APK through the shipped CLI', () => {
  const directory = mkdtempSync(join(tmpdir(), 'eden-android-download-'));
  const release = androidFixture();
  const payloads = {};
  for (const asset of release.assets.filter(asset => asset.name !== 'SHA256SUMS')) {
    const bytes = Buffer.from(`fixture bytes for ${asset.name}\n`);
    asset.size = bytes.length;
    asset.digest = `sha256:${createHash('sha256').update(bytes).digest('hex')}`;
    payloads[asset.browser_download_url] = bytes.toString('base64');
  }
  const text = release.assets.filter(asset => asset.name !== 'SHA256SUMS').map(asset => `${asset.digest.slice(7)}  ${asset.name}\n`).join('');
  const checksums = release.assets.find(asset => asset.name === 'SHA256SUMS');
  checksums.size = Buffer.byteLength(text);
  payloads[checksums.browser_download_url] = Buffer.from(text).toString('base64');
  const mock = join(directory, 'fetch-fixture.mjs');
  writeFileSync(mock, `
    const release = ${JSON.stringify(release)};
    const payloads = ${JSON.stringify(payloads)};
    globalThis.fetch = async (url, options) => {
      if (options.headers.Authorization) throw new Error('Public download used authentication');
      if (url === 'https://api.github.com/repos/CarterLeeAlt/Claude-Eden/releases/tags/' + release.tag_name) return new Response(JSON.stringify(release));
      if (!payloads[url]) throw new Error('Unexpected download');
      return new Response(Buffer.from(payloads[url], 'base64'));
    };
  `);
  try {
    const output = join(directory, 'release.json'), downloads = join(directory, 'downloads');
    const result = spawnSync(process.execPath, ['--import', pathToFileURL(mock).href, fileURLToPath(new URL('./sync-release.mjs', import.meta.url)), '--tag', release.tag_name, '--output', output, '--archive-directory', downloads], { encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    const apk = release.assets.at(-1);
    assert.deepEqual(readFileSync(join(downloads, apk.name)), Buffer.from(payloads[apk.browser_download_url], 'base64'));
    assert.equal(JSON.parse(readFileSync(output, 'utf8')).platforms.android.downloadURL, apk.browser_download_url);
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

function unsignedFixture() {
  const release = fixture();
  release.assets.shift();
  for (const name of ['Eden-0.2.0-alpha.10-windows-amd64-setup-unsigned.exe', `qemu-${'a'.repeat(40)}-source.tar.gz`]) {
    release.assets.push({ ...fixture().assets[0], name, browser_download_url: `https://github.com/CarterLeeAlt/Claude-Eden/releases/download/${release.tag_name}/${name}` });
  }
  return release;
}

test('unsigned Windows requires a prerelease and source checksums; signed Windows takes priority', () => {
  const release = unsignedFixture();
  const metadata = releaseMetadata(release, sums(release));
  assert.equal(metadata.platforms.windows.downloadURL, release.assets[2].browser_download_url);
  assert.equal(metadata.platforms.windows.releaseURL, metadata.releaseURL);
  assert.equal(metadata.platforms.windows.unsigned, true);
  const signed = fixture().assets[0];
  const both = { ...release, assets: [...release.assets, signed] };
  assert.equal(releaseMetadata(both, sums(both)).platforms.windows.downloadURL, signed.browser_download_url);
  assert.ok(!releaseMetadata(both, sums(both)).platforms.windows.unsigned);
  for (const mutate of [
    r => { r.prerelease = false; }, r => { r.assets[2].state = 'new'; },
    r => { r.assets[2].browser_download_url = r.assets[2].browser_download_url.replace('/v0.2.0-alpha.10/', '/desktop-preview-0.2.0-alpha.10/'); },
    r => { r.assets.push({ ...r.assets[2] }); }, r => { r.assets.pop(); },
    r => { r.assets[2].digest = 'sha256:' + 'b'.repeat(64); },
  ]) {
    const invalid = structuredClone(release); mutate(invalid);
    assert.throws(() => releaseMetadata(invalid, sums(invalid)));
    assert.ok(!platformMetadata(invalid, sums(invalid)).windows);
  }
  assert.throws(() => releaseMetadata(release, sums(fixture())));
});

test('explicit RC manifest exposes Windows and Android with QEMU sources', async () => {
  const tag = 'v0.1.7-rc.1', version = tag.slice(1);
  const names = [`Eden-${version}-windows-amd64-setup.exe`, 'mihomo-v1.19.33-source.tar.gz', 'qemu-11.1.0-windows-full-source.tar.gz', `qemu-${'a'.repeat(40)}-source.tar.gz`, `qemu-keycodemapdb-${'b'.repeat(40)}-source.tar.gz`, 'qemu-11.1.0-eden-changes-source.tar.gz', `Eden-${version}-android-arm64.apk`];
  const manifest = names.map(name => `${hash}  ${name}\n`).join('');
  const release = await publicManifestRelease(tag, async (url, options = {}) => {
    assert.equal(options.headers?.Authorization, undefined);
    if (url.endsWith('/SHA256SUMS')) return new Response(manifest);
    assert.equal(options.method, 'HEAD');
    return new Response('', { headers: { 'content-length': String(url.includes('full-source') ? 700 * 1024 * 1024 : 100) } });
  });
  assert.equal(release.prerelease, true);
  assert.equal(releaseAssets(release).windows.name, names[0]);
  const metadata = releaseMetadata(release, manifest);
  assert.equal(metadata.version, version);
  assert.equal(metadata.platforms.windows.downloadURL, release.assets.find(asset => asset.name === names[0]).browser_download_url);
  assert.equal(metadata.platforms.android.downloadURL, release.assets.find(asset => asset.name === names.at(-1)).browser_download_url);
  assert.deepEqual(Object.keys(metadata.platforms).sort(), ['android', 'windows']);
  assert.ok(!metadata.platforms.windows.unsigned);
  assert.equal(metadata.releaseURL, `https://github.com/CarterLeeAlt/Claude-Eden/releases/tag/${tag}`);
});

test('manifest source allowance keeps installer and source size bounds separate and names constrained', async () => {
  const names = ['Eden-0.1.7-rc.1-windows-amd64-setup.exe', 'mihomo-v1.19.33-source.tar.gz', 'qemu-11.1.0-windows-full-source.tar.gz'];
  for (const [oversized, size] of [[names[0], 512 * 1024 * 1024 + 1], [names[2], 2048 * 1024 * 1024 + 1]]) {
    await assert.rejects(publicManifestRelease('v0.1.7-rc.1', async url => url.endsWith('/SHA256SUMS')
      ? new Response(names.map(name => `${hash}  ${name}\n`).join(''))
      : new Response('', { headers: { 'content-length': String(url.endsWith(oversized) ? size : 100) } })), /Missing or incomplete public asset/);
  }
  for (const invalid of ['qemu-11.1.0-windows-full-source.tar.gz.exe', 'qemu-latest-windows-full-source.tar.gz', 'qemu-11.1.0-windows-full-source/escape.tar.gz', 'Eden-0.1.6-android-arm64.apk', 'Eden-0.1.7-rc.1-android-arm64.apk.exe', 'Eden-0.1.7-rc.1-android-amd64.apk']) {
    await assert.rejects(publicManifestRelease('v0.1.7-rc.1', async () => new Response([...names, invalid].map(name => `${hash}  ${name}\n`).join(''))));
  }
});
