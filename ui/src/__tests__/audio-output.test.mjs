import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";
import { setImmediate as nextTurn } from "node:timers/promises";
import { audioFormatSchema } from "../lib/contracts.ts";
import { transpileSource } from "./source-module.mjs";

const source = transpileSource(
  readFileSync(new URL("../lib/audio-output.ts", import.meta.url), "utf8"),
)
  .replace(/^import .*;\n/gm, "")
  .replace("export class AudioOutput", "class AudioOutput");
const format = {
  ready: true,
  format: "s16le",
  sample_rate: 48000,
  channels: 2,
};
function fixture(api = async () => format) {
  const sockets = [],
    errors = [],
    states = [],
    retries = new Set();
  const AudioOutput = vm.runInNewContext(source + "\nAudioOutput", {
    AbortController,
    audioFormatSchema,
    api,
    errorMessage: (error) => error.message,
    runtimeSocketURL: (path) => `ws://fixture${path}`,
    setTimeout: (callback) => {
      retries.add(callback);
      return callback;
    },
    clearTimeout: (callback) => retries.delete(callback),
    WebSocket: class {
      constructor(url) {
        this.url = url;
        sockets.push(this);
      }
      close() {
        this.closed = true;
        this.onclose?.();
      }
    },
  });
  const output = new AudioOutput(
    (state) => states.push(state),
    (error) => errors.push(error),
  );
  output.active = true;
  output.context = {
    state: "running",
    suspend: async () => {},
    close: async () => {},
  };
  output.prepare = async () => {};
  return { output, sockets, errors, states, retries };
}
test("a disposed audio preparation never requests a format or opens a socket", async () => {
  const f = fixture(() => assert.fail("disposed output made an HTTP request"));
  let release;
  f.output.prepare = () =>
    new Promise((resolve) => {
      release = resolve;
    });
  const connecting = f.output.connect();
  f.output.close();
  release();
  await connecting;
  assert.equal(f.sockets.length, 0);
  assert.equal(f.retries.size, 0);
  assert.deepEqual(f.errors, []);
});
test("mute and close abort pending audio HTTP without errors or stale reconnection", async () => {
  for (const cancel of [
    (output) => output.setActive(false),
    (output) => output.close(),
  ]) {
    let request;
    const f = fixture(async (_path, _body, _method, _headers, options) => {
      request = options.signal;
      return new Promise((_, reject) =>
        request.addEventListener("abort", () => reject(request.reason), {
          once: true,
        }),
      );
    });
    const connecting = f.output.connect();
    await nextTurn();
    assert.ok(request);
    cancel(f.output);
    await connecting;
    assert.equal(request.aborted, true);
    assert.equal(f.output.request, null);
    assert.equal(f.sockets.length, 0);
    assert.equal(f.retries.size, 0);
    assert.ok(f.errors.every((message) => message === ""));
  }
});
test("a late audio format from an old generation cannot replace resumed playback", async () => {
  const replies = [];
  const f = fixture(() => new Promise((resolve) => replies.push(resolve)));
  const old = f.output.connect();
  await nextTurn();
  f.output.disconnect();
  const resumed = f.output.connect();
  await nextTurn();
  replies[1](format);
  await resumed;
  const socket = f.output.socket;
  replies[0](format);
  await old;
  assert.equal(f.sockets.length, 1);
  assert.equal(f.output.socket, socket);
  f.output.close();
});
test("audio failures remain visible and valid headers gate binary playback", async () => {
  const failed = fixture(async () => {
    throw new Error("guest audio unavailable");
  });
  await failed.output.connect();
  assert.deepEqual(failed.errors, ["guest audio unavailable"]);
  assert.equal(failed.retries.size, 1);
  failed.output.close();
  assert.equal(failed.retries.size, 0);

  for (const header of [
    "invalid JSON",
    JSON.stringify({ ...format, channels: "2" }),
    JSON.stringify({ ...format, channels: 1 }),
    JSON.stringify(format),
  ]) {
    const f = fixture();
    const frames = [];
    f.output.node = {
      port: { postMessage: (data) => frames.push(data) },
      disconnect() {},
    };
    await f.output.connect();
    const socket = f.sockets[0];
    socket.onmessage({ data: new ArrayBuffer(4) });
    assert.equal(frames.length, 0, "unconfirmed PCM must not play");
    socket.onmessage({ data: header });
    if (header === JSON.stringify(format)) {
      socket.onmessage({ data: new ArrayBuffer(4) });
      assert.equal(frames.length, 1);
      assert.equal(f.states.at(-1), "playing");
    } else {
      assert.equal(socket.closed, true);
      assert.equal(f.retries.size, 1);
    }
    f.output.close();
  }
});
