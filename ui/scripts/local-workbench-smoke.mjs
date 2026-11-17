import assert from "node:assert/strict";
import { createWorkbench } from "./workbench-fixture.mjs";
import { smokeCaptureDirectory } from "./smoke-captures.mjs";

const output = await smokeCaptureDirectory("local-workbench");
for (const scenario of ["connect", "slow-connect", "prepare-failed", "candidate-failed", "commit-failed", "cancel", "cancel-failed", "lost-active", "context-failed", "invalid-context"]) {
  const selected = { mode: scenario === "lost-active" ? "local" : "none", desktop_gateway: true, client_platform: "windows", server_platform: "", connection_id: "initial", connection: { state: scenario === "lost-active" ? "failed" : "disconnected", message: "本机工作台连接已断开", error_code: "connection_lost" } };
  let rejectPrepare = scenario === "prepare-failed", rejectCommit = scenario === "commit-failed", rejectCancel = scenario === "cancel-failed";
  let rejectContext = scenario === "context-failed", invalidContext = scenario === "invalid-context";
  let preparations = 0, commits = 0, cancellations = 0, holdContext = false, releaseContext;
  const calls = [];
  const fixture = await createWorkbench({
    waitForReady: false,
    clientContext: selected,
    clientRequest: async (route, path, body) => {
      calls.push({ path, body });
      if (path === "/client/context") {
        if (rejectContext) return route.fulfill({ status: 503, contentType: "text/plain; charset=utf-8", body: "本机状态读取失败" });
        if (invalidContext) return route.fulfill({ json: { ...selected, mode: "invalid" } });
        const snapshot = structuredClone(selected);
        if (holdContext) { holdContext = false; await new Promise(resolve => { releaseContext = resolve; }); }
        return route.fulfill({ json: snapshot });
      }
      if (path === "/client/candidate") {
        preparations++;
        assert.deepEqual(body, { connection_id: selected.connection_id, mode: "local" });
        if (rejectPrepare) return route.fulfill({ status: 503, contentType: "text/plain; charset=utf-8", body: "本机运行组件不可用" });
        selected.candidate = { id: `candidate-${preparations}`, mode: "local", connection: { state: scenario === "candidate-failed" && preparations === 1 ? "failed" : "connecting", message: "连接测试失败" } };
        return route.fulfill({ json: selected.candidate });
      }
      if (path === "/client/commit") {
        commits++;
        assert.deepEqual(body, { connection_id: selected.connection_id, candidate_id: selected.candidate.id, local_action: "auto" });
        if (rejectCommit) return route.fulfill({ status: 409, contentType: "text/plain; charset=utf-8", body: "候选工作台连接已中断" });
        selected.mode = "local";
        selected.server_platform = "windows";
        selected.connection_id = selected.candidate.id;
        selected.connection = { state: "connected" };
        delete selected.candidate;
        return route.fulfill({ json: { ok: true } });
      }
      if (path === "/client/cancel") {
        cancellations++;
        assert.deepEqual(body, { connection_id: selected.connection_id, candidate_id: selected.candidate.id });
        if (rejectCancel) return route.fulfill({ status: 503, contentType: "text/plain; charset=utf-8", body: "取消连接失败" });
        delete selected.candidate;
        return route.fulfill({ json: { ok: true } });
      }
      return route.fulfill({ json: {} });
    },
    beforeNavigate: async page => {
      if (scenario === "slow-connect") await page.clock.install();
      await page.addInitScript(() => localStorage.setItem("eden.audio", JSON.stringify({ muted: true })));
    },
  });
  const { page, browser, errors, state } = fixture;
  const reconnect = page.getByRole("button", { name: "重新连接本机", exact: true });
  const cancel = page.getByRole("button", { name: "取消此次连接", exact: true });
  const waitForCandidate = async () => {
    await cancel.waitFor();
    await page.waitForFunction(() => [...document.querySelectorAll("button")].some(button => button.textContent === "取消此次连接" && !button.disabled));
  };
  try {
    if (scenario === "invalid-context" || scenario === "context-failed") {
      await page.getByRole("alert").waitFor();
      assert.equal(preparations, 0);
      invalidContext = rejectContext = false;
      await page.getByRole("button", { name: "重新加载", exact: true }).click();
    } else if (scenario === "lost-active") {
      await page.getByRole("alert").filter({ hasText: "连接已断开" }).waitFor();
      assert.equal(preparations, 0, "lost active connection must wait for an explicit reconnect");
      await reconnect.click();
    } else if (scenario === "prepare-failed") {
      await page.getByRole("alert").filter({ hasText: "本机运行组件不可用" }).waitFor();
      assert.equal(preparations, 1);
      rejectPrepare = false;
      await reconnect.click();
    } else if (scenario === "candidate-failed") {
      await page.getByRole("alert").filter({ hasText: "连接测试失败" }).waitFor();
      assert.equal(preparations, 1);
      await reconnect.click();
      assert.equal(cancellations, 1, "manual retry must cancel the old candidate first");
    }
    await waitForCandidate();
    if (scenario === "slow-connect") {
      await page.clock.fastForward(31 * 60 * 1000);
      await waitForCandidate();
      assert.equal(commits, 0, "elapsed time must not commit or fail a connecting candidate");
      assert.equal(await page.getByRole("alert").count(), 0);
      await page.clock.resume();
    }
    if (scenario === "cancel" || scenario === "cancel-failed") {
      if (scenario === "cancel") {
        selected.candidate.connection.state = "connected";
        holdContext = true;
        const deadline = Date.now() + 3000;
        while (!releaseContext && Date.now() < deadline) await page.waitForTimeout(25);
        assert.ok(releaseContext, "candidate polling did not start");
      }
      await cancel.click();
      if (scenario === "cancel-failed") {
        await page.getByRole("alert").filter({ hasText: "取消连接失败" }).waitFor();
        rejectCancel = false;
        await cancel.click();
      }
      await reconnect.waitFor();
      releaseContext?.();
      await page.waitForTimeout(450);
      assert.equal(commits, 0, "a late connected result must not commit a cancelled candidate");
      assert.equal(preparations, 1, "cancel must not trigger another automatic connection");
      await page.screenshot({ path: `${output}/${scenario}.png` });
      await reconnect.click();
      await waitForCandidate();
    }
    selected.candidate.connection.state = "connected";
    if (scenario === "commit-failed") {
      await page.getByRole("alert").filter({ hasText: "候选工作台连接已中断" }).waitFor();
      assert.equal(commits, 1);
      rejectCommit = false;
      await reconnect.click();
      await waitForCandidate();
      selected.candidate.connection.state = "connected";
    }
    await page.getByRole("complementary", { name: "项目与终端" }).waitFor();
    assert.equal(await page.getByRole("button", { name: "SSH 主机", exact: true }).count(), 0);
    await page.screenshot({ path: `${output}/${scenario}-connected.png` });
    state.status.ready = state.status.vm_running = true;
    await page.getByRole("heading", { name: "Claude Eden 环境已就绪", exact: true }).waitFor();
    await page.waitForFunction(() => Boolean(window.edenWorkbenchAction));
    await page.evaluate(() => window.emitTestDesktopEvent("eden-native-action", "quit"));
    const quit = page.getByRole("dialog", { name: "退出 Claude Eden", exact: true });
    await quit.getByRole("button", { name: "取消", exact: true }).click();
    assert.ok(!calls.some(call => ["/client/hosts", "/client/connect"].includes(call.path)));
    assert.equal(state.requests.filter(request => request.path === "/start").length, 0, "connecting must not install or start the guest");
    assert.deepEqual(errors, []);
    console.log(`PASS ${scenario}: local candidate/commit/cancel, generation reload, explicit recovery and native menu`);
  } finally {
    releaseContext?.();
    await browser.close();
  }
}
