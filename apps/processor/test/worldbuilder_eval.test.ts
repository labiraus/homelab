import { readFileSync } from "node:fs";
import { test } from "node:test";
import assert from "node:assert/strict";
import { runCanon, type CanonRequest } from "../src/worldbuilder.js";
const fixture = JSON.parse(
  readFileSync(
    new URL("./fixtures/worldbuilder-evaluation.json", import.meta.url),
    "utf8",
  ),
);
const citation = (id: string) =>
  id.includes(":")
    ? id
    : `${fixture.worldId}:${fixture.revision}:${fixture.timeline}:${id}`;
for (const sample of fixture.cases)
  test(`Worldbuilder evaluation fixture: ${sample.id}`, async () => {
    const request: CanonRequest = {
      worldId: fixture.worldId,
      revision: fixture.revision,
      timeline: fixture.timeline,
      workflow: sample.workflow,
      question: sample.question,
      evidence: fixture.records.map((record: any) => ({
        id: citation(record.id),
        worldId: fixture.worldId,
        revision: fixture.revision,
        timeline: fixture.timeline,
        record,
      })),
    };
    const output = {
      ...sample.output,
      citations: sample.output.citations.map(citation),
      ...(sample.output.findings
        ? {
            findings: sample.output.findings.map((f: any) => ({
              ...f,
              citations: f.citations.map(citation),
            })),
          }
        : {}),
    };
    const run = () =>
      runCanon(request, async (messages) => {
        assert.match(messages[0].content, /untrusted DATA/);
        return {
          text: JSON.stringify(output),
          model: "offline-scripted-fixture",
          usage: { total_tokens: 0 },
        };
      });
    if (sample.accept) {
      const result = await run();
      assert.equal(result.answer, output.answer);
      assert.equal(result.model, "offline-scripted-fixture");
    } else await assert.rejects(run);
  });
