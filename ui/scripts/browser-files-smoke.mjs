import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { createWorkbench } from "./workbench-fixture.mjs";
const id = "34e399c1-78fe-4cf1-93b0-d7fef431b211";
const chooser = "61c716bb-5e8f-4a32-b872-cfa099a695ef";
const nativeCalls = [],
  uploads = [];
let saved = [],
  selections = [],
  cancelSave = true;
let holdPicker = false, releasePicker, cancelMode = "success", releaseCancel, finishes = 0, holdUpload = false, releaseUpload;
const output = "../.tmp/ui-migration/browser-files";
await mkdir(output, { recursive: true });
const download = {
  id,
  name: "video.mp4",
  state: "downloading",
  receivedBytes: 3,
  totalBytes: 100,
};
const fixture = await createWorkbench({
  nativeRequest: async (body) => {
    if (body.action === "chooseUploadFiles") { nativeCalls.push(body); if (holdPicker) await new Promise(resolve => { releasePicker = resolve; }); return { fileSelection: "fixture-selection", count: 2 }; }
    if (body.action === "uploadHostFiles") { nativeCalls.push(body); assert.equal(body.target, "browser"); assert.equal(body.id, chooser); assert.equal(body.selection, "fixture-selection"); return {ok:true}; }
    if (body.action === "browserDownloadAction" && body.operation === "save") {
      nativeCalls.push(body);
      if (cancelSave) return { canceled: true };
      saved = [
        { id, name: "视频.mp4", path: "C:/Users/test/Downloads/视频.mp4", saved_at: "2026-11-16T00:00:00Z" },
      ];
      return saved[0];
    }
    if (body.action === "browserDownloadAction") {
      nativeCalls.push(body);
      return { ok: true };
    }
    return null;
  },
});
const { browser, page, state, errors } = fixture;
await page.route("**/api/browser/tabs", (route) =>
  route.fulfill({
    json: {
      tabs: state.tabs,
      active: state.active,
      downloads: [download],
      uploads: selections,
    },
  }),
);
await page.route("**/api/browser/files/saved", (route) =>
  route.fulfill({ json: saved }),
);
await page.route("**/api/input/files/browser/**", async (route) => {
  uploads.push({
    name: new URL(route.request().url()).searchParams.get("name"),
    bytes: route.request().postDataBuffer(),
  });
  if (holdUpload) await new Promise(resolve => { releaseUpload = resolve; });
  await route.fulfill({ json: { ok: true } });
});
await page.route("**/api/browser/upload-finish", (route) => {
  finishes++;
  assert.ok(selections.some(selection => selection.id === route.request().postDataJSON().id));
  selections = [];
  return route.fulfill({ json: { ok: true } });
});
await page.route("**/api/browser/upload-cancel", async (route) => {
  if (cancelMode === "failure") return route.fulfill({ status: 503, contentType: "text/plain; charset=utf-8", body: "上传请求尚未取消，请重试" });
  if (cancelMode === "hold") await new Promise(resolve => { releaseCancel = resolve; });
  selections = [];
  return route.fulfill({ json: { ok: true } });
});
const poll = async (condition) => {
  for (let i = 0; i < 120; i++) {
    if (condition()) return;
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  throw new Error("UI state timeout");
};
try {
  state.status.ready = true;
  state.status.vm_running = true;
  await page.reload();
  await page.getByRole("button", { name: "打开浏览器", exact: true }).click();
  await page.getByRole("button", { name: "浏览器下载", exact: true }).click();
  await page.getByText("正在下载", { exact: false }).waitFor();
  await page
    .getByRole("dialog", { name: "下载文件" })
    .getByRole("button", { name: "关闭面板" })
    .click();
  selections = [{ id: chooser, tabId: "tab-1", multiple: true }];
  await page.getByRole("dialog", { name: "上传本机文件" }).waitFor();
  Object.assign(download, {
    state: "saved",
    path: "C:/Users/test/Workbench/Downloads/video.mp4",
  });
  await page.getByRole("button", { name: "选择本机文件", exact: true }).click();
  await page.getByRole("dialog", { name: "上传本机文件" }).waitFor({ state: "hidden" });
  assert.ok(nativeCalls.some(call => call.action === "uploadHostFiles"));
  await poll(() =>
    nativeCalls.some((call) => call.action === "browserDownloadAction" && call.operation === "save"),
  );
  assert.equal(saved.length, 0, "cancel must not invent a save destination");
  await page.getByRole("button", { name: "浏览器下载", exact: true }).click();
  await page
    .getByText("C:/Users/test/Workbench/Downloads/video.mp4", { exact: true })
    .waitFor();
  cancelSave = false;
  await page.getByRole("button", { name: "另存为…", exact: true }).click();
  await page
    .getByText("C:/Users/test/Downloads/视频.mp4", { exact: true })
    .waitFor();
  await page
    .getByRole("dialog", { name: "下载文件" })
    .getByRole("button", { name: "打开文件", exact: true })
    .click();
  await page
    .getByRole("button", { name: "在文件夹中显示", exact: true })
    .click();
  await poll(
    () =>
      nativeCalls.filter((call) => call.action === "browserDownloadAction" && call.operation !== "save")
        .length === 2,
  );
  assert.deepEqual(
    nativeCalls
      .filter((call) => call.action === "browserDownloadAction" && call.operation !== "save")
      .map((call) => [call.operation, call.id]),
    [
      ["open", id],
      ["reveal", id],
    ],
  );
  assert.ok(
    nativeCalls.every((call) => !("path" in call)),
    "web UI must not send arbitrary host paths",
  );
  await page
    .getByRole("dialog", { name: "下载文件" })
    .getByRole("button", { name: "关闭面板" })
    .click();
  selections = [{ id: "a8af5851-5837-45ca-a820-1a2565bc7ec6", tabId: "tab-1", multiple: false }];
  holdPicker = true;
  cancelMode = "failure";
  const upload = page.getByRole("dialog", { name: "上传本机文件" });
  await upload.getByRole("button", { name: "选择本机文件", exact: true }).click();
  await poll(() => Boolean(releasePicker));
  const uploadCalls = nativeCalls.filter(call => call.action === "uploadHostFiles").length;
  const completed = finishes;
  await page
    .getByRole("dialog", { name: "上传本机文件" })
    .getByRole("button", { name: "取消上传" })
    .click();
  await upload.getByRole("alert").filter({ hasText: "上传请求尚未取消" }).waitFor();
  assert.equal(await upload.isVisible(), true, "a failed cancellation must keep the request visible");
  await upload.screenshot({ path: `${output}/cancel-failed.png` });
  cancelMode = "hold";
  await upload.getByRole("button", { name: "取消上传", exact: true }).click();
  await poll(() => Boolean(releaseCancel));
  assert.equal(await upload.getByRole("button", { name: "正在取消…", exact: true }).isDisabled(), true);
  assert.equal(await upload.isVisible(), true, "pending cancellation was reported as success");
  releaseCancel();
  await page
    .getByRole("dialog", { name: "上传本机文件" })
    .waitFor({ state: "hidden" });
  releasePicker();
  await page.waitForTimeout(100);
  assert.equal(nativeCalls.filter(call => call.action === "uploadHostFiles").length, uploadCalls, "a cancelled native picker must not start uploading");
  assert.equal(finishes, completed, "a cancelled selection must not finish an old upload request");
  const downloadCalls = nativeCalls.filter(call => call.action === "browserDownloadAction").length;
  await page.getByRole("button", { name: "浏览器下载", exact: true }).click();
  await page
    .getByText("C:/Users/test/Downloads/视频.mp4", { exact: true })
    .waitFor();
  await page
    .getByRole("dialog", { name: "下载文件" })
    .getByRole("button", { name: "打开文件", exact: true })
    .click();
  await poll(
    () =>
      nativeCalls.filter((call) => call.action === "browserDownloadAction")
        .length === downloadCalls + 1,
  );
  await page.getByRole("dialog", { name: "下载文件" }).getByRole("button", { name: "关闭面板", exact: true }).click();
  await page.evaluate(() => { delete window.edenRuntimeTransport; });
  selections = [{ id: "browser-file-input", tabId: "tab-1", multiple: true }];
  const input = upload.locator('input[type="file"]');
  await input.waitFor();
  await input.setInputFiles(Array.from({ length: 101 }, (_, index) => ({ name: `file-${index}.txt`, mimeType: "text/plain", buffer: Buffer.from("fixture") })));
  await upload.getByRole("alert").filter({ hasText: "多选最多 100 个文件" }).waitFor();
  assert.equal(uploads.length, 0);
  const payload = Buffer.from("中文文档\n原始内容", "utf8");
  await input.setInputFiles({ name: "中文 文档.txt", mimeType: "text/plain", buffer: payload });
  await upload.waitFor({ state: "hidden" });
  assert.equal(uploads.length, 1);
  assert.equal(uploads[0].name, "中文 文档.txt");
  assert.deepEqual(uploads[0].bytes, payload);
  assert.equal(finishes, completed + 1);
  selections = [{ id: "browser-pending-file-input", tabId: "tab-1", multiple: true }];
  cancelMode = "success";
  holdUpload = true;
  await input.setInputFiles({ name: "pending.txt", mimeType: "text/plain", buffer: Buffer.from("pending") });
  await poll(() => Boolean(releaseUpload));
  await upload.getByRole("button", { name: "取消上传", exact: true }).click();
  await upload.waitFor({ state: "hidden" });
  releaseUpload();
  await page.waitForTimeout(100);
  assert.equal(finishes, completed + 1, "a cancelled browser upload must not finish a stale batch");
  assert.deepEqual(errors, []);
  console.log(
    "PASS: opaque native selection, failed/pending/confirmed cancellation, late picker rejection, browser file bytes/limits/cancellation, deferred Save As, saved path and Open/Reveal IDs",
  );
} finally {
  releaseCancel?.();
  releasePicker?.();
  releaseUpload?.();
  await browser.close();
}
