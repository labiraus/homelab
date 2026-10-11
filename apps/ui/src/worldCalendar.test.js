import { describe, it, expect } from "vitest";
import { calendarFromDraft, createCalendarFormatter } from "./worldCalendar.js";

export const calendar = {
  name: "Harbour",
  epochTick: "10",
  epochYear: "-2",
  ticksPerDay: 4,
  months: [
    { name: "Tide", days: 2 },
    { name: "Rest", days: 0, leapDays: 1 },
    { name: "Dusk", days: 1 },
  ],
  cycleYears: 2,
  leapYears: [2],
  weekdays: ["Sun", "Moon", "Star"],
  weekdayAtEpoch: 1,
};

describe("fictional calendars", () => {
  it("handles negative day division, leap/intercalary months, weekdays and subday ticks", () => {
    const format = createCalendarFormatter(calendar);
    expect(format(10)).toBe(
      "Moon, 1 Tide, year -2, tick 0 of day (Harbour; tick 10)",
    );
    expect(format(9)).toBe(
      "Sun, 1 Dusk, year -3, tick 3 of day (Harbour; tick 9)",
    );
    expect(format(18)).toContain("1 Dusk, year -2"); // skip intercalary month
    expect(format(22)).toContain("1 Tide, year -1");
    expect(format(30)).toContain("1 Rest, year -1");
    expect(format(34)).toContain("1 Dusk, year -1");
    expect(format(38)).toContain("1 Tide, year 0");
    expect(format(-18)).toContain("1 Tide, year -4");
  });
  it("preserves full-range ticks and differences/derived years larger than int64", () => {
    const format = createCalendarFormatter({
      ...calendar,
      epochTick: "9223372036854775807",
      epochYear: "0",
      ticksPerDay: 1,
      months: [{ name: "Day", days: 1 }],
      cycleYears: 1,
      leapYears: [],
      weekdays: [],
      weekdayAtEpoch: 0,
    });
    expect(format("-9223372036854775808")).toBe(
      "1 Day, year -18446744073709551615 (Harbour; tick -9223372036854775808)",
    );
    expect(format("9223372036854775807")).toContain("year 0");
    expect(() => format(9223372036854775807)).toThrow(/exact/);
    expect(createCalendarFormatter(null)("-9223372036854775808")).toBe(
      "-9223372036854775808",
    );
  });
  it("supports a 400-year leap pattern in both directions without wall-clock dates", () => {
    const c = {
      name: "Civil",
      epochTick: "0",
      epochYear: "2001",
      ticksPerDay: 1,
      cycleYears: 400,
      leapYears: Array.from({ length: 400 }, (_, i) => i + 1).filter(
        (y) => y % 4 === 0 && (y % 100 !== 0 || y === 400),
      ),
      months: [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31].map(
        (days, i) => ({
          name: `Month ${i + 1}`,
          days,
          leapDays: i === 1 ? 1 : 0,
        }),
      ),
      weekdayAtEpoch: 0,
    };
    const format = createCalendarFormatter(c);
    expect(format(-1)).toContain("31 Month 12, year 2000");
    expect(format(-307)).toContain("29 Month 2, year 2000");
    expect(format(146097)).toContain("1 Month 1, year 2401");
    expect(format(-146097)).toContain("1 Month 1, year 1601");
  });
  it("validates edited counts and leap configuration before staging", () => {
    expect(
      calendarFromDraft({ ...calendar, leapYears: "2", ticksPerDay: "4" })
        .ticksPerDay,
    ).toBe(4);
    for (const change of [
      { cycleYears: "" },
      { ticksPerDay: 0 },
      { epochTick: "9223372036854775808" },
      { leapYears: "2,2" },
      { leapYears: [] },
      { weekdayAtEpoch: 3 },
      { months: [{ name: "Empty", days: 0 }] },
    ])
      expect(() => calendarFromDraft({ ...calendar, ...change })).toThrow();
  });
});
