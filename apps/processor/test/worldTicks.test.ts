import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import type { AddressInfo } from "node:net";
import { exactTick, parseWorldJSON } from "../src/worldTicks.js";
import {
  handleCanonRequest,
  runCanon,
  type CanonRequest,
} from "../src/worldbuilder.js";

test("HTTP reasoning carries exact evidence into the provider and back as candidate strings", async (t) => {
  const saved = {
    token: process.env.WORLDBUILDER_AI_TOKEN,
    url: process.env.AI_GATEWAY_BASE_URL,
  };
  process.env.WORLDBUILDER_AI_TOKEN = "offline-exact-tick-test";
  process.env.AI_GATEWAY_BASE_URL = "http://fixture.invalid/v1";
  t.after(() => {
    if (saved.token === undefined) delete process.env.WORLDBUILDER_AI_TOKEN;
    else process.env.WORLDBUILDER_AI_TOKEN = saved.token;
    if (saved.url === undefined) delete process.env.AI_GATEWAY_BASE_URL;
    else process.env.AI_GATEWAY_BASE_URL = saved.url;
  });
  const nativeFetch = globalThis.fetch;
  let calls = 0;
  t.mock.method(
    globalThis,
    "fetch",
    async (_url: unknown, options?: RequestInit) => {
      calls++;
      const prompt = JSON.parse(String(options?.body)).messages[1].content;
      assert.match(prompt, /"from":"-9223372036854775808"/);
      const content =
        '{"answer":"Candidate","unknown":false,"citations":["world:1:main:epoch"],"operations":[{"action":"put","id":"epoch","record":{"id":"epoch","kind":"event","from":9007199254740993,"until":9223372036854775807}}]}';
      return new Response(
        JSON.stringify({ choices: [{ message: { content } }] }),
      );
    },
  );
  const server = createServer((request, response) => {
    void handleCanonRequest(request, response);
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(
    () =>
      new Promise<void>((resolve, reject) => {
        server.close((err) => (err ? reject(err) : resolve()));
        server.closeAllConnections();
      }),
  );
  const endpoint = `http://127.0.0.1:${(server.address() as AddressInfo).port}/worldbuilder/generate`;
  const body =
    '{"worldId":"world","revision":1,"timeline":"main","workflow":"canon_proposal","question":"Extend epoch","evidence":[{"id":"world:1:main:epoch","worldId":"world","revision":1,"timeline":"main","record":{"id":"epoch","kind":"event","from":-9223372036854775808,"until":9223372036854775807}}]}';
  const response = await nativeFetch(endpoint, {
    method: "POST",
    headers: { Authorization: "Bearer offline-exact-tick-test" },
    body,
  });
  assert.equal(response.status, 200);
  assert.match(await response.text(), /"from":"9007199254740993"/);
  const invalid = await nativeFetch(endpoint, {
    method: "POST",
    headers: { Authorization: "Bearer offline-exact-tick-test" },
    body: body.replace("-9223372036854775808", "-9223372036854775809"),
  });
  assert.equal(invalid.status, 422);
  assert.equal(calls, 1);
});

test("exact signed ticks survive grounded prompts and numeric model candidates", async () => {
  const request = parseWorldJSON(
    '{"worldId":"world","revision":1,"timeline":"main","workflow":"canon_proposal","question":"Extend epoch","evidence":[{"id":"world:1:main:epoch","worldId":"world","revision":1,"timeline":"main","record":{"id":"epoch","kind":"event","from":-9223372036854775808,"until":9223372036854775807}}]}',
  ) as CanonRequest;
  const result = await runCanon(request, async (messages) => {
    assert.match(messages[1].content, /"from":"-9223372036854775808"/);
    assert.match(messages[1].content, /"until":"9223372036854775807"/);
    return {
      text: '{"answer":"Candidate","unknown":false,"citations":["world:1:main:epoch"],"operations":[{"action":"put","id":"epoch","record":{"id":"epoch","kind":"event","from":9007199254740993,"until":9223372036854775807}}]}',
      model: "fixture",
      usage: {},
    };
  });
  assert.match(JSON.stringify(result.operations), /"from":"9007199254740993"/);
  assert.match(
    JSON.stringify(result.operations),
    /"until":"9223372036854775807"/,
  );
});

test("date parsing rejects fractional tokens, overflow and pre-rounded Numbers", () => {
  for (const value of [
    "9223372036854775808",
    "-9223372036854775809",
    "1.0000000000000001",
    "1e3",
  ])
    assert.throws(() => parseWorldJSON(`{"from":${value}}`));
  assert.throws(() => exactTick(9007199254740992));
  assert.equal(parseWorldJSON('{"from":-42,"until":null}').from, -42);
  assert.equal(exactTick("9007199254740991"), 9007199254740991);
});
