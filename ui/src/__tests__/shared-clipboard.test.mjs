import test from "node:test";
import assert from "node:assert/strict";
import {
  clipboardReconnectDelay,
  shareClipboard,
} from "../lib/shared-clipboard.ts";
import { native } from "../lib/api.ts";
import { createNativeTransport } from "./native-runtime.mjs";

const settle = () => new Promise((resolve) => setImmediate(resolve));

function fixture({ available = true, native: invoke } = {}) {
  const sent = [],
    writes = [],
    active = [],
    notices = [],
    released = [];
  let revision = 1,
    content = { text: "host initial" },
    pauseRead,
    pauseWrite,
    nativeGeneration = 0,
    delayAcks = false;
  const pendingAcks = [];
  const socket = {
    readyState: 1,
    send: (text) => {
      const value = JSON.parse(text);
      sent.push(
        Object.fromEntries(
          Object.entries(value).filter(([key]) => key !== "serial"),
        ),
      );
      if (delayAcks) pendingAcks.push(value.serial);
      else
        queueMicrotask(() =>
          socket.onmessage({ data: JSON.stringify({ ack: value.serial }) }),
        );
    },
    close() {
      this.readyState = 3;
      this.onclose?.();
    },
  };
  const controller = shareClipboard({
    connect: () => socket,
    interval: 60000,
    onStatus: (text) => notices.push(text),
    setActive: (value) => active.push(value),
    native:
      invoke ??
      (async (action, value) => {
        if (action === "clipboardRevision") return revision;
        if (action === "clipboardSyncRead") {
          const snapshot = { ...content, revision };
          if (pauseRead) await pauseRead;
          return snapshot;
        }
        if (action === "clipboardSyncCancel") {
          nativeGeneration++;
          return;
        }
        if (action === "clipboardSyncRelease") {
          released.push(value.files);
          return;
        }
        if (action === "clipboardSyncWrite") {
          const generation = nativeGeneration;
          if (pauseWrite) await pauseWrite;
          if (generation !== nativeGeneration || value.revision !== revision)
            return { superseded: true, revision };
          content = { ...value };
          delete content.revision;
          writes.push(content);
          return { revision: ++revision };
        }
      }),
  });
  socket.onopen();
  socket.onmessage({
    data: JSON.stringify({
      hello: {
        version: 1,
        os: "linux",
        session: "fixture-session",
        formats: ["text", "png", "files"],
        available,
      },
    }),
  });
  return {
    socket,
    controller,
    sent,
    writes,
    active,
    notices,
    released,
    host: (value) => {
      content = value;
      revision++;
    },
    guest: (value) => socket.onmessage({ data: JSON.stringify(value) }),
    readDelay: (value) => {
      pauseRead = value;
    },
    writeDelay: (value) => {
      pauseWrite = value;
    },
    delayAcks: () => {
      delayAcks = true;
    },
    acknowledge: () => {
      assert.ok(pendingAcks.length, "a guest acknowledgement is pending");
      socket.onmessage({ data: JSON.stringify({ ack: pendingAcks.shift() }) });
    },
  };
}

for (const transition of ["new-session", "replacement-instance"]) {
  test(`late native cancellation cannot invalidate a ${transition} clipboard read or file write`, async (t) => {
    const originalWindow = globalThis.window;
    t.after(() => {
      if (originalWindow === undefined) delete globalThis.window;
      else globalThis.window = originalWindow;
    });
    let revision = 1,
      content = { text: "initial" },
      delayedRead,
      finishRead,
      finishCancel;
    let pauseCancel = false,
      nativeGeneration = 0;
    const calls = [],
      released = [],
      written = [];
    globalThis.window = {
      edenRuntimeTransport: createNativeTransport(async (request) => {
        calls.push(request.action);
        if (request.action === "clipboardRevision") return revision;
        if (request.action === "clipboardSyncRead") {
          const snapshot = { ...content, revision };
          if (delayedRead) {
            const pending = delayedRead;
            delayedRead = undefined;
            await pending;
          }
          return snapshot;
        }
        if (request.action === "clipboardSyncCancel") {
          if (pauseCancel) {
            pauseCancel = false;
            await new Promise((resolve) => {
              finishCancel = resolve;
            });
          }
          nativeGeneration++;
          return true;
        }
        if (request.action === "clipboardSyncRelease") {
          released.push(request.files);
          return true;
        }
        if (request.action === "clipboardSyncWrite") {
          written.push({ files: request.files, nativeGeneration });
          return { revision: ++revision };
        }
        assert.fail(`Unexpected native operation ${request.action}`);
      }),
    };
    let f = fixture({ native });
    try {
      await f.controller.sync();
      content = { files: "old-in-flight" };
      revision++;
      delayedRead = new Promise((resolve) => {
        finishRead = resolve;
      });
      const old = f.controller.poll();
      await settle();
      pauseCancel = true;
      if (transition === "new-session") {
        f.guest({
          hello: {
            version: 1,
            os: "linux",
            session: "replacement",
            formats: ["text", "png", "files"],
            available: true,
          },
        });
      } else {
        f.controller.dispose();
        f = fixture({ native });
      }
      content = { files: "current-selection" };
      revision++;
      const sync = f.controller.sync();
      await settle();
      const readCount = calls.filter(
        (action) => action === "clipboardSyncRead",
      ).length;
      assert.equal(
        readCount,
        2,
        "new native reads wait for actual cancel completion",
      );
      assert.equal(
        typeof finishCancel,
        "function",
        "cancel was not queued behind the old blocked read",
      );
      finishCancel();
      await sync;
      assert.deepEqual(f.sent.at(-1), { files: "current-selection" });
      finishRead();
      await old;
      await settle();
      assert.equal(
        f.sent.some((value) => value.files === "old-in-flight"),
        false,
      );
      assert.equal(released.filter((id) => id === "old-in-flight").length, 1);
      f.guest({ files: "new-guest-selection" });
      await settle();
      assert.deepEqual(written, [
        { files: "new-guest-selection", nativeGeneration: 1 },
      ]);
      assert.equal(
        released.filter((id) => id === "new-guest-selection").length,
        1,
      );
      assert.equal(f.active.at(-1), true);
    } finally {
      finishRead?.();
      finishCancel?.();
      f.controller.dispose();
      await settle();
    }
  });
}

test("native cancellation failures remain visible instead of suggesting a file-size problem", async (t) => {
  const originalWindow = globalThis.window;
  let failCancel = false;
  globalThis.window = {
    edenRuntimeTransport: createNativeTransport(async (request) => {
      if (request.action === "clipboardSyncCancel") {
        if (failCancel) throw new Error("测试：旧剪贴板取消失败");
        return true;
      }
      if (request.action === "clipboardRevision") return 1;
      if (request.action === "clipboardSyncRead")
        return { text: "initial", revision: 1 };
      assert.fail(request.action);
    }),
  };
  t.after(() => {
    if (originalWindow === undefined) delete globalThis.window;
    else globalThis.window = originalWindow;
  });
  const f = fixture({ native });
  try {
    await f.controller.sync();
    failCancel = true;
    f.guest({
      hello: {
        version: 1,
        os: "linux",
        session: "locked",
        formats: ["text", "png", "files"],
        available: false,
      },
    });
    await settle();
    assert.deepEqual(f.notices, ["剪贴板共享暂未完成：测试：旧剪贴板取消失败"]);
    assert.equal(f.active.at(-1), false);
  } finally {
    failCancel = false;
    f.controller.dispose();
    await settle();
  }
});

test("explicit sync rereads a superseded host snapshot and waits for the latest guest acknowledgement", async () => {
  const f = fixture();
  try {
    await settle();
    let release;
    f.host({ files: "slow host files" });
    f.readDelay(
      new Promise((resolve) => {
        release = resolve;
      }),
    );
    const background = f.controller.poll();
    await settle();
    f.host({ text: "latest host copy" });
    f.delayAcks();
    let finished = false;
    const sync = f.controller.sync().then(() => {
      finished = true;
    });
    release();
    await background;
    await settle();
    assert.deepEqual(f.sent.at(-1), { text: "latest host copy" });
    assert.deepEqual(f.released, ["slow host files"]);
    assert.equal(finished, false);
    f.acknowledge();
    await sync;
    assert.equal(finished, true);
  } finally {
    f.controller.dispose();
  }
});

test("guest file references stay alive until the last queued consumer finishes", async () => {
  const f = fixture();
  try {
    await settle();
    let release;
    f.writeDelay(
      new Promise((resolve) => {
        release = resolve;
      }),
    );
    f.guest({ files: "shared-selection" });
    await settle();
    f.guest({ files: "shared-selection" });
    await settle();
    assert.deepEqual(f.released, []);
    release();
    await settle();
    await settle();
    assert.deepEqual(f.released, ["shared-selection"]);
    assert.deepEqual(f.writes, [{ files: "shared-selection" }]);
  } finally {
    f.controller.dispose();
  }
});

test("explicit sync follows a new host revision that arrives while its acknowledgement is pending", async () => {
  const f = fixture();
  try {
    await settle();
    f.delayAcks();
    f.host({ text: "first copy" });
    let finished = false;
    const sync = f.controller.sync().then(() => {
      finished = true;
    });
    await settle();
    f.host({ text: "second copy" });
    f.acknowledge();
    await settle();
    assert.deepEqual(f.sent.at(-1), { text: "second copy" });
    assert.equal(finished, false);
    f.acknowledge();
    await sync;
  } finally {
    f.controller.dispose();
  }
});

for (const transition of ["lock", "switch", "disconnect"]) {
  test(`explicit sync fails when the guest ${transition} occurs before acknowledgement`, async () => {
    const f = fixture();
    try {
      await settle();
      f.delayAcks();
      f.host({ text: "unconfirmed host copy" });
      const rejected = assert.rejects(f.controller.sync());
      await settle();
      if (transition === "disconnect") f.socket.close();
      else
        f.guest({
          hello: {
            version: 1,
            os: "linux",
            session: transition === "lock" ? "fixture-session" : "next-session",
            formats: ["text", "png", "files"],
            available: transition !== "lock",
          },
        });
      f.acknowledge();
      await rejected;
    } finally {
      f.controller.dispose();
    }
  });
}

test("disconnect rejects explicit sync before a slow native read finishes", async () => {
  const f = fixture();
  let release;
  try {
    await settle();
    f.host({ files: "slow host files" });
    f.readDelay(
      new Promise((resolve) => {
        release = resolve;
      }),
    );
    const rejected = assert.rejects(f.controller.sync());
    await settle();
    f.socket.close();
    await Promise.race([
      rejected,
      new Promise((_, reject) =>
        setTimeout(
          () => reject(new Error("sync waited for an invalidated native read")),
          100,
        ),
      ),
    ]);
  } finally {
    release?.();
    f.controller.dispose();
  }
});

test("explicit sync rejects an unavailable guest", async () => {
  const f = fixture({ available: false });
  try {
    await assert.rejects(f.controller.sync());
  } finally {
    f.controller.dispose();
  }
});

test("a guest write error rejects its pending sync", async () => {
  const f = fixture();
  try {
    await settle();
    f.delayAcks();
    f.host({ text: "unconfirmed copy" });
    const rejected = assert.rejects(f.controller.sync());
    await settle();
    f.guest({ error: "clipboard_write_failed" });
    await rejected;
    assert.equal(f.active.at(-1), false);
  } finally {
    f.controller.dispose();
  }
});

test("a guest without an unlocked GUI cannot activate clipboard sharing", async () => {
  const f = fixture({ available: false });
  try {
    await settle();
    await f.controller.poll();
    assert.deepEqual(f.sent, []);
    assert.equal(f.active.at(-1), false);
    f.guest({
      hello: {
        version: 1,
        os: "linux",
        session: "unlocked-session",
        formats: ["text", "png", "files"],
        available: true,
      },
    });
    await settle();
    assert.deepEqual(f.sent, [{ text: "host initial" }]);
    assert.equal(f.active.at(-1), true);
  } finally {
    f.controller.dispose();
  }
});

test("locking a guest invalidates an in-flight host clipboard snapshot", async () => {
  const f = fixture();
  try {
    await settle();
    let release;
    f.host({ text: "copy made before guest lock" });
    f.readDelay(
      new Promise((resolve) => {
        release = resolve;
      }),
    );
    const pending = f.controller.poll();
    await settle();
    f.guest({
      hello: {
        version: 1,
        os: "linux",
        session: "locked",
        formats: ["text", "png", "files"],
        available: false,
      },
    });
    release();
    await pending;
    await settle();
    assert.equal(
      f.sent.some((value) => value.text === "copy made before guest lock"),
      false,
    );
    assert.equal(f.active.at(-1), false);
  } finally {
    f.controller.dispose();
  }
});

test("a restarted GUI session seeds the current host selection again", async () => {
  const f = fixture();
  try {
    await settle();
    f.guest({
      hello: {
        version: 1,
        os: "linux",
        session: "restarted-gui",
        formats: ["text", "png", "files"],
        available: true,
      },
    });
    await settle();
    assert.deepEqual(f.sent, [
      { text: "host initial" },
      { text: "host initial" },
    ]);
    assert.equal(f.active.at(-1), true);
  } finally {
    f.controller.dispose();
  }
});

for (const transition of ["lock", "switch", "disconnect"]) {
  test(`a pending native file write cannot update the host after guest ${transition}`, async () => {
    const f = fixture();
    try {
      await settle();
      let release;
      f.writeDelay(
        new Promise((resolve) => {
          release = resolve;
        }),
      );
      f.guest({ files: "old-session-selection" });
      await settle();
      if (transition === "disconnect") f.socket.close();
      else
        f.guest({
          hello: {
            version: 1,
            os: "linux",
            session: transition === "lock" ? "fixture-session" : "next-session",
            formats: ["text", "png", "files"],
            available: transition !== "lock",
          },
        });
      release();
      await settle();
      assert.deepEqual(f.writes, []);
      assert.deepEqual(f.notices, []);
    } finally {
      f.controller.dispose();
    }
  });
}

test("text, images and file selections synchronize both ways and do not echo", async () => {
  const f = fixture();
  try {
    await settle();
    assert.deepEqual(f.sent, [{ text: "host initial" }]);
    for (const content of [
      { text: "中文🙂\nsecond line" },
      { png: "fixture-png" },
      { files: "fixture-file-selection" },
    ]) {
      f.guest(content);
      await settle();
      assert.deepEqual(f.writes.at(-1), content);
      const sent = f.sent.length;
      await f.controller.poll();
      assert.equal(f.sent.length, sent, "a native write is not sent back");
      const writes = f.writes.length;
      f.guest(content);
      await settle();
      assert.equal(
        f.writes.length,
        writes,
        "a duplicate guest event is ignored",
      );
      f.host({
        ...content,
        ...(content.text
          ? { text: "new host text" }
          : content.png
            ? { png: "new host PNG" }
            : { files: "new host files" }),
      });
      await f.controller.poll();
      assert.equal(f.sent.length, sent + 1);
    }
  } finally {
    f.controller.dispose();
  }
});

test("a local copy made before a delayed guest write wins", async () => {
  const f = fixture();
  try {
    await settle();
    let release;
    f.writeDelay(
      new Promise((resolve) => {
        release = resolve;
      }),
    );
    f.guest({ text: "old guest copy" });
    await settle();
    f.host({ text: "new local copy" });
    release();
    await settle();
    await settle();
    assert.deepEqual(f.writes, []);
    assert.deepEqual(f.sent.at(-1), { text: "new local copy" });
  } finally {
    f.controller.dispose();
  }
});

test("a guest copy invalidates a host snapshot that was already being read", async () => {
  const f = fixture();
  try {
    await settle();
    f.host({ files: "slow host upload" });
    let release;
    f.readDelay(
      new Promise((resolve) => {
        release = resolve;
      }),
    );
    const poll = f.controller.poll();
    await settle();
    f.guest({ text: "new guest copy" });
    await settle();
    release();
    await poll;
    await settle();
    assert.equal(
      f.sent.some((value) => value.files === "slow host upload"),
      false,
    );
  } finally {
    f.controller.dispose();
  }
});

test("disposed connections cannot send snapshots or receive delayed writes", async () => {
  const f = fixture();
  await settle();
  f.controller.dispose();
  f.guest({ text: "late guest update" });
  await f.controller.poll();
  await settle();
  assert.deepEqual(f.writes, []);
  assert.deepEqual(f.active.at(-1), false);
});

test("a failing guest clipboard reconnects with backoff that resets after a successful sync", async (t) => {
  const delays = [],
    pending = [];
  let fail = true,
    revision = 1;
  const hello = {
    hello: {
      version: 1,
      os: "linux",
      session: "gui-a",
      available: true,
      formats: ["text", "png", "files"],
    },
  };
  const connect = () => {
    const socket = {
      readyState: 1,
      send: (text) => {
        const value = JSON.parse(text);
        queueMicrotask(() =>
          socket.onmessage({ data: JSON.stringify({ ack: value.serial }) }),
        );
      },
      close() {
        this.readyState = 3;
        this.onclose?.();
      },
    };
    queueMicrotask(() => {
      socket.onopen?.();
      socket.onmessage({ data: JSON.stringify(hello) });
    });
    return socket;
  };
  const controller = shareClipboard({
    connect,
    interval: 60000,
    onStatus: () => {},
    setActive: () => {},
    schedule: (callback, delay) => {
      delays.push(delay);
      pending.push(callback);
      return 0;
    },
    native: async (action) => {
      if (action === "clipboardRevision") return revision;
      if (action === "clipboardSyncRead") {
        if (fail) throw new Error("guest clipboard unavailable");
        return { text: "host", revision };
      }
    },
  });
  t.after(() => controller.dispose());
  for (let attempt = 0; attempt < 6; attempt++) {
    await settle();
    await settle();
    pending.shift()?.();
  }
  assert.deepEqual(delays, [1500, 3000, 6000, 12000, 24000, 30000]);
  fail = false;
  await settle();
  await settle();
  assert.equal(pending.length, 0);
  fail = true;
  revision = 2;
  controller.poll().catch(() => {});
  await settle();
  await settle();
  assert.equal(delays.at(-1), 1500);
  assert.equal(clipboardReconnectDelay(0), 1500);
  assert.equal(clipboardReconnectDelay(100), 30000);
});
