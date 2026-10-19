import { useEffect, useRef, useState } from "react";
import {
  saveLocalDraft,
  loadLocalDraft,
  offlineEnabled,
  onDeviceChange,
  registerLocalWriter,
} from "./worldOffline.js";
export default function useLocalWriting(name, value, restore) {
  const latest = useRef(value),
    restoreRef = useRef(restore);
  latest.current = value;
  restoreRef.current = restore;
  const [enabled, setEnabled] = useState(offlineEnabled()),
    [ready, setReady] = useState(false),
    [error, setError] = useState(""),
    [saved, setSaved] = useState(false);
  useEffect(() => onDeviceChange(() => setEnabled(offlineEnabled())), []);
  useEffect(() => {
    let active = true;
    setReady(false);
    setSaved(false);
    if (!name || !enabled) return;
    loadLocalDraft(name)
      .then((v) => {
        if (active) {
          if (v) restoreRef.current(v);
          setReady(true);
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [name, enabled]);
  useEffect(() => {
    if (!ready || !name || !enabled) return;
    return registerLocalWriter(() => saveLocalDraft(name, latest.current));
  }, [name, ready, enabled]);
  useEffect(() => {
    if (!ready || !name || !enabled) return;
    let active = true;
    setSaved(false);
    saveLocalDraft(name, value)
      .then(() => {
        if (active) {
          setSaved(true);
          setError("");
        }
      })
      .catch((e) => {
        if (active)
          setError(
            `Local save failed: ${e.message}. Export or copy your writing before leaving.`,
          );
      });
    return () => {
      active = false;
    };
  }, [name, value, ready, enabled]);
  return { localSaved: saved, localError: error };
}
