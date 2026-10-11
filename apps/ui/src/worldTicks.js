// Keep safe dates compatible with numeric clients; carry larger dates as exact
// decimal strings. Never construct a BigInt from an already rounded Number.
export function exactTick(value) {
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

export function parseWorldJSON(text) {
  return JSON.parse(text, (key, value, context) => {
    if (["from", "until", "at"].includes(key) && typeof value === "number") {
      // Revivers run after numeric conversion. Only the original token is safe.
      // Older engines must fail visibly instead of accepting rounded evidence.
      if (!context?.source)
        throw new Error(
          "Exact fictional dates require a browser with JSON source support. Please update your browser.",
        );
      return exactTick(context.source);
    }
    return value;
  });
}
