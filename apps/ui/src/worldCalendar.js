import { exactTick } from "./worldTicks.js";

const integer = (value, min, max) => {
  if (!/^[0-9]+$/.test(String(value)))
    throw new Error("Calendar counts must be whole numbers.");
  const n = Number(value);
  if (!Number.isSafeInteger(n) || n < min || n > max)
    throw new Error(`Calendar count must be between ${min} and ${max}.`);
  return n;
};
const name = (value) => {
  if (
    typeof value !== "string" ||
    !value ||
    value.trim() !== value ||
    new TextEncoder().encode(value).length > 80
  )
    throw new Error(
      "Calendar names require 1–80 UTF-8 bytes without surrounding spaces.",
    );
  return value;
};
const unique = (values) => {
  if (new Set(values).size !== values.length)
    throw new Error("Calendar names must be unique within months or weekdays.");
};

// Convert structured editor inputs once; never convert tick/year strings to Number.
export function calendarFromDraft(draft) {
  if (!draft) return undefined;
  const c = {
    name: name(draft.name),
    epochTick: String(exactTick(draft.epochTick)),
    epochYear: String(exactTick(draft.epochYear)),
    ticksPerDay: integer(draft.ticksPerDay, 1, 1e9),
    cycleYears: integer(draft.cycleYears, 1, 1000),
    months: draft.months.map((m) => ({
      name: name(m.name),
      days: integer(m.days, 0, 10000),
      leapDays: integer(m.leapDays ?? 0, 0, 10000),
    })),
    leapYears:
      typeof draft.leapYears === "string"
        ? draft.leapYears.trim()
          ? draft.leapYears.split(",").map((s) => s.trim())
          : []
        : draft.leapYears || [],
    weekdays:
      typeof draft.weekdays === "string"
        ? draft.weekdays
          ? draft.weekdays.split("\n")
          : []
        : draft.weekdays || [],
    weekdayAtEpoch: integer(draft.weekdayAtEpoch ?? 0, 0, 31),
  };
  if (!c.months.length || c.months.length > 32 || c.weekdays.length > 32)
    throw new Error("Use 1–32 months and at most 32 weekdays.");
  unique(c.months.map((m) => m.name));
  unique(c.weekdays.map(name));
  c.leapYears = c.leapYears.map((y) => integer(y, 1, c.cycleYears));
  if (c.leapYears.some((y, i) => i > 0 && y <= c.leapYears[i - 1]))
    throw new Error("Leap positions must increase without duplicates.");
  if (
    c.months.some((m) => m.days + m.leapDays === 0) ||
    !c.months.some((m) => m.days > 0)
  )
    throw new Error("Months and ordinary years must have days.");
  if (Boolean(c.leapYears.length) !== c.months.some((m) => m.leapDays > 0))
    throw new Error("Configure leap positions and extra month days together.");
  if (c.weekdayAtEpoch >= Math.max(1, c.weekdays.length))
    throw new Error("Choose an epoch weekday from the configured list.");
  return c;
}

const floorDiv = (a, b) => a / b - (a % b < 0n ? 1n : 0n);
const mod = (a, b) => a - floorDiv(a, b) * b;
const prefix = (lengths) =>
  lengths.reduce((p, n) => [...p, p.at(-1) + BigInt(n)], [0n]);
// Largest start <= offset. Zero-length intercalary months are skipped.
function segment(starts, offset) {
  let low = 0,
    high = starts.length - 1;
  while (low + 1 < high) {
    const mid = Math.floor((low + high) / 2);
    if (starts[mid] <= offset) low = mid;
    else high = mid;
  }
  return low;
}

export function createCalendarFormatter(calendar) {
  if (!calendar) return (value) => String(value);
  const c = calendarFromDraft(calendar);
  const leap = new Set(c.leapYears);
  const ordinaryMonths = prefix(c.months.map((m) => m.days));
  const leapMonths = prefix(c.months.map((m) => m.days + m.leapDays));
  const years = prefix(
    Array.from({ length: c.cycleYears }, (_, i) =>
      (leap.has(i + 1) ? leapMonths : ordinaryMonths).at(-1),
    ),
  );
  const epoch = BigInt(c.epochTick),
    year0 = BigInt(c.epochYear),
    perDay = BigInt(c.ticksPerDay);
  return (value) => {
    const tick = BigInt(exactTick(value));
    const elapsed = tick - epoch,
      day = floorDiv(elapsed, perDay);
    const cycle = floorDiv(day, years.at(-1)),
      dayInCycle = mod(day, years.at(-1));
    const y = segment(years, dayInCycle),
      dayInYear = dayInCycle - years[y];
    const months = leap.has(y + 1) ? leapMonths : ordinaryMonths;
    const m = segment(months, dayInYear),
      d = dayInYear - months[m] + 1n;
    const year = year0 + cycle * BigInt(c.cycleYears) + BigInt(y);
    const weekday = c.weekdays.length
      ? `${c.weekdays[Number(mod(day + BigInt(c.weekdayAtEpoch), BigInt(c.weekdays.length)))]}, `
      : "";
    const subday = perDay > 1n ? `, tick ${mod(elapsed, perDay)} of day` : "";
    return `${weekday}${d} ${c.months[m].name}, year ${year}${subday} (${c.name}; tick ${tick})`;
  };
}
