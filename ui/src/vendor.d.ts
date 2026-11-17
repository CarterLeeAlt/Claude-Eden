declare module "*novnc/core/input/util.js" {
  export function getKeycode(event: KeyboardEvent): string;
  export function getKeysym(event: KeyboardEvent): number | null;
}
declare module "*.generated/xterm.mjs" {
  export { Terminal } from "@xterm/xterm";
}
declare module "*novnc/core/rfb.js" {
  const RFB: typeof import("./lib/novnc-types.ts").RFB;
  export default RFB;
}
