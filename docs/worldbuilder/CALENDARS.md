# Reviewed fictional calendars and date facts

Calendars are optional canon settings, published through independent human review. Edit the world's `world_settings` record in the wiki, enable **Use a fictional calendar**, fill its structured fields, stage and submit the complete record. Reviewers inspect the calendar before publication. Disabling it proposes removal of the calendar field; dates then display raw ticks. World settings themselves cannot be deleted.

This functionality requires explicit migration 006 and schema 6 before activation. CI publishes artifacts; it does not install the schema. Follow the [LAN runbook](LAN-RUNBOOK.md).

## Calendar contract

`calendar` is permitted only on `world_settings`. For example:

```json
{
  "name": "Harbour calendar",
  "epochTick": "10",
  "epochYear": "-2",
  "ticksPerDay": 4,
  "months": [
    {"name": "Tide", "days": 2},
    {"name": "Rest", "days": 0, "leapDays": 1},
    {"name": "Dusk", "days": 1}
  ],
  "cycleYears": 2,
  "leapYears": [2],
  "weekdays": ["Sun", "Moon", "Star"],
  "weekdayAtEpoch": 1
}
```

- `epochTick` and `epochYear` are canonical signed int64 decimal **strings**, including for small numbers. No exponent, leading plus/zeroes, negative zero or spaces. The epoch begins day 1 of the first nonempty month in the epoch year.
- `ticksPerDay` is an integer from 1 to 1,000,000,000. Subday ticks start at zero; a tick is not implicitly a second.
- `months` contains 1–32 ordered unique names. Ordinary `days` and optional additive `leapDays` are each 0–10,000. A month must have some days, and the ordinary year must be nonempty. A zero-day ordinary month with leap days is intercalary and skipped in ordinary years.
- `cycleYears` is 1–1000. `leapYears` is an increasing unique list of positions in 1–cycleYears; position 1 means the epoch year. The pattern repeats in both directions. Configure leap positions and extra month days together, or neither. A 400-year Gregorian-like pattern or 19-year lunar pattern uses the same representation.
- Optional `weekdays` contains up to 32 unique names. `weekdayAtEpoch` is its zero-based index, or zero with no weekdays. Weekdays advance on every day, including intercalary days, without restarting at year boundaries.
- Calendar, month and weekday names are 1–80 UTF-8 bytes, without surrounding spaces. Months and weekdays are unique within their respective lists. Unknown nested fields fail decoding.

For the example, tick 10 is Moon, 1 Tide, year -2; tick 9 is Sun, 1 Dusk, year -3, subday tick 3. Tick 30 is 1 Rest, year -1; tick 38 starts year 0. Negative dates use floor division. Derived year labels may extend beyond int64 although input ticks/epochs remain within it. Computation uses BigInt, never JavaScript Date or SQL timestamps.

## Scope and history

Canon and search responses include `calendar` from their requested immutable snapshot, or null. This metadata remains available when filtering/pagination omits settings. Wiki dates, relationship intervals and chronology use that returned calendar. Staged settings, the editor catalog, unsaved filters and current-world metadata cannot format a historical view. All timelines share the revision's calendar.

Formatting retains the original tick alongside the civil date. Editing and `at` filters still take raw exact ticks; display does not reinterpret stored history, change interval semantics or infer timeline inheritance. Validity includes the start and excludes the end. This contract supports bounded repeating cycles; irregular historical reforms, per-timeline calendars and civil-date input conversion are outside it.

## Typed dates and life bounds

Declare date predicates as `"fields": {"birth": "tick", "death": "tick"}`. Facts use `"valueType": "tick"` with a canonical decimal **string** value, for example `"value": "-9223372036854775808"`. Numeric JSON values are refused for this new type. Record `from`/`until` retain their numeric history representation and exact numeric/string input compatibility.

`life_bounds` compares `tick` facts across signed int64, accepting birth and excluding death. Existing `number` date facts retain safe-integer validation and are never silently converted. Change an existing schema by proposing its type definition and affected facts together for review. Pinned revisions retain their old schema/values. Other numeric facts keep their existing semantics.

Offline acceptance covers full-range dates/life bounds, leap/intercalary/subday/negative arithmetic, a 400-year cycle, strict configuration, reviewed publication/removal, filtered/historical calendars, structured editing and actual Chromium staging isolation against disposable PostgreSQL. Production installation and deployed acceptance remain LAN gates.
