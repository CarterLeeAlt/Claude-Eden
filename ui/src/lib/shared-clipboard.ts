import { z } from "zod";
import type { Native } from "./native-contracts.ts";
import { errorMessage } from "./errors.ts";
interface SharedClipboardPayload {
  text?: string;
  png?: string;
  files?: string;
  revision?: number;
}
const incomingSchema = z.object({
  text: z.string().optional(),
  png: z.string().optional(),
  files: z.string().optional(),
  revision: z.number().optional(),
  ack: z.number().optional(),
  error: z.string().optional(),
  hello: z
    .object({
      version: z.number(),
      os: z.string(),
      session: z.string(),
      formats: z.array(z.string()),
      available: z.boolean(),
    })
    .optional(),
});
type ClipboardMessage = z.infer<typeof incomingSchema>;
// A guest whose clipboard service keeps failing, for example while its
// components wait for an update, must not be hit every 1.5 seconds.
export const clipboardReconnectDelay = (failures: number) =>
  Math.min(1500 * 2 ** Math.max(0, failures), 30000);

// One connection owns clipboard synchronization for the workbench window,
// including while its remote canvas is hidden or the window is unfocused.
export function shareClipboard({
  connect,
  native,
  onStatus,
  setActive,
  interval = 300,
  schedule = setTimeout,
}: {
  connect: () => WebSocket;
  native: Native;
  onStatus: (message: string) => void;
  setActive: (active: boolean) => void;
  interval?: number;
  schedule?: typeof setTimeout;
}) {
  let socket: WebSocket | undefined,
    timer: ReturnType<typeof setInterval> | undefined,
    retry: ReturnType<typeof setTimeout> | undefined;
  let disposed = false,
    revision: number | undefined,
    generation = 0,
    guestReady = false,
    guestSession: string | undefined,
    failures = 0;
  let polling: Promise<void> | undefined,
    writes = Promise.resolve(),
    lastPayload = "",
    serial = 0,
    errorShown = false,
    acknowledgedRevision: number | undefined;
  let sessionGeneration = 0;
  let sessionController = new AbortController();
  const acknowledgements = new Map<
    number,
    { resolve: () => void; reject: () => void }
  >();
  const fileReferences = new Map<string, number>();
  const retainFiles = (value: { files?: string }) => {
    const id = value?.files;
    if (typeof id !== "string") return () => {};
    fileReferences.set(id, (fileReferences.get(id) || 0) + 1);
    return () => {
      const remaining = fileReferences.get(id)! - 1;
      if (remaining) {
        fileReferences.set(id, remaining);
        return;
      }
      fileReferences.delete(id);
      void native("clipboardSyncRelease", { files: id }).catch(() => {});
    };
  };
  const disconnected = () =>
    new Error("剪贴板连接已变化，请确认客机已连接后重新粘贴。");
  const invalidateSession = () => {
    generation++;
    sessionGeneration++;
    sessionController.abort();
    sessionController = new AbortController();
    guestReady = false;
    revision = undefined;
    acknowledgedRevision = undefined;
    lastPayload = "";
    polling = undefined;
    writes = Promise.resolve();
    for (const pending of acknowledgements.values()) pending.reject();
    acknowledgements.clear();
    setActive(false);
    void native("clipboardSyncCancel").catch(warn);
  };
  const requireSession = (signal: AbortSignal) => {
    if (disposed || signal.aborted || !guestReady || socket?.readyState !== 1)
      throw disconnected();
  };
  const waitForSession = <T>(work: Promise<T>, signal: AbortSignal) =>
    new Promise<T>((resolve, reject) => {
      const abort = () => {
        signal.removeEventListener("abort", abort);
        reject(disconnected());
      };
      Promise.resolve(work)
        .then(resolve, reject)
        .finally(() => signal.removeEventListener("abort", abort));
      if (signal.aborted) abort();
      else signal.addEventListener("abort", abort, { once: true });
    });
  const warn = (error?: unknown) => {
    if (!disposed && !errorShown) {
      errorShown = true;
      onStatus(
        error === undefined
          ? "剪贴板共享暂未完成，请检查文件是否仍存在及大小限制。"
          : `剪贴板共享暂未完成：${errorMessage(error)}`,
      );
    }
  };
  const send = (connection: WebSocket, content: SharedClipboardPayload) =>
    new Promise<void>((resolve, reject) => {
      const id = ++serial;
      const timeout = setTimeout(() => {
        acknowledgements.delete(id);
        reject(new Error("Clipboard acknowledgement timed out"));
      }, 8000);
      acknowledgements.set(id, {
        resolve: () => {
          clearTimeout(timeout);
          resolve();
        },
        reject: () => {
          clearTimeout(timeout);
          reject(new Error("Clipboard disconnected"));
        },
      });
      try {
        connection.send(JSON.stringify({ ...content, serial: id }));
      } catch {
        acknowledgements.get(id)?.reject();
        acknowledgements.delete(id);
      }
    });
  const payload = (value: {
    text?: string | null;
    png?: string;
    files?: string;
  }): SharedClipboardPayload | null =>
    typeof value.text === "string"
      ? { text: value.text }
      : typeof value.png === "string"
        ? { png: value.png }
        : typeof value.files === "string"
          ? { files: value.files }
          : null;
  const readHost = async (initial = false) => {
    if (disposed || !guestReady || socket?.readyState !== 1) return;
    const currentSocket = socket,
      currentGeneration = generation,
      session = sessionGeneration;
    let releaseFiles = () => {};
    try {
      const current = await native("clipboardRevision");
      if (disposed || socket !== currentSocket || session !== sessionGeneration)
        return;
      if (typeof current !== "number")
        throw new Error("Clipboard revision unavailable");
      if (!initial && current === revision && current === acknowledgedRevision)
        return;
      const value = await native("clipboardSyncRead");
      releaseFiles = retainFiles(value);
      const content = payload(value);
      const finalRevision = await native("clipboardRevision");
      if (
        disposed ||
        !guestReady ||
        socket !== currentSocket ||
        currentGeneration !== generation ||
        value.revision !== finalRevision
      )
        return;
      revision = value.revision;
      if (!content) throw new Error("Clipboard content unavailable");
      const encoded = JSON.stringify(content);
      if (
        initial ||
        value.revision !== acknowledgedRevision ||
        encoded !== lastPayload
      )
        await send(currentSocket, content);
      if (
        disposed ||
        !guestReady ||
        socket !== currentSocket ||
        currentGeneration !== generation
      )
        return;
      acknowledgedRevision = value.revision;
      lastPayload = encoded;
      setActive(true);
      errorShown = false;
      failures = 0;
    } catch (error) {
      if (disposed || socket !== currentSocket || session !== sessionGeneration)
        return;
      warn(error);
      setActive(false);
      revision = undefined;
      lastPayload = "";
      currentSocket.close();
      throw new Error("Clipboard synchronization failed");
    } finally {
      releaseFiles();
    }
  };
  const poll = (initial = false) => {
    if (!polling) {
      const current = readHost(initial).finally(() => {
        if (polling === current) polling = undefined;
      });
      polling = current;
    }
    return polling;
  };
  const receive = (value: ClipboardMessage) => {
    if (value.hello) {
      const hello = value.hello;
      const supported =
        hello.version === 1 &&
        hello.os === "linux" &&
        typeof hello.session === "string" &&
        hello.session.length > 0 &&
        ["text", "png", "files"].every((format) =>
          hello.formats?.includes(format),
        );
      const previouslyReady = guestReady;
      if (
        previouslyReady &&
        (guestSession !== hello.session || !supported || !hello.available)
      )
        invalidateSession();
      guestSession = hello.session;
      guestReady = supported && hello.available === true;
      if (!guestReady) setActive(false);
      if (guestReady && (!previouslyReady || revision === undefined))
        void poll(true).catch(warn);
      return;
    }
    if (value.ack) {
      acknowledgements.get(value.ack)?.resolve();
      acknowledgements.delete(value.ack);
      return;
    }
    if (value.error) {
      warn();
      socket?.close();
      return;
    }
    const content = payload(value),
      sequence = ++generation,
      currentSocket = socket,
      session = sessionGeneration;
    if (!content) return;
    const releaseFiles = retainFiles(content);
    writes = writes
      .then(async () => {
        if (
          disposed ||
          !guestReady ||
          socket !== currentSocket ||
          sequence !== generation
        )
          return;
        const current = await native("clipboardRevision");
        if (
          disposed ||
          !guestReady ||
          socket !== currentSocket ||
          sequence !== generation
        )
          return;
        if (revision === undefined || current !== revision) {
          // A newer local copy wins over an update that was already in transit.
          void poll().catch(warn);
          return;
        }
        if (JSON.stringify(content) === lastPayload) return;
        const result = await native("clipboardSyncWrite", {
          ...content,
          revision,
        });
        if (
          disposed ||
          socket !== currentSocket ||
          session !== sessionGeneration
        )
          return;
        if (result.superseded) {
          void poll().catch(warn);
          return;
        }
        revision = result.revision;
        acknowledgedRevision = result.revision;
        lastPayload = JSON.stringify(content);
        onStatus(
          content.files ? "文件已复制到本机，可在本机粘贴" : "已复制到本机",
        );
      })
      .catch(() => {
        if (session === sessionGeneration) warn();
      })
      .finally(releaseFiles);
  };
  const open = () => {
    if (disposed) return;
    revision = undefined;
    lastPayload = "";
    guestReady = false;
    socket = connect();
    const connection = socket;
    socket.onopen = () => {
      if (disposed || socket !== connection) return;
      timer = setInterval(() => void poll().catch(warn), interval);
    };
    socket.onmessage = (event) => {
      if (disposed || socket !== connection) return;
      const parsed = incomingSchema.safeParse(JSON.parse(event.data));
      if (parsed.success) receive(parsed.data);
    };
    socket.onclose = () => {
      if (disposed || socket !== connection) return;
      invalidateSession();
      clearInterval(timer);
      retry = schedule(open, clipboardReconnectDelay(failures++));
    };
    socket.onerror = () => {
      /* onclose owns reconnects. */
    };
  };
  open();
  return {
    poll,
    async sync() {
      const signal = sessionController.signal;
      requireSession(signal);
      for (;;) {
        await waitForSession(poll(), signal);
        const pendingWrites = writes;
        await waitForSession(pendingWrites, signal);
        const sequence = generation;
        const current = await waitForSession(
          native("clipboardRevision"),
          signal,
        );
        requireSession(signal);
        if (typeof current !== "number")
          throw new Error("无法确认当前剪贴板版本，请重新粘贴。");
        if (
          current === acknowledgedRevision &&
          sequence === generation &&
          pendingWrites === writes
        )
          return;
      }
    },
    dispose() {
      disposed = true;
      invalidateSession();
      clearInterval(timer);
      clearTimeout(retry);
      socket?.close();
    },
  };
}
