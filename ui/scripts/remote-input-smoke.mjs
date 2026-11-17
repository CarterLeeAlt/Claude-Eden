// Start the UI dev server first. Uses the real noVNC keyboard and RFB encoder,
// with an in-memory socket; this does not replace native IME/guest acceptance.
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { installNativeFixture } from "./native-fixture.mjs";
const require = createRequire(import.meta.url);
const { chromium } = require(
  process.env.EDEN_PLAYWRIGHT_MODULE || "playwright",
);
const browser = await chromium.launch({ headless: true, executablePath: process.env.EDEN_TEST_BROWSER_PATH });
const page = await browser.newPage();
await installNativeFixture(page);
const errors = [];
page.on("pageerror", (error) => errors.push(error.message));
const origin = process.env.EDEN_UI_URL || "http://127.0.0.1:5173";
try {
  await page.route("**/remote-input-smoke", (route) =>
    route.fulfill({
      contentType: "text/html; charset=utf-8",
      body: '<div id="remote"><canvas tabindex="-1"></canvas></div><input aria-label="本机地址栏">',
    }),
  );
  await page.route("**/api/**", route => {
    if (new URL(route.request().url()).pathname === "/api/diagnostics/events") {
      errors.push(route.request().postDataJSON().message);
      return route.fulfill({ json: {} });
    }
    errors.push(`Unexpected fixture API request: ${route.request().url()}`);
    return route.abort();
  });
  await page.goto(origin + "/remote-input-smoke#isolated-keyboard-fixture");
  await page.evaluate(async () => {
    const fixtureTransport = window.edenRuntimeTransport;
    delete window.edenRuntimeTransport;
    const [{ default: RFB }, { default: Keyboard }, { installRemoteInput }] =
      await Promise.all([
        import("/.generated/novnc/core/rfb.js"),
        import("/.generated/novnc/core/input/keyboard.js"),
        import("/src/lib/remote-input.ts"),
      ]);
    const container = document.querySelector("#remote");
    const rfb = new EventTarget();
    // Use production methods through the final wire encoder, without opening
    // a guest connection or capturing any user clipboard content.
    for (const name of [
      "sendKey",
      "_workbenchSendKey",
      "_workbenchQueueInput",
      "_handleKeyEvent",
      "_handleServerFenceMsg",
      "workbenchSetKeyboardTarget",
      "focus",
      "blur",
    ])
      rfb[name] = RFB.prototype[name].bind(rfb);
    rfb._canvas = container.querySelector("canvas");
    rfb._keyboard = new Keyboard(rfb._canvas);
    rfb._rfbConnectionState = "connected";
    rfb._remoteCapsLock = rfb._remoteNumLock = null;
    const packets = [];
    let bytes = [];
    rfb._sock = {
      sQpush8(value) {
        bytes.push(value);
      },
      sQpush16(value) {
        bytes.push((value >> 8) & 255, value & 255);
      },
      sQpush32(value) {
        bytes.push(
          value >>> 24,
          (value >> 16) & 255,
          (value >> 8) & 255,
          value & 255,
        );
      },
      sQpushString(value) {
        bytes.push(...new TextEncoder().encode(value));
      },
      flush() {
        packets.push(bytes);
        bytes = [];
      },
    };
    const texts = [];
    rfb.workbenchPasteText = (text) =>
      rfb._workbenchQueueInput(() => {
        texts.push(text);
      });
    let active = true;
    const options = {
      mode: () => "mac",
      isActive: () => active,
      insertText: (text) => rfb.workbenchPasteText(text),
      onStatus() {},
      onFallback() {},
    };
    const install = () => installRemoteInput({
      events: rfb,
      setKeyboardTarget: target => rfb.workbenchSetKeyboardTarget(target),
      sendKey: (...key) => rfb.sendKey(...key),
      focus: () => rfb.focus(),
      pasteText: text => rfb.workbenchPasteText(text),
      runKeyAction: action => rfb._workbenchQueueInput(() => action((...key) => rfb._workbenchSendKey(...key))),
    }, container, options);
    window.probe = {
      rfb,
      packets,
      texts,
      compositions: [],
      input: install(),
      install,
      installComposition() {
        this.input.dispose();
        options.tabId = () => "tab-1";
        options.caretRect = async () => ({ available: true, context: "a".repeat(32), rect: [240, 120, 0, 24] });
        options.insertComposition = (id, value) => rfb._workbenchQueueInput(async () => {
          this.compositions.push({ id, ...value });
          if (value.mode === "commit") await new Promise(resolve => { this.finishComposition = resolve; });
        });
        const canvas = rfb._canvas;
        canvas.width = 1000; canvas.height = 500;
        canvas.style.cssText = "width:500px;height:250px;margin:40px";
        this.input = install();
        rfb.focus();
      },
      installNative() {
        this.input.dispose();
        this.compositions.length = 0;
        this.nativeRequests = [];
        window.edenNativeIMEAvailable = true;
        window.edenRuntimeTransport = fixtureTransport;
        const defaultRequest = window.edenTestNativeRequest;
        window.edenTestNativeRequest = async value => {
          if (value.action !== "remoteInput") return defaultRequest(value);
          this.nativeRequests.push(value); return { accepted: true, returnedFocus: false };
        };
        options.insertComposition = (id, value) => rfb._workbenchQueueInput(() => { this.compositions.push({ id, ...value }); });
        this.input = install(); rfb.focus();
      },
      setActive: (value) => {
        active = value;
      },
      fence(payload) {
        const bytes = [0, 0, 0, 0, 0, 0, 0, payload.length, ...new TextEncoder().encode(payload)];
        rfb._sock.rQwait = () => false;
        rfb._sock.rQskipBytes = (length) => bytes.splice(0, length);
        rfb._sock.rQshift32 = () => bytes.splice(0, 4).reduce((value, byte) => (value << 8) | byte, 0);
        rfb._sock.rQshift8 = () => bytes.shift();
        rfb._sock.rQshiftStr = (length) => String.fromCharCode(...bytes.splice(0, length));
        rfb._handleServerFenceMsg();
      },
    };
    rfb.focus();
  });
  const target = page.getByLabel("Claude Eden 网页输入");
  const keys = () =>
    page.evaluate(() =>
      probe.packets.filter(p => p[0] !== 248).map((p) => {
        if (p.length !== 8 || p[0] !== 4)
          throw new Error("Unexpected RFB key packet");
        return {
          down: p[1],
          sym: ((p[4] << 24) | (p[5] << 16) | (p[6] << 8) | p[7]) >>> 0,
        };
      }),
    );
  const texts = () => page.evaluate(() => [...probe.texts]);
  const clear = () =>
    page.evaluate(() => {
      probe.packets.length = 0;
      probe.texts.length = 0;
    });
  const dispatch = (type, detail) =>
    target.evaluate(
      (node, { type, detail }) => {
        const EventType = type.startsWith("composition")
          ? CompositionEvent
          : InputEvent;
        node.dispatchEvent(new EventType(type, { bubbles: true, ...detail }));
      },
      { type, detail },
    );
  const compose = async (text) => {
    await dispatch("compositionstart", { data: "" });
    await dispatch("input", {
      data: "nihao",
      isComposing: true,
      inputType: "insertCompositionText",
    });
    await dispatch("compositionend", { data: text });
    await dispatch("input", { data: text, inputType: "insertFromComposition" });
  };
  await page.keyboard.type("abc");
  assert.deepEqual(
    (await keys()).filter((k) => k.down).map((k) => k.sym),
    [97, 98, 99],
  );
  assert.equal(await target.inputValue(), "");
  await clear();
  await page.keyboard.press("Shift+Enter");
  assert.deepEqual((await keys()).map(({ sym, down }) => [sym, down]), [
    [0xffe1, 1], [0xff0d, 1], [0xff0d, 0], [0xffe1, 0],
  ], "Shift+Enter must preserve the modifier through the real RFB encoder");
  assert.deepEqual(await texts(), [], "newline must not also commit textarea text");
  await clear();
  await page.keyboard.down("Shift");
  await page.evaluate(() => {
    probe.rfb._workbenchPasteJob = { id: "punctuation", phase: "fence", timer: setTimeout(() => {}, 1000), resolve() {} };
    probe.fence("punctuation");
    probe.fence("punctuation-keys");
  });
  await page.keyboard.up("Shift");
  assert.deepEqual((await keys()).map(({ sym, down }) => [sym, down]), [
    [0xffe1, 1], [0xffe1, 0], [0xffe3, 1], [0x76, 1], [0x76, 0], [0xffe3, 0], [0xffe1, 1], [0xffe1, 0],
  ], "Desktop text commit releases held Shift for Ctrl+V and restores it before the host release");
  await clear();
  // Exercise Chromium's composition implementation as well as the explicit
  // cross-engine event sequences below. Native desktop IME needs separate acceptance.
  const cdp = await page.context().newCDPSession(page);
  for (const character of ["？", "？", "，", "。", "！"]) {
    await cdp.send("Input.dispatchKeyEvent", { type: "keyDown", key: character, code: "Slash", text: character });
    await cdp.send("Input.dispatchKeyEvent", { type: "keyUp", key: character, code: "Slash" });
  }
  assert.deepEqual(await texts(), ["？", "？", "，", "。", "！"]);
  assert.deepEqual(await keys(), [], "fullwidth punctuation must bypass X11 keysyms");
  await clear();
  await page.keyboard.press("Shift+?");
  assert.deepEqual(await texts(), ["?"]);
  assert.deepEqual((await keys()).map(({ sym }) => sym), [0xffe1, 0xffe1], "only Shift is sent as a physical key");
  await clear();
  await cdp.send("Input.imeSetComposition", {
    text: "nihao",
    selectionStart: 5,
    selectionEnd: 5,
  });
  assert.equal(await target.inputValue(), "nihao");
  assert.deepEqual(await keys(), []);
  await cdp.send("Input.insertText", { text: "你好" });
  assert.deepEqual(await texts(), ["你好"]);
  assert.deepEqual(await keys(), []);
  assert.equal(await target.inputValue(), "");
  await clear();
  await cdp.send("Input.insertText", { text: "🙂" });
  assert.deepEqual(await texts(), ["🙂"]);
  await clear();
  await cdp.detach();
  await dispatch("compositionstart", { data: "" });
  const accepted = await target.evaluate((node) =>
    node.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "Process",
        code: "KeyN",
        keyCode: 229,
        bubbles: true,
        cancelable: true,
        isComposing: true,
      }),
    ),
  );
  assert.equal(
    accepted,
    true,
    "host IME key default action must remain enabled",
  );
  await page.keyboard.press("Space");
  assert.deepEqual(
    await keys(),
    [],
    "candidate selection must not reach the guest",
  );
  await dispatch("compositionend", { data: "你好🙂" });
  await dispatch("input", {
    data: "你好🙂",
    inputType: "insertFromComposition",
  });
  await page.keyboard.press("Enter");
  assert.deepEqual(
    (await keys()).filter((k) => k.down).map((k) => k.sym),
    [0xff0d],
  );
  assert.equal(await target.inputValue(), "");
  await clear();
  await compose("你");
  await compose("你");
  assert.deepEqual(await texts(), ["你", "你"]);
  await clear();
  // A delayed commit must keep subsequent Enter behind text completion.
  await page.evaluate(() => {
    probe.rfb._workbenchQueueInput(
      () =>
        new Promise((resolve) => {
          probe.finishText = resolve;
        }),
    );
  });
  await page.keyboard.press("Enter");
  assert.deepEqual(await keys(), []);
  await page.evaluate(() => probe.finishText());
  assert.deepEqual(
    (await keys()).map((k) => k.sym),
    [0xff0d, 0xff0d],
  );
  await clear();
  // On transfer failure, do not submit a form with the committed text missing.
  await page.evaluate(() => {
    probe.rfb._workbenchQueueInput(
      () =>
        new Promise((resolve, reject) => {
          probe.failText = reject;
        }),
    );
  });
  await page.keyboard.press("Enter");
  await page.evaluate(() => probe.failText(new Error("test transfer failure")));
  assert.deepEqual(await keys(), []);
  await compose("");
  assert.deepEqual(await keys(), []);
  await page.keyboard.down("Shift");
  await dispatch("compositionstart", { data: "" });
  await page.keyboard.up("Shift");
  assert.deepEqual(
    (await keys()).map((k) => k.down),
    [1, 0],
    "modifiers released during composition must not stick",
  );
  assert.equal(
    await page.evaluate(() => probe.input.edit("undo")),
    false,
    "native menus must edit the host composition",
  );
  await dispatch("compositionend", { data: "" });
  await clear();
  await page.getByLabel("本机地址栏").fill("本机文字");
  assert.deepEqual(await keys(), []);
  await page.evaluate(() => probe.rfb.focus());
  await page.keyboard.press("Meta+f");
  assert.deepEqual(
    (await keys()).filter((k) => k.down).map((k) => k.sym),
    [0xffe3, 102],
  );
  await clear();
  await page.evaluate(() => probe.setActive(false));
  await compose("后台不提交");
  assert.deepEqual(await keys(), []);
  await page.evaluate(() => {
    probe.setActive(true);
    probe.input.dispose();
    probe.input = probe.install();
    probe.rfb.focus();
  });
  assert.equal(await target.count(), 1);
  await page.keyboard.press("b");
  assert.deepEqual(
    (await keys()).filter((k) => k.down).map((k) => k.sym),
    [98],
  );
  await clear();
  await page.evaluate(() => probe.installComposition());
  await page.waitForFunction(() => {
    const box = probe.rfb._canvas.getBoundingClientRect();
    return document.activeElement.style.left === `${box.left + 120}px`;
  });
  const nativeCDP = await page.context().newCDPSession(page);
  await nativeCDP.send("Input.imeSetComposition", { text: "ni'hao", selectionStart: 6, selectionEnd: 6 });
  await nativeCDP.send("Input.insertText", { text: "你好？" });
  await page.waitForFunction(() => !!probe.finishComposition);
  await page.keyboard.press("Enter");
  assert.deepEqual(await keys(), [], "Enter must wait behind the native composition commit");
  const nativeMessages = await page.evaluate(() => probe.compositions);
  assert.equal(nativeMessages[0].mode, "begin");
  assert.equal(nativeMessages.at(-1).mode, "commit");
  assert.ok(nativeMessages.slice(1, -1).every(({ mode }) => mode === "update"));
  assert.ok(nativeMessages.some(({ text }) => text === "ni'hao"));
  assert.equal(nativeMessages.at(-1).text, "你好？");
  assert.equal(new Set(nativeMessages.map(({ session }) => session)).size, 1);
  await page.evaluate(() => probe.finishComposition());
  assert.deepEqual((await keys()).map(({ sym }) => sym), [0xff0d, 0xff0d]);
  await nativeCDP.detach();
  await clear();
  await page.evaluate(() => probe.installNative());
  await page.waitForFunction(() => probe.nativeRequests.some(value => value.operation === "activate"));
  await page.evaluate(() => {
    const owner = probe.nativeRequests.find(value => value.operation === "activate").owner;
    for (const kind of ["keydown", "keyup"]) {
      window.emitTestDesktopEvent("eden-native-input", { owner, kind, key: "Enter", code: "Enter", shiftKey: true });
    }
  });
  assert.deepEqual((await keys()).map(({ sym, down }) => [sym, down]), [
    [0xffe1, 1], [0xff0d, 1], [0xff0d, 0], [0xffe1, 0],
  ], "native Enter's shiftKey must survive without a preceding DOM Shift event");
  await clear();
  await page.evaluate(() => {
    const owner = probe.nativeRequests.find(value => value.operation === "activate").owner;
    const emit = value => window.emitTestDesktopEvent("eden-native-input", { owner, ...value });
    emit({kind:"compositionstart"});
    emit({kind:"compositionupdate",text:"ni'hao",selectionStart:3,selectionEnd:6});
    emit({kind:"compositionend",text:"你好"});
    emit({kind:"input",text:"？"});
    emit({kind:"keydown",key:"?",code:"Slash",textKey:true});
    emit({kind:"keyup",key:"?",code:"Slash"});
    window.emitTestDesktopEvent("eden-native-input", {owner:"stale",kind:"input",text:"误写"});
  });
  assert.deepEqual((await page.evaluate(() => probe.compositions)).map(({mode})=>mode), ["begin","update","commit"]);
  assert.deepEqual(await texts(), ["？"], "native direct punctuation is committed exactly once");
  assert.deepEqual((await keys()).map(({sym})=>sym), [63,63], "native ASCII punctuation keeps the key path without needing a DOM default action");
  await page.getByLabel("本机地址栏").focus();
  await page.waitForFunction(() => probe.nativeRequests.some(value => value.operation === "deactivate"));
  await clear();
  await page.evaluate(() => {
    const oldTarget = document.activeElement;
    probe.input.dispose();
    oldTarget.dispatchEvent(
      new CompositionEvent("compositionend", { data: "迟到的文字" }),
    );
    probe.rfb.focus();
  });
  assert.equal(await target.count(), 0);
  assert.equal(
    await page.evaluate(() => document.activeElement.tagName),
    "CANVAS",
  );
  assert.deepEqual(await keys(), []);
  const suspendedInput = await page.evaluate(async () => {
    const { default: RFB } = await import('/.generated/novnc/core/rfb.js');
    const check = (condition, message) => { if (!condition) throw new Error(message); };
    const result = promise => promise.then(() => 'resolved', () => 'rejected');
    const drain = async () => { await Promise.resolve(); await Promise.resolve(); };
    const u32 = value => [value >>> 24, value >>> 16 & 255, value >>> 8 & 255, value & 255];
    const client = Object.create(RFB.prototype), packets = [];
    let outgoing = [], incoming = [];
    client._sock = {
      sQpush8: value => outgoing.push(value),
      sQpush16: value => outgoing.push(value >>> 8 & 255, value & 255),
      sQpush32: value => outgoing.push(...u32(value)),
      sQpushString: value => outgoing.push(...new TextEncoder().encode(value)),
      sQpushBytes: value => outgoing.push(...value),
      flush: () => { packets.push(outgoing); outgoing = []; },
      workbenchControl() {},
      rQwait: (_label, count) => incoming.length < count,
      rQskipBytes: count => incoming.splice(0, count),
      rQshift32: () => incoming.splice(0, 4).reduce((value, byte) => (value * 256 + byte) >>> 0, 0),
      rQshift8: () => incoming.shift(),
      rQshiftStr: count => String.fromCharCode(...incoming.splice(0, count)),
    };
    Object.assign(client, { _rfbConnectionState: 'connected', _viewOnly: false, _supportsFence: true,
      _workbenchDisplayVisible: true, _enabledContinuousUpdates: false, _fbWidth: 640, _fbHeight: 480,
      _clipboardServerCapabilitiesActions: { [1 << 27]: true, [1 << 28]: true },
      _clipboardServerCapabilitiesFormats: { 1: true } });
    const acknowledge = id => {
      incoming = [0, 0, 0, ...u32(0), id.length, ...new TextEncoder().encode(id)];
      check(client._handleServerFenceMsg(), 'late input fence was not consumed');
    };
    const requestClipboard = () => {
      incoming = [0, 0, 0, ...u32(-4), ...u32((1 << 25) | 1)];
      check(client._handleServerCutText(), 'clipboard request was not consumed');
    };
    let committed = 0;
    const waiting = result(client.workbenchRunInput(() => committed++));
    const queued = result(client.workbenchRunInput(() => committed++));
    client.sendKey(0xff0d, 'Enter', true);
    await drain();
    const first = client._workbenchInputFence.id;
    client.workbenchSetVisible(false); client.workbenchSetVisible(true);
    acknowledge(first);
    check(await waiting === 'rejected' && await queued === 'rejected', 'hidden queue was replayed after return');
    check(committed === 0 && packets.filter(value => value[0] === 4).length === 0, 'hidden input reached the wire');

    const acknowledged = result(client.workbenchRunInput(() => committed++));
    await drain(); acknowledge(client._workbenchInputFence.id); client.workbenchSetVisible(false);
    check(await acknowledged === 'rejected' && committed === 0, 'acknowledged input escaped the visibility generation');
    // Cleanup from the new generation is allowed while the viewer is hidden.
    const cleanup = result(client.workbenchRunInput(() => committed++));
    await drain(); acknowledge(client._workbenchInputFence.id);
    check(await cleanup === 'resolved' && committed === 1, 'composition cleanup was blocked by the cancelled queue');
    client.workbenchSetVisible(true);

    for (const content of [{ text: 'delayed text' }, { png: 'delayed image' }, { files: ['delayed file'] }]) {
      let ready;
      const waitingContent = new Promise(resolve => { ready = resolve; });
      const paste = result(client.workbenchPasteContent(waitingContent, () => committed++, () => true, () => committed++));
      await drain(); client.workbenchSetVisible(false); client.workbenchSetVisible(true); ready(content);
      check(await paste === 'rejected' && committed === 1, 'asynchronous clipboard content survived suspension');
    }
    for (const phase of ['data', 'fence', 'keys', 'complete']) {
      const count = packets.filter(value => value[0] === 4).length;
      const paste = result(client.workbenchPasteText('clipboard ' + phase));
      await drain();
      const id = client._workbenchPasteJob.id;
      if (phase !== 'data') requestClipboard();
      if (phase === 'keys' || phase === 'complete') acknowledge(id);
      if (phase === 'complete') acknowledge(id + '-keys');
      else {
        client.workbenchSetVisible(false); client.workbenchSetVisible(true);
        if (phase === 'data') requestClipboard();
        acknowledge(id); acknowledge(id + '-keys');
      }
      check(await paste === (phase === 'complete' ? 'resolved' : 'rejected'), 'clipboard completion state changed at ' + phase);
      const sent = packets.filter(value => value[0] === 4).length - count;
      check(sent === (phase === 'keys' || phase === 'complete' ? 4 : 0), 'late clipboard acknowledgement emitted paste keys at ' + phase);
      check(client._workbenchPasteJob === null, 'paste timer retained after ' + phase);
    }
    check(client._rfbConnectionState === 'connected', 'input cancellation disconnected the retained display');
    return { queued_input: true, resolved_fence: true, asynchronous_clipboard: true, paste_phases: true, cleanup_and_resume: true };
  });
  assert.deepEqual(suspendedInput, { queued_input: true, resolved_fence: true, asynchronous_clipboard: true, paste_phases: true, cleanup_and_resume: true });
  assert.deepEqual(errors, []);
  console.log(
    JSON.stringify(
      {
        passed: true,
        checks: [
          "real noVNC keyboard and RFB encoding",
          "Shift+Enter and native modifier recovery",
          "English input",
          "fullwidth question mark and punctuation",
          "English question mark and modifier release",
          "Desktop paste while Shift is held",
          "shared composition begin, preedit and commit order",
          "candidate anchor maps the actual canvas bounds",
          "native host composition, punctuation and stale owner rejection",
          "native focus returns to local inputs",
          "Chromium IME engine commit",
          "composition default action",
          "candidate suppression",
          "Unicode text transfer",
          "text and Enter order",
          "failed transfer cancels queued Enter",
          "repeated commits",
          "cancel",
          "modifier release",
          "native edit routing",
          "local field isolation",
          "Command mapping",
          "inactive view",
          "reconnect and disposal",
          "retained hidden input cancellation, late acknowledgements and resumed input",
        ],
        limitation:
          "Covers the Chromium composition engine and simulated event sequences; native Windows IME and guest rendering require separate acceptance.",
      },
      null,
      2,
    ),
  );
} finally {
  await browser.close();
}
