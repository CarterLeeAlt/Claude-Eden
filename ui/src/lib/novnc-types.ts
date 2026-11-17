import type { ClipboardContent } from "./types.ts";
import type { InputKey } from "./remote-input.ts";
export declare class RFB extends EventTarget {
  constructor(
    container: HTMLElement,
    url: string,
    options: {
      credentials: { password: string };
      workbenchBrowserDisplay: boolean;
      workbenchWheelSink?: (wheel: {
        deltaX: number;
        deltaY: number;
        x: number;
        y: number;
      }) => Promise<boolean>;
    },
  );
  scaleViewport: boolean;
  resizeSession: boolean;
  background: string;
  qualityLevel: number;
  compressionLevel: number;
  focus(): void;
  blur(): void;
  disconnect(): void;
  sendKey(
    keysym: number | null | undefined,
    code?: string,
    isDown?: boolean,
  ): void;
  workbenchSetVisible(visible: boolean): void;
  workbenchSetKeyboardTarget(target: HTMLTextAreaElement | null): void;
  workbenchPasteText(text: string): Promise<unknown>;
  workbenchRunInput<T>(
    action: (send: (...key: InputKey) => unknown) => T | Promise<T>,
  ): Promise<T>;
  workbenchPasteContent(
    content: ClipboardContent | Promise<ClipboardContent>,
    send: (payload: ClipboardContent) => Promise<unknown>,
    valid: () => boolean,
    sendText?: (text: string) => Promise<unknown>,
  ): Promise<unknown>;
}
