import assert from "node:assert/strict";
import http from "node:http";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "../../scripts/isolated-vite.mjs";

test("the UI fixture serves frontend resources and blocks an inherited API proxy before it can forward", async (t) => {
  let forwarded = 0;
  const upstream = http.createServer((_request, response) => {
    forwarded++;
    response.end("must not be reached");
  });
  await new Promise((resolve) => upstream.listen(0, "127.0.0.1", resolve));
  t.after(() => new Promise((resolve) => upstream.close(resolve)));
  const server = await createServer({
    configFile: false,
    root: fileURLToPath(new URL("../../", import.meta.url)),
    server: {
      host: "127.0.0.1",
      port: 0,
      proxy: { "/api": `http://127.0.0.1:${upstream.address().port}` },
    },
  });
  await server.listen();
  const origin = `http://127.0.0.1:${server.httpServer.address().port}`;
  assert.equal((await fetch(origin + "/@vite/client")).status, 200);
  assert.equal((await fetch(origin + "/api/status")).status, 500);
  assert.equal(
    (
      await fetch(origin + "/api/diagnostics/events", {
        method: "POST",
        body: "fixture error",
      })
    ).status,
    500,
  );
  assert.equal(forwarded, 0);
  await assert.rejects(
    server.close(),
    /UI fixture left unhandled API requests/,
  );
});
