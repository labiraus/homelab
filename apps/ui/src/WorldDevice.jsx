import { useEffect, useState } from "react";
import { api, onlineApi } from "./Worldbuilder.jsx";
import { readWorldRoute } from "./worldWorkspace.js";
import { RecordSummary } from "./WorldResponse.jsx";
import {
  offlineEnabled,
  enableOffline,
  downloadWorld,
  deviceQueue,
  onDeviceChange,
  syncDevice,
  exportDevice,
  importDevice,
  clearDevice,
  removeQueued,
  flushLocalWriting,
  installationPrompt,
  consumedInstallation,
} from "./worldOffline.js";

function Conflict({ item, onDone }) {
  const [preview, setPreview] = useState(null),
    [choices, setChoices] = useState({}),
    [error, setError] = useState("");
  async function inspect() {
    try {
      const worldId = item.path.split("/")[1];
      const world = await onlineApi(`/${worldId}`);
      const rows = [];
      for (const op of item.body.operations || []) {
        const timeline = op.record?.timeline || world.defaultTimeline;
        const read = async (revision) => {
          try {
            return (
              await onlineApi(
                `/${worldId}/records/${op.id || op.record.id}?timeline=${encodeURIComponent(timeline)}&revision=${revision}`,
              )
            ).record;
          } catch (e) {
            if (e.status === 404) return null;
            throw e;
          }
        };
        rows.push({
          id: op.id || op.record.id,
          original: await read(item.body.baseRevision),
          current: await read(world.revision),
          proposed: op.record || null,
          op,
        });
      }
      setPreview({ world, rows });
      setChoices({});
    } catch (e) {
      setError(e.message);
    }
  }
  async function reconcile() {
    try {
      const operations = preview.rows
        .filter((r) => choices[r.id] === "proposed")
        .map((r) => r.op);
      if (!operations.length)
        throw Error(
          "Choose at least one proposed change, or discard this local request.",
        );
      await api(item.path, {
        ...item.body,
        id: crypto.randomUUID(),
        baseRevision: preview.world.revision,
        reason: `Reconciled offline draft: ${item.body.reason}`,
        operations,
      });
      await removeQueued(item.id);
      onDone();
    } catch (e) {
      setError(e.message);
    }
  }
  if (!item.path.endsWith("/changes"))
    return (
      <p>
        Your story changed on the server. Export this writing, inspect the
        current story, and explicitly author a new turn. Dependent turns remain
        paused.
      </p>
    );
  return (
    <div>
      <button onClick={inspect}>Compare Original / Current / Proposed</button>
      {preview && (
        <>
          <p>
            Fresh draft at revision {preview.world.revision}. Publication still
            requires the world’s policy.
          </p>
          {preview.rows.map((r) => (
            <article key={r.id}>
              <h4>{r.id}</h4>
              {[
                ["Original", r.original],
                ["Current canon", r.current],
                ["Proposed", r.proposed],
              ].map(([label, value]) => (
                <section key={label}>
                  <h5>{label}</h5>
                  <RecordSummary record={value} />
                </section>
              ))}
              <label>
                Resolution
                <select
                  value={choices[r.id] || ""}
                  onChange={(e) =>
                    setChoices({ ...choices, [r.id]: e.target.value })
                  }
                >
                  <option value="">Choose explicitly</option>
                  <option value="current">Keep current canon</option>
                  <option value="proposed">Use proposed change</option>
                </select>
              </label>
            </article>
          ))}
          <button
            disabled={preview.rows.some((r) => !choices[r.id])}
            onClick={reconcile}
          >
            Save reconciled draft
          </button>
        </>
      )}
      {error && <p role="alert">{error}</p>}
    </div>
  );
}
export default function WorldDevice({ pwa }) {
  const [enabled, setEnabled] = useState(offlineEnabled()),
    [queue, setQueue] = useState([]),
    [message, setMessage] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [install, setInstall] = useState(installationPrompt()),
    [storage, setStorage] = useState(null),
    [confirmClear, setConfirmClear] = useState(false);
  const {
    needRefresh: [refresh],
    offlineReady: [ready],
    updateServiceWorker,
  } = pwa;
  async function reload() {
    setInstall(installationPrompt());
    setEnabled(offlineEnabled());
    setQueue(await deviceQueue());
    setStorage(await navigator.storage?.estimate?.());
  }
  useEffect(() => {
    void reload().catch((e) => setError(e.message));
    return onDeviceChange(
      () => void reload().catch((e) => setError(e.message)),
    );
  }, []);
  useEffect(() => {
    const listen = (e) => {
      e.preventDefault();
      setInstall(e);
    };
    window.addEventListener("beforeinstallprompt", listen);
    return () => window.removeEventListener("beforeinstallprompt", listen);
  }, []);
  async function act(fn) {
    setBusy(true);
    setError("");
    try {
      await fn();
      await reload();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <details className="wb-device">
      <summary>
        App and offline writing{" "}
        {queue.some((q) => q.status !== "synced") ? "· work waiting" : ""}
      </summary>
      <div className="wb-device-body">
        <p>
          Install Worldbuilder for a dedicated writing workspace. Downloads stay
          on this device and are not encrypted. Sign-out hides them until this
          account signs in again.
        </p>
        {install ? (
          <button
            onClick={() =>
              act(async () => {
                await install.prompt();
                await install.userChoice;
                consumedInstallation();
                setInstall(null);
              })
            }
          >
            Install Worldbuilder
          </button>
        ) : (
          <p className="wb-muted">
            Use your browser’s Install app or Add to Home screen action when
            available.
          </p>
        )}
        {ready && <p>Application assets are ready offline.</p>}
        {refresh && (
          <button
            onClick={() =>
              act(async () => {
                await flushLocalWriting();
                await updateServiceWorker(true);
              })
            }
          >
            Update application
          </button>
        )}
        {!enabled ? (
          <button disabled={busy} onClick={() => act(enableOffline)}>
            Enable offline writing on this device
          </button>
        ) : (
          <>
            <div className="wb-device-actions">
              <button
                disabled={busy}
                onClick={() =>
                  act(async () => {
                    const route = readWorldRoute();
                    if (!route.worldId) throw Error("Open a world first.");
                    const world = await onlineApi(`/${route.worldId}`);
                    const count = await downloadWorld(
                      route.worldId,
                      route.params?.get("timeline") || world.defaultTimeline,
                      onlineApi,
                    );
                    setMessage(
                      `Downloaded ${count} records with their published scope.`,
                    );
                  })
                }
              >
                Download this timeline and private stories
              </button>
              <button
                disabled={busy}
                onClick={() => act(() => syncDevice(onlineApi))}
              >
                Sync saved work
              </button>
              <button
                onClick={() =>
                  act(async () => {
                    const blob = new Blob([await exportDevice()], {
                      type: "application/json",
                    });
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement("a");
                    a.href = url;
                    a.download = "worldbuilder-local-writing.json";
                    a.click();
                    setTimeout(() => URL.revokeObjectURL(url), 1000);
                  })
                }
              >
                Export unsynced writing
              </button>
              <label>
                Import writing
                <input
                  type="file"
                  accept="application/json"
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file)
                      void act(async () => importDevice(await file.text()));
                  }}
                />
              </label>
            </div>
            <p className="wb-muted">
              {storage
                ? `${Math.round((storage.usage || 0) / 1024 / 1024)} MB used on this origin.`
                : ""}{" "}
              Browser storage can be cleared or evicted; export important
              unsynced writing. AI, approvals and publication require a
              connection.
            </p>
            {queue.map((item) => (
              <article className="wb-device-item" key={item.id}>
                <strong>
                  {
                    {
                      queued: "Waiting to sync",
                      synced: "Synced",
                      conflict: "Needs resolution",
                      denied: "Access changed",
                      invalid: "Needs correction",
                      "sign-in": "Needs sign-in",
                    }[item.status]
                  }
                </strong>
                <p>{item.body.reason || item.body.narration || item.id}</p>
                {item.error && <p>{item.error}</p>}
                {item.status === "conflict" && (
                  <Conflict item={item} onDone={reload} />
                )}
                <button onClick={() => act(() => removeQueued(item.id))}>
                  {item.status === "synced"
                    ? "Dismiss receipt"
                    : "Discard queued request"}
                </button>
              </article>
            ))}
            <button onClick={() => setConfirmClear(true)}>
              Clear this account’s device data…
            </button>
            {confirmClear && (
              <div role="group" aria-label="Confirm device clearing">
                <p>
                  This removes downloaded content and unsynced writing. Export
                  anything you need first.
                </p>
                <button
                  onClick={() =>
                    act(async () => {
                      await clearDevice();
                      setConfirmClear(false);
                    })
                  }
                >
                  Delete local data
                </button>
                <button onClick={() => setConfirmClear(false)}>
                  Keep local data
                </button>
              </div>
            )}
          </>
        )}
        {message && <p role="status">{message}</p>}
        {error && <p role="alert">{error}</p>}
      </div>
    </details>
  );
}
