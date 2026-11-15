#!/usr/bin/env node
// Environment/path guard for trusted builds and tests, not an OS sandbox.
import { spawn, execFile } from 'node:child_process';
import { lstatSync, statSync, realpathSync, mkdirSync, existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { constants } from 'node:os';
import { promisify } from 'node:util';

const repository = realpathSync(fileURLToPath(new URL('../', import.meta.url)));
const execFileAsync = promisify(execFile);
const usage = 'node scripts/run-in-workspace.mjs [--cwd <directory inside repo>] -- <command> [arguments]\nnode scripts/run-in-workspace.mjs --check';

export function workspaceVariables(root) {
  const temporary = path.join(root, '.tmp');
  const home = path.join(temporary, 'user-profile');
  return {
    HOME: home, USERPROFILE: home,
    APPDATA: path.join(home, 'AppData', 'Roaming'), LOCALAPPDATA: path.join(home, 'AppData', 'Local'),
    XDG_CONFIG_HOME: path.join(home, '.config'), XDG_CACHE_HOME: path.join(home, '.cache'),
    XDG_DATA_HOME: path.join(home, '.local', 'share'),
    GOPATH: path.join(temporary, 'go'), GOMODCACHE: path.join(temporary, 'go', 'pkg', 'mod'),
    GOCACHE: path.join(temporary, 'go', 'cache'), GOTMPDIR: temporary,
    TEMP: temporary, TMP: temporary, TMPDIR: temporary, RUNNER_TEMP: temporary,
    DOTNET_CLI_HOME: path.join(temporary, 'dotnet-home'),
    DOTNET_BUNDLE_EXTRACT_BASE_DIR: path.join(temporary, 'dotnet-bundle'),
    NUGET_PACKAGES: path.join(temporary, 'nuget', 'packages'), NUGET_HTTP_CACHE_PATH: path.join(temporary, 'nuget', 'http'),
    npm_config_cache: path.join(temporary, 'npm-cache'), PIP_CACHE_DIR: path.join(temporary, 'pip-cache'),
    UV_CACHE_DIR: path.join(temporary, 'uv-cache'), UV_PYTHON_INSTALL_DIR: path.join(temporary, 'uv-python'),
    PYTHONPYCACHEPREFIX: path.join(temporary, 'python-cache'),
    GOENV: path.join(home, 'go', 'env'), GIT_CONFIG_GLOBAL: path.join(home, '.gitconfig'),
    PSModuleAnalysisCachePath: path.join(temporary, 'powershell-module-analysis-cache'),
  };
}

const fileVariables = new Set(['GOENV', 'GIT_CONFIG_GLOBAL', 'PSModuleAnalysisCachePath']);
const explicitPaths = new Set(['EDEN_CONFIG', 'AGENT_VM_STATE_DIR', 'AGENT_VM_LIMA_HOME', 'EDEN_LOGICAL_LIMA_HOME', 'LIMA_HOME', 'AGENT_VM_BIN_DIR']);
const settings = { DOTNET_GENERATE_ASPNET_CERTIFICATE: 'false', DOTNET_CLI_TELEMETRY_OPTOUT: '1', DOTNET_SKIP_FIRST_TIME_EXPERIENCE: '1' };
const inside = (root, target) => {
  const relative = path.relative(root, target);
  return relative === '' || (!path.isAbsolute(relative) && relative !== '..' && !relative.startsWith(`..${path.sep}`));
};

// Check every existing ancestor, including dangling links, before any mkdir.
export function assertWorkspacePath(root, target) {
  if (!inside(root, target)) throw new Error(`Path is outside the workspace: ${target}`);
  let current = root;
  for (const component of ['', ...path.relative(root, target).split(path.sep).filter(Boolean)]) {
    current = path.join(current, component);
    try { lstatSync(current); }
    catch (error) { if (error.code === 'ENOENT') break; throw error; }
    if (!inside(root, realpathSync(current))) throw new Error(`Path ancestor resolves outside the workspace: ${current}`);
  }
}

function makeDirectory(root, directory) {
  assertWorkspacePath(root, directory);
  if (!existsSync(directory)) {
    makeDirectory(root, path.dirname(directory));
    try { mkdirSync(directory); }
    catch (error) { if (error.code !== 'EEXIST') throw error; }
  }
  assertWorkspacePath(root, directory);
  if (!statSync(directory).isDirectory()) throw new Error(`Expected a directory: ${directory}`);
}

export function prepareWorkspace(root, inherited = process.env, cwd = '.', { check = false } = {}) {
  root = realpathSync(root);
  cwd = path.resolve(root, cwd);
  assertWorkspacePath(root, cwd);
  if (!statSync(cwd).isDirectory()) throw new Error(`Working directory is not a directory: ${cwd}`);
  for (const [name, value] of Object.entries(inherited)) {
    if (!value || !explicitPaths.has(process.platform === 'win32' ? name.toUpperCase() : name)) continue;
    try { assertWorkspacePath(root, path.resolve(cwd, value)); }
    catch (error) { throw new Error(`${name}: ${error.message}`); }
  }
  const variables = workspaceVariables(root);
  for (const [name, target] of Object.entries(variables)) {
    assertWorkspacePath(root, target);
    if (existsSync(target) && statSync(target).isDirectory() === fileVariables.has(name)) {
      throw new Error(`Unexpected path type for ${name}: ${target}`);
    }
  }
  const expected = { ...variables, ...settings, EDEN_WORKSPACE_ROOT: root };
  if (process.platform === 'win32') {
    expected.HOMEDRIVE = path.parse(variables.HOME).root.replace(/[\\/]$/, '');
    expected.HOMEPATH = variables.HOME.slice(expected.HOMEDRIVE.length);
  }
  if (check) {
    for (const [name, value] of Object.entries(expected)) {
      const entries = Object.entries(inherited).filter(([key]) => process.platform === 'win32' ? key.toLowerCase() === name.toLowerCase() : key === name);
      if (entries.length !== 1 || entries[0][1] !== value) throw new Error(`Workspace environment is not active (${name}). Start with: ${usage.split('\n')[0]}`);
    }
    return { root, cwd, env: inherited };
  }
  for (const [name, target] of Object.entries(variables)) makeDirectory(root, fileVariables.has(name) ? path.dirname(target) : target);
  const env = { ...inherited };
  if (process.platform === 'win32') {
    const names = new Set(Object.keys(expected).map(name => name.toLowerCase()));
    for (const key of Object.keys(env)) if (names.has(key.toLowerCase())) delete env[key];
  }
  return { root, cwd, env: { ...env, ...expected } };
}

function executable(command, cwd, env) {
  const batchError = () => new Error('Windows .cmd/.bat commands are not supported without shell reparsing. Use node <workspace-node>/node_modules/npm/bin/npm-cli.js instead of npm.cmd (or npm).');
  if (/\.(cmd|bat)$/i.test(command)) throw batchError();
  if (process.platform !== 'win32') return command;
  const candidates = path.isAbsolute(command) || /[\\/]/.test(command)
    ? [path.resolve(cwd, command)]
    : [cwd, ...(env.PATH || env.Path || '').split(path.delimiter)].map(directory => path.join(directory, command));
  for (const candidate of candidates) {
    for (const suffix of path.extname(candidate) ? [''] : ['.exe', '.com', '.cmd', '.bat', '']) {
      const target = candidate + suffix;
      if (!existsSync(target) || !statSync(target).isFile()) continue;
      if (/\.(cmd|bat)$/i.test(target)) throw batchError();
      return target;
    }
  }
  return command;
}

async function terminateTree(child, signal, env) {
  if (!child.pid) return;
  if (process.platform === 'win32') {
    if (child.exitCode !== null || child.signalCode !== null) return;
    await execFileAsync(path.join(process.env.SystemRoot, 'System32', 'taskkill.exe'), ['/PID', String(child.pid), '/T', '/F'], { env, windowsHide: true });
  } else {
    try { process.kill(-child.pid, signal); }
    catch (error) { if (error.code === 'ESRCH') return; throw error; }
    await new Promise(resolve => setTimeout(resolve, 1000));
    try { process.kill(-child.pid, 'SIGKILL'); }
    catch (error) { if (error.code !== 'ESRCH') throw error; }
  }
}

async function execute(command, args, { cwd, env, signal, stdio = 'inherit' }) {
  if (signal?.aborted) return signal.reason === 'SIGINT' ? 130 : 143;
  const child = spawn(command, args, { cwd, env, shell: false, stdio, windowsHide: true, detached: process.platform !== 'win32' });
  let cancellation;
  const cancel = () => { cancellation ||= terminateTree(child, signal.reason === 'SIGINT' ? 'SIGINT' : 'SIGTERM', env); cancellation.catch(() => {}); };
  signal?.addEventListener('abort', cancel, { once: true });
  try {
    const result = await new Promise((resolve, reject) => {
      child.once('error', reject);
      child.once('close', (code, stoppedBy) => resolve({ code, stoppedBy }));
    });
    await cancellation;
    if (signal?.aborted) return signal.reason === 'SIGINT' ? 130 : 143;
    return result.code ?? 128 + (constants.signals[result.stoppedBy] || 1);
  } finally { signal?.removeEventListener('abort', cancel); }
}

export async function runInWorkspace(command, args, { root = repository, cwd = '.', env = process.env, signal } = {}) {
  if (!command || typeof command !== 'string' || !Array.isArray(args)) throw new Error(usage);
  const workspace = prepareWorkspace(root, env, cwd);
  command = executable(command, workspace.cwd, workspace.env);
  const isGo = /^go(?:\.exe)?$/i.test(path.basename(command));
  const goDirectory = args[0] === '-C' ? args[1] : args[0]?.startsWith('-C=') ? args[0].slice(3) : undefined;
  if (isGo && goDirectory) {
    // Go changes directory internally; validate relative configuration paths
    // against the directory in which the actual tool will run as well.
    prepareWorkspace(workspace.root, workspace.env, path.resolve(workspace.cwd, goDirectory), { check: true });
  }
  const goArgs = args[0] === '-C' ? args.slice(2) : args[0]?.startsWith('-C=') ? args.slice(1) : args;
  if (isGo && goArgs[0] === 'test') {
    const checker = path.join(workspace.root, 'scripts', 'check-test-paths');
    assertWorkspacePath(workspace.root, checker);
    if (!existsSync(checker)) throw new Error(`Go test path checker is missing: ${checker}`);
    const nativeEnv = { ...workspace.env };
    delete nativeEnv.GOOS; delete nativeEnv.GOARCH;
    const status = await execute(command, ['run', './scripts/check-test-paths', '-root', workspace.root], { ...workspace, cwd: workspace.root, env: nativeEnv, signal, stdio: ['inherit', 2, 2] });
    if (status !== 0) return status;
  }
  return execute(command, args, { ...workspace, signal });
}

async function main(args) {
  if (args.length === 1 && args[0] === '--check') { prepareWorkspace(repository, process.env, '.', { check: true }); return 0; }
  let cwd = '.';
  if (args[0] === '--cwd' && args[1]) { cwd = args[1]; args = args.slice(2); }
  if (args[0] !== '--' || !args[1]) throw new Error(usage);
  const abort = new AbortController();
  const interrupt = () => abort.abort('SIGINT');
  const terminate = () => abort.abort('SIGTERM');
  process.on('SIGINT', interrupt); process.on('SIGTERM', terminate);
  try { return await runInWorkspace(args[1], args.slice(2), { cwd, signal: abort.signal }); }
  finally { process.off('SIGINT', interrupt); process.off('SIGTERM', terminate); }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  main(process.argv.slice(2)).then(code => { process.exitCode = code; }).catch(error => { console.error(`Workspace runner: ${error.message}`); process.exitCode = 1; });
}
