export {};
import type { RuntimeTransport } from "@wailsio/runtime";
declare module "react" {
  interface CSSProperties {
    [key: `--${string}`]: string | number | undefined;
  }
}
declare global {
  var AudioWorkletProcessor: { new (): { readonly port: MessagePort } };
  var registerProcessor:
    | ((name: string, processor: typeof AudioWorkletProcessor) => void)
    | undefined;
  var sampleRate: number;
  interface Window {
    edenWindowVisible?: boolean;
    edenSharedClipboardActive?: boolean;
    edenNativeIMEAvailable?: boolean;
    edenSyncClipboard?: () => Promise<unknown>;
    edenWorkbenchAction?: (command: string) => unknown;
    edenEditAction?: (command: string) => unknown;
    chrome?: { webview?: unknown };
    edenRuntimeTransport?: RuntimeTransport;
  }
  interface KeyboardEvent {
    edenNativeTextKey?: boolean;
    edenNativeInput?: boolean;
  }
}
