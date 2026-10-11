// This independently packaged service shares the date wire contract with the UI.
export function exactTick(value: unknown): number | string {
  if (
    (typeof value !== "string" && typeof value !== "number") ||
    (typeof value === "number" && !Number.isSafeInteger(value)) ||
    !/^-?(0|[1-9][0-9]*)$/.test(String(value))
  )
    throw new Error("Fictional time must be an exact signed decimal integer.");
  const tick = BigInt(value);
  if (tick < -(1n << 63n) || tick >= 1n << 63n)
    throw new Error("Fictional time exceeds the signed 64-bit range.");
  return tick >= BigInt(Number.MIN_SAFE_INTEGER) &&
    tick <= BigInt(Number.MAX_SAFE_INTEGER)
    ? Number(tick)
    : tick.toString();
}

export function parseWorldJSON(text: string): any {
  return JSON.parse(
    text,
    (key: string, value: unknown, context?: { source: string }) => {
      if (["from", "until", "at"].includes(key) && typeof value === "number") {
        if (!context?.source)
          throw new Error("Exact fictional dates require JSON source support.");
        return exactTick(context.source);
      }
      return value;
    },
  );
}
