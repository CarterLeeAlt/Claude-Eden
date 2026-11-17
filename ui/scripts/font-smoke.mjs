import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { createWorkbench } from "./workbench-fixture.mjs";

const output = path.resolve(
  import.meta.dirname,
  "../../.tmp/ui-migration/font-rendering",
);
await fs.mkdir(output, { recursive: true });
const results = [];
for (const dpr of [1, 1.25, 1.5, 2]) {
  let socket;
  const fixture = await createWorkbench({
    deviceScaleFactor: dpr,
    mockWebSockets: false,
    beforeNavigate: async (page, state) => {
      await page.clock.setFixedTime(new Date("2026-11-16T04:00:00Z"));
      state.appearance.reducedMotion = "on";
      state.account.username = "字体预览";
      await page.routeWebSocket("**/api/**", (remote) => {
        if (!remote.url().includes("/terminals/")) return remote.close();
        socket = remote;
        remote.send(
          Buffer.from(
            "\x1b[2J\x1b[H" +
              [
                "\x1b[1mClaude Eden\x1b[0m  /  中文开发工作区",
                "",
                "$ npm run build",
                "✓ 构建完成，2186 个模块已处理。",
                "✓ Build completed successfully.",
                "",
                'const project = "Claude Eden";',
                'const message = "保存设置，然后继续工作。";',
                "",
                "中文标点：，。；：！？（）「」《》",
                "易混字符：0 O o / 1 I l | / {} [] () <>",
                "运算符号：=> != === <= >= ++ -- && ||",
                "",
                "┌──────────────────┬──────────┬────────────┐",
                "│ 组件             │ 状态     │ 用时       │",
                "├──────────────────┼──────────┼────────────┤",
                "│ Proxy / 代理     │ 已连接   │ 128 ms     │",
                "│ Editor / 编辑器  │ 已就绪   │ 2.84 s     │",
                "└──────────────────┴──────────┴────────────┘",
                "",
                "以上为固定示例文本，由当前终端组件实际渲染。",
                "",
                "$ ",
              ].join("\r\n") +
              "\x1b[?25l",
          ),
        );
      });
    },
  });
  const { page, browser, errors } = fixture;
  try {
    const cdp = await page.context().newCDPSession(page);
    await cdp.send("DOM.enable");
    await cdp.send("CSS.enable");
    const fontsFor = async (selector) => {
      const { root } = await cdp.send("DOM.getDocument");
      const { nodeId } = await cdp.send("DOM.querySelector", {
        nodeId: root.nodeId,
        selector,
      });
      assert.ok(nodeId, selector);
      return (await cdp.send("CSS.getPlatformFontsForNode", { nodeId })).fonts;
    };
    const checkFonts = (fonts, expected) => {
      assert.ok(
        fonts.some((font) => font.familyName.startsWith(expected)),
        JSON.stringify(fonts),
      );
      assert.ok(
        fonts.every((font) => font.isCustomFont),
        `System font used: ${JSON.stringify(fonts)}`,
      );
    };
    const shot = async (scene, terminal = false) => {
      if (dpr !== 1) return;
      const screenshot = (
        await page.screenshot({ animations: "disabled", caret: "hide" })
      ).toString("base64");
      const labels = [
        "界面中英文、数字：Noto Sans SC   |   界面基准：14 px（亮色）",
        terminal
          ? "等宽正文中文：Noto Sans Mono CJK SC   |   英文、数字：JetBrains Mono   |   字号：13 px"
          : "应用内置字体；当前软件界面实际渲染。",
      ];
      const image = await page.evaluate(
        async ({ screenshot, labels }) => {
          const bitmap = new Image();
          bitmap.src = `data:image/png;base64,${screenshot}`;
          await bitmap.decode();
          const canvas = document.createElement("canvas");
          canvas.width = bitmap.width;
          canvas.height = bitmap.height + 66;
          const ctx = canvas.getContext("2d");
          ctx.fillStyle = "#f4f5f6";
          ctx.fillRect(0, 0, canvas.width, 66);
          ctx.fillStyle = "#202020";
          ctx.font = '14px "Noto Sans SC"';
          labels.forEach((label, index) =>
            ctx.fillText(label, 16, 24 + index * 24),
          );
          ctx.drawImage(bitmap, 0, 66);
          return canvas.toDataURL("image/png").split(",")[1];
        },
        { screenshot, labels },
      );
      const filename = terminal
        ? `UI_Noto_Sans_SC_14px_Terminal_Noto_Sans_Mono_CJK_SC_JetBrains_Mono_13px_${scene}.png`
        : `UI_Noto_Sans_SC_14px_Light_${scene}.png`;
      await fs.writeFile(
        path.join(output, filename),
        Buffer.from(image, "base64"),
      );
    };
    assert.equal(
      await page.evaluate(
        () => getComputedStyle(document.documentElement).fontSize,
      ),
      "14px",
    );
    await shot("Home");
    await page.getByRole("button", { name: /^设置/ }).click();
    await page.getByRole("button", { name: "外观", exact: true }).click();
    const ui = {
      cn: await fontsFor(".settings-sidebar h2"),
      en: await fontsFor(".theme-card-heading .inline-select-trigger > span"),
    };
    checkFonts(ui.cn, "Noto Sans SC");
    checkFonts(ui.en, "Noto Sans SC");
    await shot("Appearance");
    for (const size of [12, 24, 14]) {
      await page.getByLabel("界面字号", { exact: true }).fill(String(size));
      await page
        .locator(".appearance-save-state")
        .filter({ hasText: "更改自动保存，即时生效" })
        .waitFor();
      const clipped = await page
        .locator('.theme-color-field input[type="text"]')
        .evaluateAll((inputs) =>
          inputs
            .map((input) => {
              const style = getComputedStyle(input),
                ctx = document.createElement("canvas").getContext("2d");
              ctx.font = style.font;
              return {
                value: input.value,
                required: ctx.measureText(input.value).width,
                available:
                  input.getBoundingClientRect().width -
                  parseFloat(style.paddingLeft) -
                  parseFloat(style.paddingRight) -
                  parseFloat(style.borderLeftWidth) -
                  parseFloat(style.borderRightWidth),
              };
            })
            .filter((item) => item.required > item.available + 1),
        );
      assert.deepEqual(clipped, []);
    }
    await page.getByRole("button", { name: "返回工作台", exact: true }).click();
    await page
      .getByRole("button", { name: "启动 Claude Eden 环境", exact: true })
      .click();
    fixture.state.status.job.log =
      "Starting VM 'product-deb-01'…\n  Resources: CPUs: 4, Memory: 4 GiB, Disk: 24 GiB\n  Base VM: built 2026-11-15\n正在启动浏览器、编辑器与共享文件。";
    fixture.state.status.job.message = "正在启动浏览器、编辑器与共享文件…";
    await page.getByRole("button", { name: "操作详情", exact: true }).click();
    await page
      .getByRole("heading", { name: "操作详情", exact: true })
      .waitFor();
    await page.locator(".job-log").filter({ hasText: "Starting VM" }).waitFor();
    const log = await fontsFor(".job-log");
    checkFonts(log, "Noto Sans Mono CJK SC");
    checkFonts(log, "JetBrains Mono");
    checkFonts(await fontsFor(".modal header h2"), "Noto Sans SC");
    assert.equal(
      await page
        .locator(".job-log")
        .evaluate((el) => getComputedStyle(el).fontSize),
      "13px",
    );
    await shot("Operation_Details", true);
    await page.keyboard.press("Escape");
    await page
      .getByRole("button", { name: "新建终端", exact: true })
      .first()
      .click();
    await page.getByRole("menuitem", { name: "普通终端", exact: true }).click();
    await page
      .locator(".xterm-rows > div")
      .filter({ hasText: "中文标点" })
      .evaluate((el) => (el.id = "font-terminal-cn"));
    await page
      .locator(".xterm-rows > div")
      .filter({ hasText: "const project" })
      .evaluate((el) => (el.id = "font-terminal-en"));
    const terminal = {
      cn: await fontsFor("#font-terminal-cn"),
      en: await fontsFor("#font-terminal-en"),
    };
    checkFonts(terminal.cn, "Noto Sans Mono CJK SC");
    checkFonts(terminal.en, "JetBrains Mono");
    assert.equal(
      await page
        .locator(".xterm-char-measure-element")
        .first()
        .evaluate((el) => getComputedStyle(el).fontSize),
      "13px",
    );
    await shot("Default_Terminal", true);
    socket.send(Buffer.from("\x1b]10;#202020\x07\x1b]11;#fafafa\x07"));
    await page.waitForFunction(
      () =>
        getComputedStyle(document.querySelector(".xterm-scrollable-element"))
          .backgroundColor === "rgb(250, 250, 250)",
    );
    await shot("OSC_Light_Terminal", true);
    assert.deepEqual(errors, []);
    results.push({ dpr, ui, log, terminal });
  } finally {
    await browser.close();
  }
}
await fs.writeFile(
  path.join(output, "font-loading.json"),
  JSON.stringify(results, null, 2),
);
console.log(
  "PASS: three bundled font families actually render UI and terminal text at 100/125/150/200% DPI; font sizes and complete hex inputs verified.",
);
