import type { RuntimeTransport } from "@wailsio/runtime";
import { z } from "zod";
import { reportDiagnostic } from "./diagnostics.ts";
import {
  DesktopInputEventSchema,
  DesktopUpdateStateSchema,
} from "./contracts.generated.ts";

export const nativeAvailable = () =>
  Boolean(window.edenRuntimeTransport || window.chrome?.webview);
let ready:
  | Promise<
      typeof import("../../.generated/bindings/github.com/sylvinus/agent-vm/internal/desktop/desktopservice.js")
    >
  | undefined;
let runtimeReady: Promise<typeof import("@wailsio/runtime")> | undefined;

async function transportReady(): Promise<RuntimeTransport> {
  if (window.edenRuntimeTransport) return window.edenRuntimeTransport;
  if (!nativeAvailable())
    throw new Error(
      "原生窗口连接不可用，请从 ClaudeEden 桌面应用打开工作台。",
    );
  return new Promise((resolve) => {
    const accept = () => {
      if (!window.edenRuntimeTransport) return;
      window.removeEventListener("eden-runtime-transport-ready", accept);
      resolve(window.edenRuntimeTransport);
    };
    window.addEventListener("eden-runtime-transport-ready", accept);
    accept();
  });
}

function desktopRuntime() {
  runtimeReady ??= (async () => {
    const transport = await transportReady();
    const runtime = await import("@wailsio/runtime");
    runtime.setTransport(transport);
    window.edenNativeIMEAvailable = true;
    return runtime;
  })();
  return runtimeReady;
}

export function desktopService() {
  ready ??= desktopRuntime().then(
    () =>
      import("../../.generated/bindings/github.com/sylvinus/agent-vm/internal/desktop/desktopservice.js"),
  );
  return ready;
}

const eventSchemas = {
  "eden-native-input": DesktopInputEventSchema,
  "eden-native-action": z.enum([
    "openProject",
    "newTab",
    "newTerminal",
    "closeTab",
    "settings",
    "quit",
  ]),
  "eden-window-deactivated": z.unknown(),
  "eden-window-visibility": z.boolean(),
  "eden-app-update": DesktopUpdateStateSchema,
  "eden-startup-state": z.object({
    phase: z.string(),
    message: z.string(),
    retryable: z.boolean().optional(),
    canOpenExisting: z.boolean().optional(),
    canCompleteUpdate: z.boolean().optional(),
  }),
} as const;
export type DesktopEventName = keyof typeof eventSchemas;
export function onDesktopEvent<E extends DesktopEventName>(
  name: E,
  receive: (value: z.infer<(typeof eventSchemas)[E]>) => void,
): () => void {
  let disposed = false;
  let remove: (() => void) | undefined;
  if (nativeAvailable())
    void desktopRuntime()
      .then(({ Events }) => {
        if (disposed) return;
        remove = Events.On(name, (event) => {
          const parsed = eventSchemas[name].safeParse(event.data as unknown);
          if (!parsed.success) {
            reportDiagnostic("frontend", parsed.error);
            return;
          }
          receive(parsed.data as z.infer<(typeof eventSchemas)[E]>);
        });
      })
      .catch((error) => {
        if (!disposed) reportDiagnostic("frontend", error);
      });
  return () => {
    disposed = true;
    remove?.();
  };
}
