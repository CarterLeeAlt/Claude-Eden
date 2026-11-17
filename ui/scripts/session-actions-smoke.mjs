// The real desktop UI uses an isolated host fixture. No request reaches personal data.
import assert from "node:assert/strict";
import { join } from "node:path";
import { smokeCaptureDirectory } from "./smoke-captures.mjs";
import { createWorkbench, sessionView, terminalView } from "./workbench-fixture.mjs";

const base = process.env.EDEN_UI_URL || "http://127.0.0.1:5178";
const engine = process.env.EDEN_TEST_BROWSER || "chromium";
const output = await smokeCaptureDirectory("session-actions-" + engine);
const desktop = await createWorkbench({url:base});
const {page, state, browser, errors} = desktop;
const workspace = state.settings.workspace;
const id = "12345678-1234-4321-aaaa-123456789abc";
const sessions = [
  {id, workspace, title:"待处理的会话", revision:1, generation:id, updated_at:"2026-11-07T08:00:00Z", pinned:false},
  {id:"12345678-1234-4321-aaaa-123456789abd", workspace, title:"另一段会话", revision:1, generation:id, updated_at:"2026-11-07T09:00:00Z", pinned:false},
].map(sessionView);
let failSave = false;
const update = (target, body) => {
  if (body.title) target.title = target.custom_title = body.title;
  if (body.pinned !== undefined) target.pinned = body.pinned;
  target.metadata_set = true; target.revision++;
  state.terminals = state.terminals.map(t => t.cli_session_id === target.id ? {...t, title:target.title, custom_title:target.custom_title, pinned:target.pinned, metadata_set:true} : t);
  return terminalView({...target, id:"fixture-shared", kind:"claude", cli_session_id:target.id});
};
const remove = target => {
  sessions.splice(sessions.indexOf(target),1);
  state.terminals = state.terminals.filter(t => t.cli_session_id !== target.id);
};
try {
  state.status.ready = state.status.vm_running = true;
  await page.evaluate(({workspace,title}) => localStorage.setItem("eden-tab-layout",JSON.stringify({[workspace]:{"terminal:fixture-shared":{title,pinned:false}}})),{workspace,title:sessions[0].title});
  state.terminals = [terminalView({id:"fixture-shared", kind:"claude", workspace, title:sessions[0].title, cli_session_id:id, metadata_set:false, pinned:false})];
  await page.route("**/api/terminals/*/metadata", async route => {
    if (failSave) return route.fulfill({status:500,contentType:"text/plain; charset=utf-8",body:"测试：磁盘暂时不可写"});
    return route.fulfill({json:update(sessions[0],route.request().postDataJSON())});
  });
  await page.route("**/api/terminals/*", async route => {
    if (route.request().method() !== "DELETE") return route.fallback();
    remove(sessions[0]); await route.fulfill({json:{ok:true}});
  });
  await page.route("**/api/cli/sessions/*/archive", async route => {
    const target = sessions.find(s => route.request().url().includes(s.id));
    remove(target); await route.fulfill({json:{ok:true}});
  });
  await page.getByRole("tab",{name:/待处理的会话/}).first().hover();
  await page.getByRole("group",{name:"待处理的会话 的操作"}).waitFor();
  assert.equal(sessions[0].metadata_set,true,"desktop local label was not imported into host metadata");
  const actions = page.getByRole("group",{name:"待处理的会话 的操作"});
  await actions.getByRole("button",{name:"置顶",exact:true}).click();
  await actions.getByRole("button",{name:"取消置顶",exact:true}).waitFor();
  await actions.getByRole("button",{name:"重命名",exact:true}).click();
  await page.getByRole("textbox",{name:"标签名称"}).fill("本机会话检查");
  failSave = true;
  await page.getByRole("button",{name:"保存名称",exact:true}).click();
  await page.getByRole("dialog",{name:"重命名标签",exact:true}).getByRole("alert").filter({hasText:"测试：磁盘暂时不可写"}).waitFor();
  await page.getByRole("dialog",{name:"重命名标签",exact:true}).waitFor();
  assert.equal(await page.getByRole("textbox",{name:"标签名称"}).inputValue(),"本机会话检查");
  assert.equal(sessions[0].title,"待处理的会话");
  failSave = false;
  await page.getByRole("button",{name:"保存名称",exact:true}).click();
  await page.getByRole("tab",{name:"本机会话检查",exact:true}).waitFor();
  await page.reload();
  await page.getByRole("tab",{name:"本机会话检查",exact:true}).hover();
  const renamed = page.getByRole("group",{name:"本机会话检查 的操作"});
  await renamed.getByRole("button",{name:"取消置顶",exact:true}).waitFor();
  await page.screenshot({path:join(output,engine+"-desktop.png")});
  await renamed.getByRole("button",{name:"归档",exact:true}).click();
  const dialog=page.getByRole("dialog",{name:"归档这段会话？"});
  await dialog.getByRole("button",{name:"取消",exact:true}).click();
  assert.equal(sessions.length,2);
  await renamed.getByRole("button",{name:"归档",exact:true}).click();
  await dialog.getByRole("button",{name:"归档会话",exact:true}).click();
  await page.getByRole("tab",{name:"本机会话检查",exact:true}).waitFor({state:"hidden"});
  assert.equal(sessions.length,1);
  assert.deepEqual(errors,[]);
  console.log(engine+": desktop pin, rename, failure/retry, archive/cancel, label persistence and reload passed");
} finally { await browser.close(); }
