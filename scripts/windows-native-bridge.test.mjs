import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
import vm from 'node:vm';
import { isNativeError } from 'node:util/types';
import { tsImport } from '../ui/node_modules/tsx/dist/esm/api/index.mjs';

const source = await readFile(new URL('../internal/desktop/transport_windows.go', import.meta.url), 'utf8');
const script = source.match(/const RuntimeTransportClient = `([\s\S]*?)`/)?.[1];
assert.ok(script);
const smokeSource = await readFile(new URL('../internal/desktop/smoke_diagnostics_windows.go', import.meta.url), 'utf8');
const smokeScript = smokeSource.match(/const smokeDiagnosticsScript = `([\s\S]*?)`/)?.[1];
assert.ok(smokeScript);

function transport({ protocol = 'http:', hostname = 'localhost', frame = false, webview = true } = {}) {
  const sent = [];
  let sendError;
  let ready = 0;
  const window = new EventTarget();
  window.top = frame ? {} : window;
  window.addEventListener('eden-runtime-transport-ready', () => ready++);
  if (webview) window.chrome = { webview: { postMessage(value) {
    if (sendError) throw sendError;
    assert.ok(value.startsWith('eden-runtime-v1:'));
    sent.push(JSON.parse(value.slice('eden-runtime-v1:'.length)));
  } } };
  const context = vm.createContext({
    window, location: { protocol, hostname }, crypto: { randomUUID },
    chrome: window.chrome, Event,
    addEventListener: window.addEventListener.bind(window), dispatchEvent: window.dispatchEvent.bind(window),
    setTimeout() { assert.fail('the custom transport must not impose a startup/operation-wide deadline'); },
  });
  vm.runInContext(script, context);
  return {
    window, sent,
    call: request => window.edenRuntimeTransport.call(0, 0, '', request),
    request: () => sent.at(-1).request,
    reply(value, id = sent.at(-1).id) { window.dispatchEvent(new CustomEvent('eden-runtime-result', { detail: { id, ...value } })); },
    failSend(value) { sendError = value; },
    reinstall() { vm.runInContext(script, context); },
    get ready() { return ready; },
  };
}

async function sentRequest(fixture, count) {
  const deadline = Date.now() + 5000;
  while (fixture.sent.length < count && Date.now() < deadline) await new Promise(resolve => setImmediate(resolve));
  assert.equal(fixture.sent.length, count, 'the real frontend dispatched the expected request');
  return fixture.sent.at(-1);
}

test('the real Windows custom transport has no general deadline for any generated binding', async () => {
  const bindings = await readFile(new URL('../ui/.generated/bindings/github.com/sylvinus/agent-vm/internal/desktop/desktopservice.ts', import.meta.url), 'utf8');
  const methods = [...bindings.matchAll(/export function (\w+)\([^]*?\$Call\.ByID\((\d+)/g)];
  assert.equal(methods.length, 27);
  assert.deepEqual(methods.map(([,name])=>name).sort(), [
    'AccountDevice','AppReference','AppUpdate','Appearance','BrowserDownloadAction','ChooseFolder','ChooseUploadFiles',
    'ClipboardRead','ClipboardReadContent','ClipboardRevision','ClipboardSyncCancel','ClipboardSyncRead','ClipboardSyncRelease',
    'ClipboardSyncWrite','ClipboardWrite','DesktopFileAction','ExportDiagnostics','ExportTheme','ImportTheme','QuitNow','Reconnect',
    'RefreshSession','RemoteInput','SaveRecoveryKey','UploadHostFiles','WorkbenchFailed','WorkbenchReady',
  ]);
  for (const [, name, id] of methods) {
    const fixture = transport();
    const args = { methodID: Number(id), args: [] };
    const pending = fixture.call(args);
    // A turn without a reply leaves the call pending, rather than starting a
    // transport timer that can misclassify slow startup or session recovery.
    let settled = false;
    pending.then(() => { settled = true; });
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(settled, false, name);
    assert.deepEqual(fixture.request(), { object: 0, method: 0, args });
    fixture.reply({ result: { saved: true } });
    assert.equal((await pending).saved, true, name);
  }
});

test('the injected Windows transport is usable by the real generated frontend readiness binding', async t => {
  const fixture = transport();
  const previous = globalThis.window;
  globalThis.window = fixture.window;
  t.after(() => { if (previous === undefined) delete globalThis.window; else globalThis.window = previous; });
  const { native, nativeAvailable } = await tsImport('../ui/src/lib/api.ts', import.meta.url);
  assert.equal(nativeAvailable(), true);
  const pending = native('workbenchReady');
  await sentRequest(fixture, 1);
  assert.equal(fixture.request().args.methodID, 1132524461);
  fixture.reply({ result: true });
  assert.equal(await pending, true);
});

test('typed failures retain their cause and do not poison subsequent calls', async () => {
  const fixture = transport();
  for (const kind of ['ReferenceError', 'TypeError', 'RuntimeError']) {
    const pending = fixture.call({ methodID: 3519489769, args: [] });
    fixture.reply({ error: { kind, message: '磁盘空间不足', cause: 'fixture cause' } });
    await assert.rejects(pending, error => error.name === kind && error.message === '磁盘空间不足' && error.cause === 'fixture cause');
  }
  const second = fixture.call({ methodID: 3282569462, args: [] });
  fixture.reply({ result: { revision: 1, text: 'next copy' } });
  assert.equal((await second).text, 'next copy');
});

test('synchronous postMessage failures reject immediately and late replies are ignored', async () => {
  const fixture = transport();
  const failure = new Error('IPC fixture failure');
  fixture.failSend(failure);
  await assert.rejects(fixture.call({ methodID: 1891984550, args: [] }), error => error === failure);
  fixture.failSend(undefined);
  const pending = fixture.call({ methodID: 1891984550, args: [] });
  fixture.reply({ result: true }, randomUUID());
  fixture.reply({ result: true });
  assert.equal(await pending, true);
  fixture.reply({ error: { message: 'duplicate reply must be ignored' } });
});

test('transport installation is idempotent and rejects non-workbench contexts', () => {
  const fixture = transport();
  const installed = fixture.window.edenRuntimeTransport;
  fixture.reinstall();
  assert.equal(fixture.window.edenRuntimeTransport, installed);
  assert.equal(fixture.ready, 2);
  for (const context of [{ protocol: 'https:' }, { hostname: '127.0.0.1' }, { frame: true }, { webview: false }]) {
    const blocked = transport(context);
    assert.equal(blocked.window.edenRuntimeTransport, undefined);
    assert.equal(blocked.ready, 0);
  }
});

test('real npm runtime and generated binding preserve errors, cancellation IDs and late reply semantics', async t => {
  const fixture = transport();
  const previous = globalThis.window;
  globalThis.window = fixture.window;
  t.after(() => { if (previous === undefined) delete globalThis.window; else globalThis.window = previous; });
  const { desktopService } = await tsImport('../ui/src/lib/desktop-runtime.ts', import.meta.url);
  const bindings = await desktopService();

  const { Call, setTransport } = await import('../ui/node_modules/@wailsio/runtime/dist/index.js');
  setTransport(fixture.window.edenRuntimeTransport);
  const unknown = Call.ByID(0);
  assert.equal(fixture.request().args.methodID, 0);
  fixture.reply({ error: { kind: 'RuntimeError', message: 'Binding call failed: unknown bound method id 0' } });
  await assert.rejects(unknown, error => isNativeError(error) && error.name === 'RuntimeError' && error.message === 'Binding call failed: unknown bound method id 0');
  const afterUnknown = bindings.AccountDevice('status');
  assert.notEqual(fixture.request().args.methodID, 0);
  fixture.reply({ result: { supported: false } });
  assert.equal((await afterUnknown).supported, false);

  const failed = bindings.WorkbenchReady();
  fixture.reply({ error: { kind: 'RuntimeError', message: 'fixture backend failure' } });
  await assert.rejects(failed, /fixture backend failure/);

  const startupFailure = bindings.WorkbenchFailed('controlled startup failure');
  assert.equal(fixture.request().args.methodID, 3446012975);
  assert.deepEqual(Array.from(fixture.request().args.args), ['controlled startup failure']);
  fixture.reply({result:true});
  assert.equal(await startupFailure,true);
  const lateFailure = bindings.WorkbenchFailed('already confirmed');
  fixture.reply({result:false});
  assert.equal(await lateFailure,false);

  for (const reply of [{ result: true }, { error: { kind: 'RuntimeError', message: 'context canceled' } }]) {
    const pending = bindings.RefreshSession();
    const original = fixture.sent.at(-1);
    assert.equal(original.request.object, 0);
    assert.equal(original.request.args.methodID, 3123219584);
    const peer = bindings.WorkbenchReady();
    const peerRequest = fixture.sent.at(-1);
    const peerRejected = assert.rejects(peer, /uncancelled peer error/);
    const rejected = assert.rejects(pending, error => error.name === 'CancelError');
    await pending.cancel();
    const cancellation = fixture.sent.at(-1);
    assert.equal(cancellation.request.object, 10);
    assert.equal(cancellation.request.method, 0);
    assert.equal(cancellation.request.args['call-id'], original.request.args['call-id']);
    fixture.reply({ result: {} }, cancellation.id);
    fixture.reply(reply, original.id);
    fixture.reply({ error: { kind: 'RuntimeError', message: 'uncancelled peer error' } }, peerRequest.id);
    await rejected;
    await peerRejected;
    await new Promise(resolve => setImmediate(resolve));
  }

  const sendFailure = new Error('generated binding IPC failure');
  fixture.failSend(sendFailure);
  await assert.rejects(bindings.WorkbenchReady(), error => error === sendFailure);
  fixture.failSend(undefined);
  const next = bindings.WorkbenchReady();
  fixture.reply({ result: true });
  assert.equal(await next, true);
});

function smokePage({ protocol = 'http:', hostname = 'localhost', frame = false, webview = true } = {}) {
  const sent = [], tasks = [], listeners = new Map();
  const window = {};
  window.top = frame ? {} : window;
  if (webview) window.chrome = { webview: { postMessage(value) {
    assert.ok(value.startsWith('eden-smoke-diagnostics:'));
    sent.push(JSON.parse(value.slice('eden-smoke-diagnostics:'.length)));
  } } };
  class HTMLScriptElement { constructor(src) { this.src = src; } }
  class HTMLLinkElement { constructor(href) { this.href = href; } }
  const context = vm.createContext({
    window, chrome: window.chrome, crypto: { randomUUID }, URL, HTMLScriptElement, HTMLLinkElement,
    location: { protocol, hostname, href: `${protocol}//${hostname}:7654/#private-token`, origin: `${protocol}//${hostname}:7654` },
    document: { querySelectorAll: () => [{ value: 'private-form-value' }] },
    addEventListener: (name, listener) => listeners.set(name, [...listeners.get(name) || [], listener]),
    setTimeout: callback => tasks.push(callback),
  });
  vm.runInContext(smokeScript, context);
  return {
    window, sent, tasks, HTMLScriptElement, HTMLLinkElement,
    emit: (name, event) => { for (const listener of listeners.get(name) || []) listener(event); },
    runTurn: () => { const current = tasks.splice(0); for (const task of current) task(); },
    reinstall: () => vm.runInContext(smokeScript, context),
  };
}

test('the actual document-start collector retains early errors, module failures and post-readiness rejections', () => {
  const fixture = smokePage();
  fixture.emit('error', {
    message: 'early failure private-form-value https://external.example/private?token=secret',
    filename: 'http://localhost:7654/assets/app.js?token=secret',
    error: { stack: 'Error: private-form-value\n at own (http://localhost:7654/assets/app.js?token=secret:1)\n at external (https://external.example/private)' },
  });
  fixture.emit('error', { target: new fixture.HTMLScriptElement('http://localhost:7654/assets/missing.js?token=secret') });
  fixture.emit('error', { target: new fixture.HTMLLinkElement('http://localhost:7654/assets/missing.css') });
  fixture.emit('error', { target: { src: '/optional-avatar.png' } });
  const snapshot = randomUUID();
  fixture.window.__edenSmokeSnapshot(snapshot);
  assert.equal(fixture.sent.length, 3, 'snapshot waits for readiness reactions rather than accepting the call as proof of a clean page');
  fixture.runTurn();
  fixture.emit('unhandledrejection', { reason: { message: 'post-ready private-form-value', stack: 'Error\n at own (http://localhost:7654/assets/app.js:2)' } });
  fixture.emit('unhandledrejection', { reason: { private: 'raw rejection object must not be exported' } });
  fixture.runTurn();
  const result = fixture.sent.at(-1);
  assert.equal(result.snapshot, snapshot);
  assert.deepEqual(result.errors.map(error => error.type), ['error', 'module-load', 'module-load', 'unhandledrejection', 'unhandledrejection']);
  assert.equal(result.errors[0].source, '/assets/app.js');
  assert.equal(result.errors[1].source, '/assets/missing.js');
  assert.equal(result.errors[0].stack.length, 1);
  assert.equal(result.errors[4].message, 'Unhandled rejection');
  const exported = JSON.stringify(result);
  for (const privateValue of ['private-form-value', 'external.example', 'token=secret', '#private-token', 'raw rejection object']) {
    assert.ok(!exported.includes(privateValue), privateValue);
  }
  fixture.reinstall();
  for (let index = 0; index < 10; index++) fixture.emit('error', { message: 'x'.repeat(1000), filename: '/assets/app.js' });
  assert.equal(fixture.sent.at(-1).errors.length, 8);
  assert.equal(fixture.sent.at(-1).errors.at(-1).message.length, 768);
});

test('the document-start collector acknowledges clean pages and is absent from foreign origins and frames', () => {
  const fixture = smokePage();
  fixture.window.__edenSmokeSnapshot('clean');
  fixture.runTurn();
  fixture.runTurn();
  assert.equal(fixture.sent.at(-1).snapshot, 'clean');
  assert.deepEqual(fixture.sent.at(-1).errors, []);
  for (const context of [{ protocol: 'https:' }, { hostname: '127.0.0.1' }, { frame: true }, { webview: false }]) {
    const blocked = smokePage(context);
    assert.equal(blocked.window.__edenSmokeSnapshot, undefined);
    assert.equal(blocked.sent.length, 0);
  }
});

test('the startup diagnostic barrier acknowledges the complete buffer without a timer', () => {
  const fixture = smokePage();
  fixture.emit('error',{message:'controlled earlier failure',filename:'/assets/entry.js'});
  fixture.window.__edenStartupSnapshot('startup-barrier');
  assert.equal(fixture.sent.at(-1).snapshot,'startup-barrier');
  assert.equal(fixture.sent.at(-1).errors.length,1);
  assert.equal(fixture.tasks.length,0);
});
