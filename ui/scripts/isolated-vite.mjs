import assert from "node:assert/strict";
import { createServer as createViteServer } from "vite";

// Unhandled requests in a UI fixture must never reach a running controller.
export async function createServer(options) {
  const unhandled = [];
  const server = await createViteServer({
    ...options,
    plugins: [
      ...(options.plugins || []),
      {
        name: "isolated-ui-api-boundary",
        configResolved(config) {
          config.server.proxy = undefined;
        },
        configureServer(server) {
          server.middlewares.use((request, response, next) => {
            if (!/^\/api(?:\/|\?|$)/.test(request.url || "")) return next();
            unhandled.push(`${request.method} ${request.url}`);
            response.writeHead(500, {
              "Content-Type": "text/plain; charset=utf-8",
            });
            response.end("Unhandled API request in isolated UI fixture");
          });
        },
      },
    ],
  });
  const close = server.close.bind(server);
  server.close = async () => {
    await close();
    assert.deepEqual(unhandled, [], "UI fixture left unhandled API requests");
  };
  return server;
}
