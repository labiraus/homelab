export default function WorldCalendarEditor({ value, onChange }) {
  const set = (key, v) => onChange({ ...value, [key]: v });
  return (
    <fieldset>
      <legend>Fictional calendar</legend>
      <label>
        <input
          type="checkbox"
          checked={!!value}
          onChange={(e) =>
            onChange(
              e.target.checked
                ? {
                    name: "World calendar",
                    epochTick: "0",
                    epochYear: "1",
                    ticksPerDay: 1,
                    months: [{ name: "First month", days: 30, leapDays: 0 }],
                    cycleYears: 1,
                    leapYears: [],
                    weekdays: [],
                    weekdayAtEpoch: 0,
                  }
                : undefined,
            )
          }
        />
        Use a fictional calendar
      </label>
      {value && (
        <>
          <p>
            The epoch begins the first day of the first nonempty month. Leap
            positions repeat relative to the epoch year, including before it.
            Raw ticks remain the editing and filtering format. Changes require
            independent review.
          </p>
          {[
            ["name", "Calendar name"],
            ["epochTick", "Epoch tick"],
            ["epochYear", "Epoch year"],
            ["ticksPerDay", "Ticks per day"],
            ["cycleYears", "Years per cycle"],
          ].map(([key, label]) => (
            <label key={key}>
              {label}
              <input
                required
                value={value[key]}
                onChange={(e) => set(key, e.target.value)}
              />
            </label>
          ))}
          <label>
            Leap year positions (comma separated, starting at 1)
            <input
              value={
                Array.isArray(value.leapYears)
                  ? value.leapYears.join(", ")
                  : value.leapYears || ""
              }
              onChange={(e) => set("leapYears", e.target.value)}
            />
          </label>
          {value.months.map((month, i) => (
            <fieldset key={i}>
              <legend>Month {i + 1}</legend>
              {[
                ["name", "Name"],
                ["days", "Ordinary days"],
                ["leapDays", "Extra days in leap years"],
              ].map(([key, label]) => (
                <label key={key}>
                  {label} {i + 1}
                  <input
                    required
                    value={month[key] ?? 0}
                    onChange={(e) =>
                      set(
                        "months",
                        value.months.map((m, j) =>
                          i === j ? { ...m, [key]: e.target.value } : m,
                        ),
                      )
                    }
                  />
                </label>
              ))}
              <button
                type="button"
                disabled={value.months.length === 1}
                onClick={() =>
                  set(
                    "months",
                    value.months.filter((_, j) => i !== j),
                  )
                }
              >
                Remove month {i + 1}
              </button>
            </fieldset>
          ))}
          <button
            type="button"
            disabled={value.months.length >= 32}
            onClick={() =>
              set("months", [
                ...value.months,
                { name: "", days: 30, leapDays: 0 },
              ])
            }
          >
            Add month
          </button>
          <label>
            Weekday names (one per line, optional)
            <textarea
              value={
                Array.isArray(value.weekdays)
                  ? value.weekdays.join("\n")
                  : value.weekdays || ""
              }
              onChange={(e) => set("weekdays", e.target.value)}
            />
          </label>
          <label>
            Epoch weekday position (starting at 0)
            <input
              required
              value={value.weekdayAtEpoch ?? 0}
              onChange={(e) => set("weekdayAtEpoch", e.target.value)}
            />
          </label>
        </>
      )}
    </fieldset>
  );
}
