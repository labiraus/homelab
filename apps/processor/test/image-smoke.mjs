// Runs via stdin inside the published image with Docker networking disabled.
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";

const raw = readFileSync("package-lock.json");
assert.equal(createHash("sha256").update(raw).digest("hex"), process.env.EXPECTED_LOCK_SHA, "image lockfile differs from the tested source");
const lock = JSON.parse(raw);
let checked = 0;
for (const [path, entry] of Object.entries(lock.packages)) {
  if (!path.startsWith("node_modules/") || !existsSync(`${path}/package.json`)) continue;
  const installed = JSON.parse(readFileSync(`${path}/package.json`, "utf8"));
  assert.equal(installed.version, entry.version, `unlocked dependency: ${path}`);
  assert.notEqual(entry.dev, true, `development-only dependency in runtime: ${path}`);
  checked++;
}
assert.ok(checked > 0, "runtime dependencies missing");
const tree = spawnSync("npm", ["ls", "--omit=dev", "--all", "--json", "--offline"], { encoding: "utf8" });
assert.equal(tree.status, 0, tree.stderr || tree.stdout);
await import("./dist/src/storage.js");
await import("./dist/src/worldbuilder.js");
const { parseWorldJSON } = await import("./dist/src/worldTicks.js");
assert.equal(parseWorldJSON('{"from":9223372036854775807}').from, "9223372036854775807");
console.log(`Verified ${checked} locked runtime packages, compiled application imports and exact dates on ${process.version}.`);
