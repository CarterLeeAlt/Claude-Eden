import test from "node:test";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { cp, mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";

// Exercise the real controller while a pipe peer delays Chromium's final exit.
// The peer releases an in-flight startup reply after Browser.close, reproducing
// the ordering seen during a cold browser shutdown.
const cases = ["Browser.getVersion", "Target.setDiscoverTargets"].flatMap(paused =>
  ["clean", "failure", "signal"].map(outcome => ({ paused, outcome })));
cases.push(
  { paused: "Page.enable", outcome: "clean", delayed: true },
  { paused: "Page.enable", outcome: "clean", startupFailure: true },
);
for (const { paused, outcome, delayed, startupFailure } of cases) {
  const reason = delayed ? "slow restored renderer" : startupFailure ? "startup failure" : outcome;
  test(`shutdown during ${paused} waits for the browser with ${reason} and sends no new work`, { skip: process.platform !== "linux", timeout: delayed ? 50000 : 20000 }, async () => {
    const directory = await mkdtemp(join(tmpdir(), "workbench-shutdown-"));
    const peer = join(directory, "peer.mjs");
    let agent, done, ended = false, stderr = "";
    const read = async name => {
      try { return await readFile(join(directory, name), "utf8"); }
      catch (error) { if (error.code !== "ENOENT") throw error; return ""; }
    };
    const waitFor = async (name, timeout = 5000) => {
      const deadline = Date.now() + timeout;
      while (!await read(name)) {
        assert.equal(ended, false, stderr);
        assert.ok(Date.now() < deadline, `Timed out waiting for ${name}: ${stderr}`);
        await delay(20);
      }
    };
    try {
      const sourceDirectory = new URL("./", import.meta.url);
      for (const name of await readdir(sourceDirectory))
        if (name.endsWith(".mjs") && !name.endsWith(".test.mjs"))
          await cp(new URL(name, sourceDirectory), join(directory, name));
      const original = await readFile(new URL("browser-agent.mjs", sourceDirectory), "utf8");
      assert.ok(original.includes("spawn(browserBinary, flags"));
      const source = original.replaceAll("/home/browser", directory)
        .replace("/etc/workbench/browser-control.json", join(directory, "control.json"))
        .replace("spawn(browserBinary, flags", `spawn(process.execPath, [${JSON.stringify(peer)}]`)
        .replace('server.listen(18500, "127.0.0.1");', 'server.listen(0, "127.0.0.1");');
      await writeFile(join(directory, "browser-agent.mjs"), source);
      await writeFile(join(directory, "control.json"), JSON.stringify({ token: "fixture", workspace: directory }));
      await writeFile(peer, `
import { writeFileSync, writeSync, existsSync, appendFileSync } from 'node:fs';
import { Socket } from 'node:net';
import { join } from 'node:path';
const directory=${JSON.stringify(directory)}, paused=${JSON.stringify(paused)}, outcome=${JSON.stringify(outcome)};
const reply=message=>writeSync(4,JSON.stringify(message)+'\\0');
let buffer='', pending;
const editor={targetId:'restored-page',type:'page',url:'https://example.test/'};
const failure=setInterval(()=>{
  if(pending&&existsSync(join(directory,'startup-failure'))){
    clearInterval(failure);reply({id:pending.id,error:{code:-32000,message:'fixture initialization failure'}});
  }
},20);
new Socket({fd:3,readable:true,writable:false}).on('data',bytes=>{
  buffer+=bytes.toString();let end;
  while((end=buffer.indexOf('\\0'))>=0){
    const message=JSON.parse(buffer.slice(0,end));buffer=buffer.slice(end+1);
    appendFileSync(join(directory,'commands.jsonl'),JSON.stringify(message)+'\\n');
    if(message.method==='Browser.close'){
      reply({id:message.id,result:{}});
      reply({id:pending.id,result:{}});
      reply({method:'Target.targetCreated',params:{targetInfo:{targetId:'late-page',type:'page',url:'about:blank'}}});
      writeFileSync(join(directory,'closing'),'yes');
      const timer=setInterval(()=>{if(existsSync(join(directory,'release'))){
        clearInterval(timer);
        if(outcome==='signal')process.kill(process.pid,'SIGKILL');
        else process.exit(outcome==='failure'?7:0);
      }},20);
    }else if(message.method===paused){pending=message;writeFileSync(join(directory,'paused'),'yes');}
    else if(message.method==='Target.getTargets')reply({id:message.id,result:{targetInfos:[editor]}});
    else if(message.method==='Target.attachToTarget')reply({id:message.id,result:{sessionId:'page-session'}});
    else if(message.method==='Browser.getWindowForTarget')reply({id:message.id,result:{windowId:1,bounds:{windowState:'normal'}}});
    else if(message.method==='Browser.getWindowBounds')reply({id:message.id,result:{bounds:{windowState:'fullscreen'}}});
    else reply({id:message.id,result:{}});
  }
});
`);
      agent = spawn(process.execPath, [join(directory, "browser-agent.mjs")], { stdio: ["ignore", "pipe", "pipe"] });
      agent.stderr.on("data", bytes => { stderr += bytes; });
      done = once(agent, "close");
      agent.once("exit", () => { ended = true; });
      await waitFor("paused");
      if (delayed) {
        await delay(31000);
        assert.equal(ended, false, stderr);
        assert.equal(await read("closing"), "", "A slow restored renderer must remain available for completion or cancellation");
      }
      if (startupFailure) await writeFile(join(directory, "startup-failure"), "yes");
      else agent.kill("SIGTERM");
      await waitFor("closing");
      await delay(2200);
      assert.equal(ended, false, "Controller exited while Chromium still owned the profile");
      const commands = (await read("commands.jsonl")).trim().split("\n").map(line => JSON.parse(line).method);
      assert.equal(commands.filter(method => method === "Browser.close").length, 1);
      assert.equal(commands.at(-1), "Browser.close", "Startup or target events issued work after shutdown");
      await writeFile(join(directory, "release"), "yes");
      assert.deepEqual(await done, [outcome === "clean" && !startupFailure ? 0 : 1, null], stderr);
      if (outcome !== "clean") assert.match(stderr, /Browser exited abnormally/);
      if (startupFailure) assert.match(stderr, /Browser startup failed: fixture initialization failure/);
    } finally {
      await writeFile(join(directory, "release"), "yes");
      if (agent && !ended) agent.kill("SIGTERM");
      if (done) await done;
      await rm(directory, { recursive: true, force: true });
    }
  });
}
