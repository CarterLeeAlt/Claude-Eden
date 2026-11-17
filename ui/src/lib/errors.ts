export class WorkbenchError extends Error {
  status?: number;
  code?: string;
  recovery?: string;
  constructor(
    message: string,
    details: {
      status?: number;
      code?: string;
      recovery?: string;
      cause?: unknown;
    } = {},
  ) {
    super(message, { cause: details.cause });
    this.name = "WorkbenchError";
    this.status = details.status;
    this.code = details.code;
    this.recovery = details.recovery;
  }
}
export function errorMessage(value: unknown): string {
  return value instanceof Error ? value.message : String(value);
}
export function errorStatus(value: unknown): number | undefined {
  return value instanceof WorkbenchError ? value.status : undefined;
}
export function errorCode(value: unknown): string | undefined {
  return value instanceof WorkbenchError ? value.code : undefined;
}
export const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === "object" && !Array.isArray(value);
