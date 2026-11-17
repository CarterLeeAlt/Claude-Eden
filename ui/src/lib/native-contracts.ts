import { z } from "zod";
import {
  DesktopClipboardContentSchema,
  DesktopFileSelectionSchema,
  DesktopFileResultSchema,
  DesktopRemoteInputResultSchema,
  DesktopUpdateStateSchema,
  DesktopDeviceStatusSchema,
} from "./contracts.generated.ts";
export const nativeResults = {
  refreshSession: z.object({ url: z.string().url() }),
  reconnect: z.boolean(),
  quitNow: z.boolean(),
  workbenchReady: z.boolean(),
  workbenchFailed: z.boolean(),
  appearance: z.boolean(),
  accountDevice: DesktopDeviceStatusSchema,
  clipboardRead: z.string(),
  clipboardWrite: z.boolean(),
  clipboardReadContent: DesktopClipboardContentSchema,
  clipboardRevision: z.number(),
  clipboardSyncRead: DesktopClipboardContentSchema,
  clipboardSyncWrite: DesktopClipboardContentSchema,
  clipboardSyncRelease: z.boolean(),
  clipboardSyncCancel: z.boolean(),
  remoteInput: DesktopRemoteInputResultSchema,
  chooseFolder: z.string(),
  chooseUploadFiles: DesktopFileSelectionSchema,
  uploadHostFiles: DesktopFileResultSchema,
  browserDownloadAction: DesktopFileResultSchema,
  desktopFileAction: DesktopFileResultSchema,
  importTheme: z.string(),
  exportTheme: z.boolean(),
  saveRecoveryKey: z.boolean(),
  exportDiagnostics: DesktopFileResultSchema,
  appReference: z.boolean(),
  appUpdate: DesktopUpdateStateSchema,
} as const;
export type NativeAction = keyof typeof nativeResults;
export type NativeResult<A extends NativeAction> = z.infer<
  (typeof nativeResults)[A]
>;
export type Native = <A extends NativeAction>(
  action: A,
  extra?: Record<string, unknown>,
) => Promise<NativeResult<A>>;
