import assert from "node:assert/strict";
import { chooseInline, createWorkbench } from "./workbench-fixture.mjs";

let socket;
const replies = [];
const { browser, page, state, errors } = await createWorkbench({
  mockWebSockets: false,
  beforeNavigate: async (page, state) => {
    state.status.ready = state.status.vm_running = true;
    await page.routeWebSocket("**/api/**", (remote) => {
      if (!remote.url().includes("/terminals/")) return remote.close();
      socket = remote;
      remote.onMessage((message) => {
        const value = JSON.parse(message);
        if (value.type === "input") replies.push(value.data);
      });
      remote.send(Buffer.from("TERMINAL_READY"));
    });
  },
});
const openSettings = async () => {
  await page.getByRole("button", { name: /^设置/ }).click();
  await page.getByRole("button", { name: "外观", exact: true }).click();
};
const saved = () =>
  page
    .locator(".appearance-save-state")
    .filter({ hasText: "更改自动保存，即时生效" })
    .waitFor();
const back = () =>
  page.getByRole("button", { name: "返回工作台", exact: true }).click();
const openTerminal = async () => {
  await page
    .getByRole("button", { name: "新建终端", exact: true })
    .first()
    .click();
  await page.getByRole("menuitem", { name: "普通终端", exact: true }).click();
  await page
    .locator(".xterm-rows")
    .getByText("TERMINAL_READY", { exact: true })
    .last()
    .waitFor();
};
const snapshot = () =>
  page.evaluate(() => {
    const rows = [...document.querySelectorAll(".xterm-rows")].find((row) =>
      row.textContent.includes("LOW_CONTRAST"),
    );
    if (!rows) throw new Error("Missing terminal output");
    const read = (text) => {
      const element = [...rows.querySelectorAll("span")].find(
        (span) => span.textContent === text,
      );
      if (!element) throw new Error(`Missing terminal text: ${text}`);
      const style = getComputedStyle(element);
      return {
        color: style.color,
        weight: style.fontWeight,
        italic: style.fontStyle,
      };
    };
    return {
      background: getComputedStyle(
        rows.closest(".xterm").querySelector(".xterm-scrollable-element"),
      ).backgroundColor,
      normal: read("NORMAL"),
      red: read("RED"),
      bold: read("BOLD"),
      italic: read("ITALIC"),
      low: read("LOW_CONTRAST"),
    };
  });
try {
  await openTerminal();
  assert.equal(
    await page
      .locator(".xterm-scrollable-element")
      .first()
      .evaluate((el) => getComputedStyle(el).backgroundColor),
    "rgb(0, 0, 0)",
  );
  socket.send(
    Buffer.from(
      "\x1b]10;#dddddd\x07\x1b]11;#181818\x07\x1b]12;#aabbcc\x07\x1b]4;1;#663399\x07" +
        "\x1b[2J\x1b[HNORMAL\r\n\x1b[31mRED\r\n\x1b[0;1mBOLD\r\n\x1b[0;3mITALIC\r\n\x1b[0;38;2;34;34;34mLOW_CONTRAST\r\n\x1b[0mCURSOR",
    ),
  );
  await page
    .locator(".xterm-rows")
    .getByText("LOW_CONTRAST", { exact: true })
    .waitFor();
  const expected = await snapshot();
  assert.equal(expected.background, "rgb(24, 24, 24)");
  assert.equal(expected.normal.color, "rgb(221, 221, 221)");
  assert.equal(expected.red.color, "rgb(102, 51, 153)");
  assert.equal(
    expected.low.color,
    "rgb(34, 34, 34)",
    "Terminal color must not be changed by Eden contrast correction",
  );
  assert.equal(expected.bold.weight, "700");
  assert.equal(expected.italic.italic, "italic");
  const connections = state.terminalConnections;
  for (const preset of ["solarized", "high-contrast", "codex"]) {
    await openSettings();
    await chooseInline(
      page.getByLabel("界面配色预设", { exact: true }),
      preset,
    );
    await saved();
    await back();
    assert.deepEqual(await snapshot(), expected);
  }
  await openSettings();
  await page.getByLabel("代码字号", { exact: true }).fill("15");
  await saved();
  await back();
  assert.deepEqual(await snapshot(), expected);
  assert.equal(state.terminalConnections, connections);
  for (const scheme of ["dark", "light"]) {
    await page.emulateMedia({ colorScheme: scheme });
    assert.equal(
      await page.evaluate(
        () => getComputedStyle(document.documentElement).colorScheme,
      ),
      "light",
    );
    assert.deepEqual(await snapshot(), expected);
  }
  const row = await page
    .locator(".xterm-rows")
    .getByText("NORMAL", { exact: true })
    .boundingBox();
  await page.mouse.move(row.x + 1, row.y + row.height / 2);
  await page.mouse.down();
  await page.mouse.move(row.x + row.width - 1, row.y + row.height / 2, {
    steps: 5,
  });
  await page.mouse.up();
  const selection = await page
    .locator(".xterm-selection div")
    .first()
    .evaluate((el) => getComputedStyle(el).backgroundColor);
  await openSettings();
  await chooseInline(page.getByLabel("界面配色预设", { exact: true }), "sepia");
  await saved();
  await back();
  await page.locator(".xterm-helper-textarea").first().focus();
  assert.equal(
    await page
      .locator(".xterm-selection div")
      .first()
      .evaluate((el) => getComputedStyle(el).backgroundColor),
    selection,
  );
  socket.send(
    Buffer.from("\x1b]10;?\x07\x1b]11;?\x07\x1b]12;?\x07\x1b]4;1;?\x07"),
  );
  await page.waitForFunction(() => document.querySelector(".xterm.focus"));
  const deadline = Date.now() + 3000;
  while (replies.length < 4 && Date.now() < deadline)
    await new Promise((resolve) => setTimeout(resolve, 10));
  for (const reply of [
    "10;rgb:dddd/dddd/dddd",
    "11;rgb:1818/1818/1818",
    "12;rgb:aaaa/bbbb/cccc",
    "4;1;rgb:6666/3333/9999",
  ])
    assert.ok(
      replies.join("").includes(reply),
      `Missing OSC reply ${reply}: ${JSON.stringify(replies)}`,
    );
  socket.send(
    Buffer.from("\x1b]110\x07\x1b]111\x07\x1b]112\x07\x1b]104;1\x07"),
  );
  await page.waitForFunction(
    () =>
      getComputedStyle(document.querySelector(".xterm-scrollable-element"))
        .backgroundColor === "rgb(0, 0, 0)",
  );
  await page.reload();
  await page.getByRole("button", { name: /^设置/ }).waitFor();
  assert.equal(state.appearance.lightTheme.preset, "sepia");
  await openTerminal();
  assert.equal(
    await page
      .locator(".xterm-scrollable-element")
      .last()
      .evaluate((el) => getComputedStyle(el).backgroundColor),
    "rgb(0, 0, 0)",
  );
  assert.deepEqual(errors, []);
  console.log(
    "PASS: terminal OSC/default/ANSI/truecolor/bold/italic/cursor/selection stay independent of UI palettes, size, OS color scheme and reload.",
  );
} finally {
  await browser.close();
}
