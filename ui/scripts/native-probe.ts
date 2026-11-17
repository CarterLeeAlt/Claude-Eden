// VM-only page probe for internal/desktop/webview_self_test_windows.go.
import {
  Call,
  Events,
  setTransport,
  type RuntimeTransport,
} from "@wailsio/runtime";
import * as DesktopService from "../.generated/bindings/github.com/sylvinus/agent-vm/internal/desktop/desktopservice.js";
import { z } from "zod";

const host = window as Window & {
  edenRuntimeTransport?: RuntimeTransport;
  chrome?: { webview?: { postMessage(message: string): void } };
};
const stateSchema = z.object({
  backendStarted: z.boolean(),
  backendExited: z.boolean(),
  acceptedCalls: z.number().int(),
  blockedNavigations: z.number().int(),
  blockedPopups: z.number().int(),
  blockedDownloads: z.number().int(),
  scriptDialogs: z.number().int(),
  controllerInitializations: z.number().int(),
});
type ProbeState = z.infer<typeof stateSchema>;
const progressSchema = z.object({
  phase: z.enum(["initial", "reloaded", "recovered"]),
  checks: z.array(z.string()),
  initializations: z.number(),
  deadline: z.number(),
});
const key = "eden-native-selftest-probe";
const checks: string[] = [];
const result = document.getElementById("result");
const pause = (milliseconds: number) =>
  new Promise<void>((resolve) => setTimeout(resolve, milliseconds));
function requireCheck(condition: unknown, message: string): asserts condition {
  if (!condition) throw new Error(message);
}
function note(message: string) {
  checks.push(message);
  if (result) result.textContent = checks.join("\n");
}
function post(message: string) {
  requireCheck(
    host.chrome?.webview,
    "WebView2 IPC 不可用；探针仅在已有测试 VM 内执行",
  );
  host.chrome.webview.postMessage(message);
}
async function state(): Promise<ProbeState> {
  const response = await fetch("/probe-state", {
    cache: "no-store",
    signal: AbortSignal.timeout(5000),
  });
  requireCheck(response.ok, `/probe-state HTTP ${response.status}`);
  return stateSchema.parse(await response.json());
}
async function until<T>(
  read: () => T | Promise<T>,
  accept: (value: T) => boolean,
  description: string,
  timeout = 8000,
): Promise<T> {
  const deadline = Date.now() + timeout;
  do {
    const value = await read();
    if (accept(value)) return value;
    await pause(50);
  } while (Date.now() < deadline);
  throw new Error(`等待超时：${description}`);
}
async function rejected(operation: PromiseLike<unknown>, expected: RegExp) {
  try {
    await operation;
  } catch (error) {
    requireCheck(
      error instanceof Error &&
        expected.test(`${error.name}: ${error.message}`),
      `错误契约不匹配：${String(error)}`,
    );
    return;
  }
  throw new Error(`应拒绝的原生调用成功了：${expected}`);
}
async function eventRoundTrip(phase: string) {
  const expected = {
    owner: "bd799ff2-597d-4e24-a0a4-7db302d6b981",
    kind: "compositionupdate",
    text: "Go Event.Emit 真实数据",
    selectionStart: 2,
    selectionEnd: 2,
    textKey: false,
    repeat: false,
    ctrlKey: false,
    altKey: false,
    shiftKey: false,
    metaKey: false,
    modifierCapsLock: false,
  };
  let received: unknown;
  const remove = Events.On("eden-native-input", (event) => {
    received = event.data as unknown;
  });
  try {
    post("eden-selftest-emit-event");
    await until(
      () => received,
      (value) => value !== undefined,
      `${phase} Go Event.Emit`,
    );
    requireCheck(
      typeof received === "object" && received !== null,
      "Wails event.data 不是对象",
    );
    const data = received as Record<string, unknown>;
    requireCheck(
      Object.keys(data).length === Object.keys(expected).length &&
        Object.entries(expected).every(([name, value]) => data[name] === value),
      "Go Emit 与 npm Events.On 的 payload 不一致",
    );
  } finally {
    remove();
  }
  note(`${phase}：真实 npm Events.On 收到完整 Go Emit payload`);
}
async function blockedFrame(opaque: boolean, request: object) {
  const before = await state();
  const frame = document.createElement("iframe");
  frame.hidden = true;
  if (opaque) frame.setAttribute("sandbox", "allow-scripts");
  const nonce = crypto.randomUUID();
  // Deliberate hostile IPC, not a replacement transport: replay a descriptor
  // captured from the actual generated AccountDevice call in a forbidden frame.
  frame.srcdoc = `<script>(() => { const value = ${JSON.stringify({ nonce, envelope: "eden-runtime-v1:" + JSON.stringify({ id: crypto.randomUUID(), request }) })}; let posted = false, error = ''; try { if (window.chrome?.webview) { window.chrome.webview.postMessage(value.envelope); posted = true; } } catch (reason) { error = String(reason); } parent.postMessage({ nonce: value.nonce, posted, error }, '*'); })();<\/script>`;
  const acknowledged = new Promise<MessageEvent>((resolve) => {
    const receive = (event: MessageEvent) => {
      if (event.source === frame.contentWindow && event.data?.nonce === nonce) {
        window.removeEventListener("message", receive);
        resolve(event);
      }
    };
    window.addEventListener("message", receive);
    frame.addEventListener(
      "load",
      () =>
        setTimeout(() => window.removeEventListener("message", receive), 4000),
      { once: true },
    );
  });
  document.body.append(frame);
  try {
    const event = await Promise.race([
      acknowledged,
      pause(5000).then(() => {
        throw new Error("非法 iframe 没有返回探针确认");
      }),
    ]);
    requireCheck(
      event.origin === (opaque ? "null" : location.origin),
      `iframe origin 不符：${event.origin}`,
    );
    await pause(400);
    requireCheck(
      (await state()).acceptedCalls === before.acceptedCalls,
      "非法 origin/iframe 的 IPC 被接受",
    );
    note(
      `${opaque ? "opaque origin" : "同源子 iframe"}：${event.data.posted ? "真实恶意 IPC 未进入 Go dispatcher" : "WebView2 未向此帧暴露 IPC"}`,
    );
  } finally {
    frame.remove();
  }
}
async function policyChecks() {
  let before = await state();
  const popup = window.open(`${location.origin}/?blocked-popup`, "_blank");
  await until(
    state,
    (value) => value.blockedPopups > before.blockedPopups,
    "popup 拒绝通知",
  );
  requireCheck(!popup || popup.closed, "被拒绝的 popup 仍可用");
  note("popup：原生策略拒绝并递增 blockedPopups");
  before = await state();
  const url = URL.createObjectURL(
    new Blob(["Claude Eden VM-only download probe"], {
      type: "application/octet-stream",
    }),
  );
  const link = document.createElement("a");
  link.href = url;
  link.download = "eden-native-selftest.txt";
  document.body.append(link);
  link.click();
  try {
    await until(
      state,
      (value) => value.blockedDownloads > before.blockedDownloads,
      "download 拒绝通知",
    );
  } finally {
    link.remove();
    URL.revokeObjectURL(url);
  }
  note("download：原生策略取消下载并递增 blockedDownloads");
  before = await state();
  const original = location.href;
  location.href = `http://127.0.0.1:${location.port}/?forbidden-navigation`;
  await until(
    state,
    (value) => value.blockedNavigations > before.blockedNavigations,
    "非法 navigation 拒绝通知",
  );
  requireCheck(location.href === original, "非法导航改变了工作台 origin");
  note("navigation：非法 loopback origin 被拦截，工作台 origin 保持不变");
  requireCheck(
    "Notification" in window,
    "WebView2 Notification API 不可用，无法验证 permission 策略",
  );
  const permission = await Notification.requestPermission();
  requireCheck(
    permission === "denied",
    `原生 permission 策略未拒绝：${permission}`,
  );
  note("permission：真实 Notification.requestPermission 返回 denied");
  before = await state();
  window.alert("Claude Eden VM 脚本对话框探针：应由原生处理并自动关闭。");
  await until(
    state,
    (value) => value.scriptDialogs > before.scriptDialogs,
    "原生 script-dialog 通知",
  );
  note("script-dialog：经过原生通知处理并自动关闭，未使用浏览器默认对话框");
}
async function run() {
  requireCheck(
    window === window.top &&
      location.hostname === "localhost" &&
      location.protocol === "http:",
    "探针只允许在测试 VM localhost 顶层页面运行",
  );
  const raw = localStorage.getItem(key);
  const progress = raw
    ? progressSchema.parse(JSON.parse(raw))
    : {
        phase: "initial" as const,
        checks: [],
        initializations: 0,
        deadline: Date.now() + 75000,
      };
  checks.push(...progress.checks);
  requireCheck(Date.now() < progress.deadline, "跨重载/重建探针超过总时限");
  const transport = await until(
    () => host.edenRuntimeTransport,
    (value) => Boolean(value),
    "edenRuntimeTransport 注入",
  );
  requireCheck(transport, "未取得真实 Transport");
  let captured: { object: number; method: number; args: unknown } | undefined;
  setTransport({
    call(object, method, windowName, args) {
      if (object === 0) captured = { object, method, args };
      return transport.call(object, method, windowName, args);
    },
  });
  const before = await state();
  const device = await DesktopService.AccountDevice("status");
  requireCheck(
    device.supported === false,
    "AccountDevice(status) 真实结果不符",
  );
  const after = await state();
  requireCheck(
    after.acceptedCalls > before.acceptedCalls && captured,
    "生成绑定没有经过真实 Go dispatcher",
  );
  const capturedRequest = captured;
  note(
    `${progress.phase}：setTransport + 真实生成 AccountDevice/Call.ByID 成功`,
  );
  await eventRoundTrip(progress.phase);
  if (progress.phase === "initial") {
    await rejected(
      DesktopService.AccountDevice("native-probe-invalid"),
      /RuntimeError.*此平台暂不支持快捷登录/,
    );
    const descriptor = z
      .object({ methodID: z.number() })
      .passthrough()
      .parse(capturedRequest.args);
    await rejected(Call.ByID(descriptor.methodID, false), /TypeError/);
    await rejected(
      Call.ByID(0),
      /^RuntimeError: Binding call failed: unknown bound method id 0$/,
    );
    requireCheck(
      (await DesktopService.AccountDevice("status")).supported === false,
      "未知方法拒绝后，后续正式绑定调用受到污染",
    );
    note("Call.ByID：真实 Go 业务错误、参数类型错误和未知方法均拒绝");
    const pending = DesktopService.RefreshSession();
    const outcome = pending.then(
      () => "resolved",
      (error: unknown) => error,
    );
    await until(
      state,
      (value) => value.backendStarted && !value.backendExited,
      "RefreshSession 测试子进程启动",
    );
    await pending.cancel("eden-native-selftest-cancel");
    const cancellation = await outcome;
    requireCheck(
      cancellation instanceof Error && cancellation.name === "CancelError",
      `调用未按 CancelError 取消：${String(cancellation)}`,
    );
    await until(
      state,
      (value) => value.backendStarted && value.backendExited,
      "取消后测试子进程退出",
      12000,
    );
    note(
      "RefreshSession：真实 CancellablePromise 取消后 test-backend 子进程已退出",
    );
    await blockedFrame(false, capturedRequest);
    await blockedFrame(true, capturedRequest);
    await policyChecks();
    localStorage.setItem(
      key,
      JSON.stringify({
        phase: "reloaded",
        checks,
        initializations: after.controllerInitializations,
        deadline: progress.deadline,
      }),
    );
    location.reload();
    return;
  }
  if (progress.phase === "reloaded") {
    requireCheck(
      after.controllerInitializations === progress.initializations,
      "普通页面 reload 意外重建了 WebView controller",
    );
    note("reload：同一 WebView controller 中重新绑定并收到真实事件");
    localStorage.setItem(
      key,
      JSON.stringify({
        phase: "recovered",
        checks,
        initializations: after.controllerInitializations,
        deadline: progress.deadline,
      }),
    );
    post("eden-selftest-rebuild-webview");
    return;
  }
  requireCheck(
    after.controllerInitializations > progress.initializations,
    "RecoverWebView 没有创建新的 controller",
  );
  note("RecoverWebView：新 controller 中重新绑定并收到真实事件");
  localStorage.removeItem(key);
  post(
    "eden-selftest-result:" +
      JSON.stringify({ success: true, checks, error: "" }),
  );
}
void run().catch((error) => {
  const message =
    error instanceof Error ? `${error.name}: ${error.message}` : String(error);
  if (result) result.textContent = [...checks, message].join("\n");
  post(
    "eden-selftest-result:" +
      JSON.stringify({ success: false, checks, error: message }),
  );
});
