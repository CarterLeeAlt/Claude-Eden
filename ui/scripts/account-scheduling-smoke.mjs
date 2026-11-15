import assert from "node:assert/strict";
import {createWorkbench} from "./workbench-fixture.mjs";
import {smokeCaptureDirectory} from "./smoke-captures.mjs";

const out=await smokeCaptureDirectory("account-scheduling");
const scheduling={mode:"balanced",switch_policy:"ask"};
let profiles=[{id:"a",label:"账号 A",email:"a@example.invalid",status:"ready",enabled:true},{id:"b",label:"账号 B",email:"b@example.invalid",status:"ready",enabled:true}];
const sid="12345678-1234-4321-aaaa-123456789abc";
  const session={id:sid,workspace:"C:/Users/developer/.claude-eden-workspace",title:"调度验证",terminal_available:true,desktop_terminal_id:"first",can_send:true,revision:1,account_id:"a",account_label:"账号 A",switch_policy:"ask",turns:[]};
let switchCalls=[];
const account=(route,body)=>{
  if(body?.action==="policy")session.switch_policy=body.policy;
  if(body?.action==="switch"){
    switchCalls.push(body);session.account_id="b";session.account_label="账号 B";session.desktop_terminal_id="second";
    session.account_switch={id:body.request_id,phase:"completed",target_label:"账号 B"};
  }
  return route.fulfill({json:{session,source_id:"12345678-1234-4321-aaaa-123456789abd",switch_policy:session.switch_policy,supported:true,candidates:[{id:"b",label:"账号 B"}]}});
};
const fixture=await createWorkbench();
const {browser,page,state,errors}=fixture;
try{
  await page.route("**/api/claude-accounts*",async route=>{
    const body=route.request().postDataJSON();
    if(body?.action==="scheduling"){
      Object.assign(scheduling,body.scheduling);
      if(body.order)profiles=body.order.map(id=>profiles.find(a=>a.id===id));
    }
    if(body?.action==="update")Object.assign(profiles.find(a=>a.id===body.id),body);
    return route.fulfill({json:{accounts:profiles,scheduling,detection_status:"offline"}});
  });

  await page.getByRole("button",{name:"Claude 账号管理",exact:true}).click();
  await page.getByText("调度设置 · 均衡分配",{exact:true}).click();
  const select=page.getByRole("combobox",{name:"新会话分配",exact:true});
  const policy=page.getByRole("combobox",{name:"新会话切换授权",exact:true});
  const before=(await policy.boundingBox()).y;
  await select.click();
  assert.ok((await policy.boundingBox()).y>before+20,"options must occupy layout space");
  await select.press("End");await select.press("Enter");
  await page.getByRole("region",{name:"账号优先顺序"}).count();
  assert.equal(scheduling.mode,"sequential");
  await page.getByRole("button",{name:"提高 账号 B 的优先级"}).click();
  await page.waitForFunction(()=>document.querySelector('.claude-account-order')?.textContent.startsWith('1. 账号 B'));
  assert.equal(profiles[0].id,"b");
  await policy.click();await page.getByRole("option",{name:"额度耗尽后自动接续",exact:true}).click();
  assert.equal(scheduling.switch_policy,"allow");
  await page.getByRole("textbox",{name:"账号 A 的接续分组"}).fill("本人账号");
  await page.getByRole("article",{name:"账号 A",exact:true}).getByRole("button",{name:"保存分组"}).click();
  assert.equal(profiles.find(a=>a.id==="a").handoff_group,"本人账号");
  await page.screenshot({path:`${out}/scheduling-desktop.png`});
  await page.reload();
  await page.getByRole("button",{name:"Claude 账号管理",exact:true}).click();
  await page.getByText("调度设置 · 顺序优先",{exact:true}).waitFor();

  await page.getByRole("button",{name:"工作台",exact:true}).click();
  state.status.ready=state.status.vm_running=true;
  state.terminals=[{id:"first",kind:"claude",workspace:session.workspace,title:session.title,cli_session_id:sid}];
  await page.route("**/api/cli/sessions",route=>route.fulfill({json:{sessions:[session]}}));
  await page.route(`**/api/cli/sessions/${sid}/account`,route=>account(route,route.request().postDataJSON()));
  await page.getByRole("tab",{name:session.title,exact:true}).click();
  await page.getByRole("button",{name:"账号 A · 会话绑定",exact:true}).click();
  const switchButton=page.getByRole("button",{name:"切换并恢复历史",exact:true});
  assert.equal(await switchButton.isDisabled(),true);
  await page.getByRole("combobox",{name:"此会话切换授权"}).click();
  await page.getByRole("option",{name:"始终等待原账号",exact:true}).click();
  assert.equal(session.switch_policy,"wait");
  await page.getByRole("checkbox",{name:"允许将此会话历史交给所选账号"}).check();
  await switchButton.click();
  await page.getByRole("button",{name:"账号 B · 会话绑定",exact:true}).waitFor();
  assert.equal(switchCalls.length,1);assert.equal(switchCalls[0].confirm,true);assert.equal(switchCalls[0].target,"b");
  assert.equal(switchCalls[0].continue,false,"switching must not imply task continuation");
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await page.screenshot({path:`${out}/session-account-desktop.png`});
  assert.deepEqual(errors,[]);
  console.log("Scheduling: inline mouse/keyboard selection, ordering, group persistence, session authorization and explicit desktop switch passed.");
}finally{await browser.close();}
