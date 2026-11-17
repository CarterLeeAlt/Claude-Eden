import { api } from "./api.ts";
import { runtimeSocketURL } from "./client-context.ts";
import workletURL from "./audio-worklet.ts?worker&url";
import { errorMessage } from "./errors.ts";
import { audioFormatSchema } from "./contracts.ts";

export class AudioOutput {
  onState: (state: string) => void;
  onError: (message: string) => void;
  active: boolean;
  volume: number;
  generation: number;
  context: AudioContext | null = null;
  gain: GainNode | null = null;
  node: AudioWorkletNode | null = null;
  socket: WebSocket | null = null;
  retry: ReturnType<typeof setTimeout> | undefined;
  opening = false;
  disposed = false;
  preparing: Promise<void> | null = null;
  request: AbortController | null = null;
  constructor(
    onState: (state: string) => void,
    onError: (message: string) => void = () => {},
  ) {
    this.onState = onState;
    this.onError = onError;
    this.active = false;
    this.volume = 1;
    this.generation = 0;
  }
  setActive(active: boolean) {
    if (this.active === active) return;
    this.active = active;
    if (active) this.activate();
    else {
      this.disconnect();
      this.context?.suspend().catch(() => {});
      this.onState("idle");
      this.onError("");
    }
  }
  setVolume(value: number) {
    this.volume = value;
    if (this.gain) this.gain.gain.value = value;
  }
  activate() {
    if (this.disposed || !this.active) return;
    try {
      if (!this.context) {
        this.context = new AudioContext({ latencyHint: "interactive" });
        this.context.onstatechange = () => {
          if (this.disposed || !this.active) return;
          if (this.context?.state === "running") this.connect();
          else {
            this.disconnect();
            this.onState("blocked");
          }
        };
      }
      // Call resume directly from the pointer/key handler, before awaiting any
      // API request, so all three platform WebViews retain the user gesture.
      const resume = this.context.resume();
      if (this.context.state !== "running") this.onState("blocked");
      void resume
        .then(() => {
          if (this.context?.state === "running") this.connect();
        })
        .catch(() => this.onState("blocked"));
    } catch {
      this.onState("unsupported");
    }
  }
  disconnect() {
    this.generation++;
    this.request?.abort();
    this.request = null;
    clearTimeout(this.retry);
    this.opening = false;
    const socket = this.socket;
    this.socket = null;
    if (socket) {
      socket.onclose = null;
      socket.close();
    }
    this.node?.port.postMessage("reset");
  }
  async prepare() {
    const context = this.context;
    if (!context) throw new Error("声音设备尚未初始化。");
    if (!this.preparing) {
      this.preparing = context.audioWorklet
        .addModule(workletURL)
        .then(() => {
          if (this.disposed) return;
          this.node = new AudioWorkletNode(context, "eden-audio", {
            numberOfInputs: 0,
            numberOfOutputs: 1,
            outputChannelCount: [2],
          });
          this.gain = context.createGain();
          this.gain.gain.value = this.volume;
          this.node.connect(this.gain).connect(context.destination);
        })
        .catch((error) => {
          this.preparing = null;
          throw error;
        });
    }
    return this.preparing;
  }
  async connect() {
    if (
      !this.active ||
      this.disposed ||
      this.context?.state !== "running" ||
      this.socket ||
      this.opening
    )
      return;
    clearTimeout(this.retry);
    this.opening = true;
    const generation = this.generation;
    const request = new AbortController();
    this.request = request;
    const live = () =>
      this.active && !this.disposed && generation === this.generation;
    this.onState("connecting");
    const reconnect = () => {
      if (!live()) return;
      this.opening = false;
      this.socket = null;
      this.node?.port.postMessage("reset");
      this.onState("disconnected");
      this.retry = setTimeout(() => this.connect(), 2500);
    };
    try {
      await this.prepare();
      if (!live()) return;
      const format = await api(
        "/audio",
        undefined,
        undefined,
        {},
        { signal: request.signal },
      );
      if (!live()) return;
      if (
        format.format !== "s16le" ||
        format.sample_rate !== 48000 ||
        format.channels !== 2
      )
        throw new Error("声音格式不兼容");
      this.onError("");
      const socket = new WebSocket(runtimeSocketURL("/audio"));
      socket.binaryType = "arraybuffer";
      this.socket = socket;
      let confirmed = false;
      socket.onmessage = ({ data }) => {
        if (!live()) return;
        if (typeof data === "string") {
          try {
            const header = audioFormatSchema.parse(JSON.parse(data));
            confirmed =
              header.format === "s16le" &&
              header.sample_rate === 48000 &&
              header.channels === 2;
          } catch {
            confirmed = false;
          }
          if (!confirmed) socket.close();
        } else if (confirmed) {
          this.onState("playing");
          this.node?.port.postMessage(data, [data]);
        }
      };
      socket.onclose = reconnect;
      socket.onerror = () => socket.close();
      this.opening = false;
    } catch (error) {
      if (live())
        this.onError(errorMessage(error) || "声音连接未能建立，请重试");
      reconnect();
    } finally {
      if (this.request === request) this.request = null;
    }
  }
  close() {
    this.disposed = true;
    this.disconnect();
    this.node?.disconnect();
    this.gain?.disconnect();
    this.context?.close().catch(() => {});
  }
}
