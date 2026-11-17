// Browser-only service fixture. Generated bindings and the npm Wails runtime
// remain real; the OS operations are replaced only in isolated UI smoke tests.
import { readFileSync } from "node:fs";

const source = readFileSync(new URL("../.generated/bindings/github.com/sylvinus/agent-vm/internal/desktop/desktopservice.ts", import.meta.url), "utf8");
const methods = Object.fromEntries([...source.matchAll(/export function (\w+)\([^]*?\$Call\.ByID\((\d+)/g)].map(([, name, id]) => [id, name[0].toLowerCase() + name.slice(1)]));

export async function installNativeFixture(page, request = async () => undefined) {
  await page.exposeFunction("edenFixtureNativeRequest", request);
  const install = methods => {
    window.emitTestDesktopEvent = (name, data) => {
      if (name === "eden-native-input") data = { selectionStart: 0, selectionEnd: 0, textKey: false, repeat: false, ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, modifierCapsLock: false, ...data };
      if (name === "eden-app-update") data = { currentVersion: "fixture", message: "", retryable: false, retryAt: "", ...data };
      window._wails.dispatchWailsEvent({ name, data });
    };
    window.edenTestNativeRequest = async body => {
      const value = await window.edenFixtureNativeRequest(body);
      if (value !== undefined && value !== null) return value;
      const { action } = body;
      if (action === "appUpdate") return { phase: "idle", currentVersion: "fixture", message: "", retryable: false, retryAt: "" };
      if (action === "accountDevice") return { supported: false };
      if (action === "remoteInput") return { accepted: true, returnedFocus: false };
      if (action === "clipboardRead") return "fixture clipboard";
      if (["clipboardReadContent", "clipboardSyncRead", "clipboardSyncWrite"].includes(action)) return { revision: 0, text: "fixture clipboard" };
      if (action === "clipboardRevision") return 0;
      if (action === "chooseUploadFiles") return { canceled: true };
      if (action === "chooseFolder") return "";
      if (["browserDownloadAction", "desktopFileAction", "uploadHostFiles", "exportDiagnostics"].includes(action)) return {};
      if (action === "importTheme") return "";
      if (action === "refreshSession") return { url: location.origin + "/#fixture-native-window-ticket" };
      return true;
    };
    window.edenRuntimeTransport = { async call(object, method, _windowName, request) {
      if (object === 10) return {};
      if (object !== 0 || method !== 0 || !methods[request.methodID]) throw new Error("Unexpected fixture runtime request");
      const action = methods[request.methodID], first = request.args[0];
      const body = { action, ...(first && typeof first === "object" ? first : {}) };
      if (["appUpdate", "accountDevice"].includes(action)) body.operation = first;
      if (action === "appReference") body.kind = first;
      if (action === "chooseFolder") body.purpose = first;
      if (action === "chooseUploadFiles") body.multiple = first;
      if (action === "clipboardWrite") body.text = first;
      if (action === "workbenchFailed") body.message = first;
      if (action === "saveRecoveryKey") body.key = first;
      return window.edenTestNativeRequest(body);
    } };
  };
  await page.addInitScript(install, methods);
  await page.evaluate(install, methods);
}
