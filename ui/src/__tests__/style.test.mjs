import test from "node:test";
import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";

test("host styles share the four scalable typography tokens", async () => {
  const directory = new URL("../styles/", import.meta.url);
  const hardcoded = [];
  for (const name of await readdir(directory)) {
    if (!name.endsWith(".css")) continue;
    const source = await readFile(new URL(name, directory), "utf8");
    for (const match of source.matchAll(/font-size:\s*([^;]+);/g)) {
      if (!/^(?:var\(|inherit|0$)/.test(match[1]))
        hardcoded.push(`${name}: ${match[0]}`);
    }
    assert.ok(
      !/font-family:[^;]*(?:Inter|Times|Georgia|serif\s*;)/.test(
        source.replaceAll("sans-serif", "sans"),
      ),
      name,
    );
  }
  assert.deepEqual(hardcoded, []);
  const source = await readFile(new URL("style.css", directory), "utf8");
  for (const token of [
    "--text-body: 1rem",
    "--text-help: 0.9286rem",
    "--text-group: 1.1429rem",
    "--text-page: 1.4286rem",
  ])
    assert.ok(source.includes(token), token);
});
