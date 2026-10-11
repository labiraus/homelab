import { describe, it, expect, vi } from "vitest";
import { exactTick, parseWorldJSON } from "./worldTicks.js";

describe("exact fictional ticks", () => {
  it("recovers numeric tokens before rounded values enter evidence or editor state", () => {
    const result = parseWorldJSON(
      '{"records":[{"from":-9223372036854775808,"until":9223372036854775807},{"from":9007199254740993,"until":9007199254740994}],"at":-42,"attributes":{"from":"unrelated prose"}}',
    );
    expect(result.records).toEqual([
      { from: "-9223372036854775808", until: "9223372036854775807" },
      { from: "9007199254740993", until: "9007199254740994" },
    ]);
    expect(result.at).toBe(-42);
    expect(result.attributes.from).toBe("unrelated prose");
    expect(JSON.parse(JSON.stringify(result))).toEqual(result);
  });
  it("preserves safe numeric clients and rejects rounding, fractions and overflow", () => {
    for (const tick of [-9007199254740991, -1, 0, 1, 9007199254740991]) {
      expect(exactTick(String(tick))).toBe(tick);
      expect(parseWorldJSON(`{"from":${tick}}`).from).toBe(tick);
    }
    for (const token of [
      "9223372036854775808",
      "-9223372036854775809",
      "1.0000000000000001",
      "1e3",
    ]) {
      expect(() => parseWorldJSON(`{"from":${token}}`)).toThrow();
    }
    for (const value of [9007199254740992, "01", "+1", " 1", "", null, true])
      expect(() => exactTick(value)).toThrow();
  });
  it("fails visibly when a browser cannot expose the original numeric token", () => {
    const nativeParse = JSON.parse;
    const mock = vi
      .spyOn(JSON, "parse")
      .mockImplementation((text, reviver) =>
        nativeParse(text, (key, value) => reviver(key, value)),
      );
    try {
      expect(() => parseWorldJSON('{"from":9223372036854775807}')).toThrow(
        /update your browser/,
      );
    } finally {
      mock.mockRestore();
    }
  });
});
