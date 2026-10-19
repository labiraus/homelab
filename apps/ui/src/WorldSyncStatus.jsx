import { useEffect, useState } from "react";
import { deviceQueue, offlineEnabled, onDeviceChange } from "./worldOffline.js";
export default function WorldSyncStatus() {
  const [status, setStatus] = useState("");
  useEffect(() => {
    let active = true;
    const load = async () => {
      if (!offlineEnabled()) {
        if (active) setStatus("");
        return;
      }
      try {
        const items = await deviceQueue();
        const text = items.some((i) => i.status === "sign-in")
          ? "Needs sign-in"
          : items.some((i) =>
                ["conflict", "denied", "invalid"].includes(i.status),
              )
            ? "Needs resolution"
            : items.some((i) => i.status === "queued")
              ? "Waiting to sync"
              : items.length
                ? "Synced"
                : "Offline writing enabled";
        if (active) setStatus(text);
      } catch {
        if (active) setStatus("Device storage unavailable");
      }
    };
    void load();
    const stop = onDeviceChange(load);
    return () => {
      active = false;
      stop();
    };
  }, []);
  return status ? (
    <span className="wb-sync-status" role="status">
      {status}
    </span>
  ) : null;
}
