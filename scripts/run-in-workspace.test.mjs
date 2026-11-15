import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, readdirSync, existsSync, copyFileSync, symlinkSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { prepareWorkspace, runInWorkspace } from './run-in-workspace.mjs';

const root = fileURLToPath(new URL('../', import.meta.url));
const runner = fileURLToPath(new URL('./run-in-workspace.mjs', import.meta.url));
const temporary = path.join(root, '.tmp');
assert.equal(path.resolve(process.env.TEMP), path.resolve(temporary), 'Bootstrap these tests with the workspace environment or runner.');

function fixture(t) {
  const directory = mkdtempSync(path.join(temporary, 'workspace-runner-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const repo = path.join(directory, 'repo');
  mkdirSync(path.join(repo, 'scripts'), { recursive: true });
  copyFileSync(runner, path.join(repo, 'scripts', 'run-in-workspace.mjs'));
  return { directory, repo, script: path.join(repo, 'scripts', 'run-in-workspace.mjs') };
}

function cli(script, args, options = {}) {
  return spawnSync(process.execPath, [script, ...args], { encoding: 'utf8', timeout: 120_000, ...options });
}

test('CLI preserves arguments, inherited settings and cwd; maps every user/cache path before launch', t => {
  const { repo, script } = fixture(t);
  const cwd = path.join(repo, 'Unicode 资料');
  mkdirSync(cwd);
  const probe = path.join(repo, 'probe.mjs');
  writeFileSync(probe, `console.log(JSON.stringify({args:process.argv.slice(2),env:process.env,cwd:process.cwd()}));`);
  const args = ['', 'two words', '中文🙂', 'a"b', "a'b", 'ends\\', '& echo unwanted', '%USERPROFILE%', '$(echo unwanted)', '; exit 8'];
  const result = cli(script, ['--cwd', cwd, '--', process.execPath, probe, ...args], { env: { ...process.env, RUNNER_TEST_INHERITED: 'kept', HOME: 'invalid', ...(process.platform === 'win32' ? { home: 'shadow', Tmp: 'shadow' } : {}) } });
  assert.equal(result.status, 0, result.stderr);
  const actual = JSON.parse(result.stdout);
  assert.deepEqual(actual.args, args);
  assert.equal(actual.cwd, cwd);
  assert.equal(actual.env.RUNNER_TEST_INHERITED, 'kept');
  assert.equal(actual.env.PATH || actual.env.Path, process.env.PATH || process.env.Path);
  for (const name of ['HOME','USERPROFILE','APPDATA','LOCALAPPDATA','XDG_CONFIG_HOME','XDG_CACHE_HOME','XDG_DATA_HOME','GOPATH','GOMODCACHE','GOCACHE','GOTMPDIR','TEMP','TMP','TMPDIR','RUNNER_TEMP','DOTNET_CLI_HOME','DOTNET_BUNDLE_EXTRACT_BASE_DIR','NUGET_PACKAGES','NUGET_HTTP_CACHE_PATH','npm_config_cache','PIP_CACHE_DIR','UV_CACHE_DIR','UV_PYTHON_INSTALL_DIR','PYTHONPYCACHEPREFIX','GIT_CONFIG_GLOBAL','GOENV','PSModuleAnalysisCachePath']) {
    assert.ok(actual.env[name].startsWith(path.join(repo, '.tmp') + path.sep) || actual.env[name] === path.join(repo, '.tmp'), name);
    assert.equal(existsSync(['GIT_CONFIG_GLOBAL','GOENV','PSModuleAnalysisCachePath'].includes(name) ? path.dirname(actual.env[name]) : actual.env[name]), true, name);
  }
  assert.equal(actual.env.TEMP, path.join(repo, '.tmp'));
  assert.equal(actual.env.GOTMPDIR, actual.env.TEMP);
  assert.equal(actual.env.GOCACHE, path.join(repo, '.tmp/go/cache'));
  assert.equal(actual.env.GOMODCACHE, path.join(repo, '.tmp/go/pkg/mod'));
  assert.equal(actual.env.NUGET_PACKAGES, path.join(repo, '.tmp/nuget/packages'));
  assert.equal(actual.env.npm_config_cache, path.join(repo, '.tmp/npm-cache'));
  assert.equal(actual.env.DOTNET_GENERATE_ASPNET_CERTIFICATE, 'false');
  assert.equal(actual.env.DOTNET_CLI_TELEMETRY_OPTOUT, '1');
  const checked = cli(script, ['--check'], { env: actual.env });
  assert.equal(checked.status, 0, checked.stderr);
  const invalid = cli(script, ['--check'], { env: { ...actual.env, LOCALAPPDATA: path.dirname(repo) } });
  assert.notEqual(invalid.status, 0);
  assert.match(invalid.stderr, /LOCALAPPDATA/);
});

test('explicit application paths cannot override the boundary and unset EDEN_CONFIG preserves CLI mode', t => {
  const variables = ['EDEN_CONFIG', 'AGENT_VM_STATE_DIR', 'AGENT_VM_LIMA_HOME', 'EDEN_LOGICAL_LIMA_HOME', 'LIMA_HOME', 'AGENT_VM_BIN_DIR'];
  const { directory, repo, script } = fixture(t);
  const inherited = { ...process.env };
  for (const name of Object.keys(inherited)) if (variables.includes(name.toUpperCase())) delete inherited[name];
  const outside = path.join(directory, 'outside');
  mkdirSync(outside);
  symlinkSync(outside, path.join(repo, 'escape'), process.platform === 'win32' ? 'junction' : 'dir');
  for (const name of variables) {
    for (const value of [outside, path.join(repo, 'escape', 'future')]) {
      const result = cli(script, ['--', process.execPath, '-e', 'process.exit(99)'], { env: { ...inherited, [name]: value } });
      assert.equal(result.status, 1);
      assert.ok(result.stderr.includes(name), result.stderr);
      assert.equal(existsSync(path.join(repo, '.tmp')), false);
      assert.deepEqual(readdirSync(outside), []);
    }
  }
  const unset = prepareWorkspace(repo, inherited);
  assert.equal(Object.hasOwn(unset.env, 'EDEN_CONFIG'), false);
  const set = prepareWorkspace(repo, { ...inherited, EDEN_CONFIG: 'profile/config.json', AGENT_VM_BIN_DIR: path.join(repo, 'tools') });
  assert.equal(set.env.EDEN_CONFIG, 'profile/config.json');
  assert.equal(set.env.AGENT_VM_BIN_DIR, path.join(repo, 'tools'));
});

test('preflight rejects an escaping ancestor, nested junction and dangling link before any isolated mkdir', t => {
  for (const relative of ['.tmp', '.tmp/user-profile/AppData', '.tmp/go/pkg', '.tmp/user-profile/.gitconfig']) {
    const { directory, repo, script } = fixture(t);
    const outside = path.join(directory, 'outside');
    mkdirSync(outside);
    const link = path.join(repo, relative);
    mkdirSync(path.dirname(link), { recursive: true });
    symlinkSync(outside, link, process.platform === 'win32' ? 'junction' : 'dir');
    const before = readdirSync(repo, { recursive: true }).sort();
    const result = cli(script, ['--', process.execPath, '-e', 'process.exit(99)']);
    assert.equal(result.status, 1, result.stderr);
    assert.match(result.stderr, /ancestor resolves outside/);
    assert.deepEqual(readdirSync(outside), []);
    assert.deepEqual(readdirSync(repo, { recursive: true }).sort(), before);
  }
  const { directory, repo, script } = fixture(t);
  const missing = path.join(directory, 'missing');
  symlinkSync(missing, path.join(repo, '.tmp'), process.platform === 'win32' ? 'junction' : 'dir');
  assert.notEqual(cli(script, ['--', process.execPath, '-e', 'process.exit(99)']).status, 0);
  assert.equal(existsSync(missing), false);
});

test('cwd traversal, sibling prefix and escaping cwd links are rejected without creating .tmp', t => {
  const { directory, repo, script } = fixture(t);
  const outside = path.join(directory, 'repo-other');
  mkdirSync(outside);
  symlinkSync(outside, path.join(repo, 'linked'), process.platform === 'win32' ? 'junction' : 'dir');
  for (const cwd of ['..', outside, path.parse(repo).root, 'linked']) {
    const result = cli(script, ['--cwd', cwd, '--', process.execPath, '-e', 'process.exit(99)']);
    assert.equal(result.status, 1);
    assert.match(result.stderr, /outside the workspace/);
    assert.equal(existsSync(path.join(repo, '.tmp')), false);
  }
  const inside = path.join(repo, 'inside');
  mkdirSync(inside);
  symlinkSync(inside, path.join(repo, 'internal-link'), process.platform === 'win32' ? 'junction' : 'dir');
  assert.equal(cli(script, ['--cwd', 'internal-link', '--', process.execPath, '-e', 'process.exit(0)']).status, 0);
});

test('invalid syntax, path types, absent executable and batch commands stop with actionable errors', t => {
  const { repo, script } = fixture(t);
  for (const args of [[], ['node'], ['--cwd'], ['--'], ['--unknown', '--', 'node']]) assert.equal(cli(script, args).status, 1);
  let result = cli(script, ['--', 'npm.cmd', 'run', 'build']);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /npm-cli\.js/);
  result = cli(script, ['--', 'workspace-command-that-does-not-exist']);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /ENOENT/);
  const wrong = fixture(t);
  writeFileSync(path.join(wrong.repo, '.tmp'), 'not a directory');
  assert.equal(cli(wrong.script, ['--', process.execPath, '-e', 'process.exit(99)']).status, 1);
  if (process.platform === 'win32') {
    writeFileSync(path.join(repo, 'unsafe.cmd'), '@exit /b 99');
    result = cli(script, ['--', 'unsafe']);
    assert.equal(result.status, 1);
    assert.match(result.stderr, /npm-cli\.js/);
  }
});

test('child failure exit status passes through unchanged', t => {
  const { script } = fixture(t);
  for (const code of [0, 7, 42]) assert.equal(cli(script, ['--', process.execPath, '-e', `process.exit(${code})`]).status, code);
});

test('Go -C cannot escape the workspace or change the meaning of relative configuration paths', t => {
  const { directory, repo, script } = fixture(t);
  const outside = path.join(directory, 'outside');
  mkdirSync(outside);
  symlinkSync(outside, path.join(repo, 'escape'), process.platform === 'win32' ? 'junction' : 'dir');
  for (const destination of [outside, '../outside', 'escape']) {
    for (const option of [['-C', destination], [`-C=${destination}`]]) {
      const result = cli(script, ['--', 'go', ...option, 'version']);
      assert.equal(result.status, 1, result.stdout);
      assert.match(result.stderr, /outside the workspace/);
    }
  }
  mkdirSync(path.join(repo, 'tools/nested'), { recursive: true });
  const result = cli(script, ['--cwd', 'tools/nested', '--', 'go', '-C', '../..', 'version'], {
    env: { ...process.env, EDEN_CONFIG: '../../profile/config.json' },
  });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /EDEN_CONFIG: Path is outside the workspace/);
  assert.deepEqual(readdirSync(outside), []);
});

test('cancellation stops the owned child and grandchild and returns a cancellation exit code', { timeout: 15_000 }, async t => {
  const { repo } = fixture(t);
  const ready = path.join(repo, 'ready.json');
  const child = path.join(repo, 'child.mjs');
  const grandchild = path.join(repo, 'grandchild.mjs');
  writeFileSync(grandchild, 'setInterval(() => {}, 1000);');
  writeFileSync(child, `import {spawn} from 'node:child_process'; import {writeFileSync} from 'node:fs';
const child=spawn(process.execPath,[process.argv[2]],{stdio:'inherit',windowsHide:true});
child.once('spawn',()=>writeFileSync(process.argv[3],JSON.stringify([process.pid,child.pid])));
setInterval(()=>{},1000);`);
  const controller = new AbortController();
  const running = runInWorkspace(process.execPath, [child, grandchild, ready], { root: repo, signal: controller.signal });
  t.after(() => controller.abort('SIGTERM'));
  const deadline = Date.now() + 5000;
  while (!existsSync(ready)) {
    assert.ok(Date.now() < deadline, 'child did not become ready');
    await new Promise(resolve => setTimeout(resolve, 20));
  }
  const identifiers = JSON.parse(readFileSync(ready, 'utf8'));
  controller.abort('SIGINT');
  assert.equal(await running, 130);
  for (const pid of identifiers) assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' });
  const already = new AbortController(); already.abort('SIGTERM');
  assert.equal(await runInWorkspace(process.execPath, ['-e', 'process.exit(99)'], { root: repo, signal: already.signal }), 143);
});

test('real Go telemetry and build/test caches stay in the fixture workspace; dangerous tests fail the AST gate', { timeout: 180_000 }, t => {
  const { repo, script } = fixture(t);
  const checker = path.join(root, 'scripts/check-test-paths/main.go');
  assert.ok(existsSync(checker), 'The Go AST checker must be installed before running this integration test.');
  mkdirSync(path.join(repo, 'scripts/check-test-paths'), { recursive: true });
  copyFileSync(checker, path.join(repo, 'scripts/check-test-paths/main.go'));
  writeFileSync(path.join(repo, 'go.mod'), 'module workspace.test/probe\n\ngo 1.26.0\n');
  const environment = cli(script, ['--', 'go', 'env', '-json', 'GOTELEMETRYDIR', 'GOCACHE', 'GOMODCACHE', 'GOTMPDIR', 'GOENV'], { env: { ...process.env, GOTOOLCHAIN: 'local' } });
  assert.equal(environment.status, 0, environment.stderr);
  const settings = JSON.parse(environment.stdout);
  for (const [name, value] of Object.entries(settings)) assert.ok(value.startsWith(path.join(repo, '.tmp')), `${name}: ${value}`);
  const source = `package probe
import ("os"; "path/filepath"; "strings"; "testing")
func TestPaths(t *testing.T) {
 base := os.Getenv("EDEN_WORKSPACE_ROOT") + string(os.PathSeparator)
 for _, value := range []string{t.TempDir(), os.TempDir()} { if !strings.HasPrefix(value, base) { t.Fatalf("outside workspace: %s", value) } }
 if err := os.WriteFile(filepath.Join(t.TempDir(), "marker"), []byte("isolated"), 0600); err != nil { t.Fatal(err) }
}
func TestCacheable(t *testing.T) { if os.Getenv("EDEN_WORKSPACE_ROOT") == "" { t.Fatal("missing workspace") } }
`;
  writeFileSync(path.join(repo, 'probe_test.go'), source);
  const run = () => cli(script, ['--', 'go', 'test', '-v', '.'], { env: { ...process.env, GOTOOLCHAIN: 'local' } });
  const first = run();
  assert.equal(first.status, 0, first.stderr);
  assert.match(first.stdout, /PASS/);
  assert.ok(readdirSync(settings.GOCACHE).length > 0, 'real Go cache was not populated');
  assert.ok(existsSync(settings.GOTELEMETRYDIR), 'real Go telemetry directory was not initialized');
  // TempDir observes its parent directory; compiler scratch changes that directory
  // between invocations. A separate stable test verifies result-cache reuse.
  const cached = () => cli(script, ['--', 'go', 'test', '-v', '-run', '^TestCacheable$', '.'], { env: { ...process.env, GOTOOLCHAIN: 'local' } });
  assert.equal(cached().status, 0);
  assert.match(cached().stdout, /\(cached\)/, 'real Go test result cache was not reused');
  const nested = path.join(repo, 'tools', 'nested');
  mkdirSync(nested, { recursive: true });
  writeFileSync(path.join(nested, 'go.mod'), 'module workspace.test/nested\n\ngo 1.26.0\n');
  writeFileSync(path.join(nested, 'probe_test.go'), source);
  const submodule = cli(script, ['--cwd', 'tools/nested', '--', 'go', 'test', '-v', '.'], { env: { ...process.env, GOTOOLCHAIN: 'local' } });
  assert.equal(submodule.status, 0, submodule.stderr);
  assert.match(submodule.stdout, /PASS/);
  assert.doesNotMatch(submodule.stdout, /Scanned \d+ Go test files/, 'preflight diagnostics must not corrupt child stdout');
  for (const option of [['-C', 'tools/nested'], ['-C=tools/nested']]) {
    const redirected = cli(script, ['--', 'go', ...option, 'test', '-v', '.'], { env: { ...process.env, GOTOOLCHAIN: 'local' } });
    assert.equal(redirected.status, 0, redirected.stderr);
    assert.match(redirected.stdout, /PASS/);
  }
  // The runtime selection excludes the unsafe test even if the AST gate regresses.
  writeFileSync(path.join(repo, 'unsafe_test.go'), 'package probe\nimport ("os"; "testing")\nfunc TestDangerous(t *testing.T) { if err := os.WriteFile("C:/workspace-runner-must-never-write", nil, 0600); err != nil { t.Fatal(err) } }\n');
  const rejected = cli(script, ['--', 'go', 'test', '-v', '-run', '^TestPaths$', '.'], { env: { ...process.env, GOTOOLCHAIN: 'local' } });
  assert.notEqual(rejected.status, 0);
  assert.doesNotMatch(rejected.stdout, /=== RUN|--- PASS/);
});
