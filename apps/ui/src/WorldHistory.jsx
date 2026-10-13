import { useEffect, useRef, useState } from "react";
import { RecordSummary } from "./WorldResponse.jsx";

function useRequestId() {
  const last = useRef(null);
  return (body) => {
    const signature = JSON.stringify(body);
    if (last.current?.signature !== signature)
      last.current = { signature, id: crypto.randomUUID() };
    return { ...body, id: last.current.id };
  };
}

function RecordValue({ record }) {
  if (!record) return <span>Absent</span>;
  return <RecordSummary record={record} />;
}

export function RecordDiffs({ records }) {
  return records.map((record) => (
    <article key={record.id}>
      <h4>{record.id}</h4>
      <div className="world-diff">
        <section>
          <h5>Before</h5>
          <RecordValue record={record.before} />
        </section>
        <section>
          <h5>After</h5>
          <RecordValue record={record.after} />
        </section>
      </div>
    </article>
  ));
}

export function ReconciliationPanel({
  api,
  path,
  target,
  canEdit,
  onSaved,
  onClose,
}) {
  const [preview, setPreview] = useState(null);
  const [choices, setChoices] = useState({});
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const withId = useRequestId();
  const panel = useRef(null);
  useEffect(() => {
    panel.current?.focus();
  }, [target.kind, target.id]);
  const sourcePath = `${path}/${target.kind}/${target.id}`;
  async function load() {
    setBusy(true);
    setError("");
    try {
      const value = await api(`${sourcePath}/reconciliation`);
      setPreview(value);
      setChoices({});
      setSaved(false);
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  useEffect(() => {
    let active = true;
    api(`${sourcePath}/reconciliation`)
      .then((value) => {
        if (active) setPreview(value);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [api, sourcePath]);
  async function save() {
    setBusy(true);
    setError("");
    try {
      const body = withId({
        baseRevision: preview.currentRevision,
        expectedVersion: preview.expectedVersion,
        reason,
        resolutions: choices,
      });
      await api(
        `${sourcePath}/${target.kind === "changes" ? "rebase" : "reconcile"}`,
        body,
      );
      setSaved(true);
      await onSaved();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <fieldset
      ref={panel}
      tabIndex={-1}
      disabled={busy}
      aria-label="Reconcile changes"
    >
      <legend>Reconcile against current canon</legend>
      {error && <p role="alert">{error}</p>}
      <button onClick={onClose}>Close comparison</button>
      <button onClick={() => void load()}>Refresh comparison</button>
      {preview && (
        <>
          <p>
            Original revision {preview.baseRevision}; current revision{" "}
            {preview.currentRevision}.
            {target.kind === "sessions" &&
              ` Story turn ${preview.expectedVersion}.`}{" "}
            Saving creates a fresh draft requiring independent review.
          </p>
          {preview.validationError && (
            <p role="status">
              Validation finding: {preview.validationError}. Resolve or omit
              affected changes before saving.
            </p>
          )}
          {preview.records.map((record) => (
            <article key={record.id}>
              <h4>
                {record.proposed?.label ||
                  record.current?.label ||
                  record.base?.label ||
                  record.id}
                {record.conflict ? " — conflict" : ""}
              </h4>
              <div className="world-diff">
                <section>
                  <h5>Original</h5>
                  <RecordValue record={record.base} />
                </section>
                <section>
                  <h5>Current canon</h5>
                  <RecordValue record={record.current} />
                </section>
                <section>
                  <h5>Proposed</h5>
                  <RecordValue record={record.proposed} />
                </section>
              </div>
              {canEdit && (
                <label>
                  Resolution for {record.id}
                  <select
                    value={
                      choices[record.id] ?? (record.conflict ? "" : "proposed")
                    }
                    onChange={(e) => {
                      setChoices({ ...choices, [record.id]: e.target.value });
                      setSaved(false);
                    }}
                  >
                    {record.conflict && (
                      <option value="">Choose a resolution</option>
                    )}
                    <option value="proposed">Use proposed record</option>
                    <option value="current">Keep current canon</option>
                  </select>
                </label>
              )}
            </article>
          ))}
          {!preview.records.length && <p>No record changes to reconcile.</p>}
          {canEdit && (
            <>
              <label>
                Reconciliation reason
                <input
                  maxLength={2000}
                  value={reason}
                  onChange={(e) => {
                    setReason(e.target.value);
                    setSaved(false);
                  }}
                />
              </label>
              <button
                disabled={
                  saved ||
                  !reason.trim() ||
                  !preview.records.length ||
                  preview.records.some((r) => r.conflict && !choices[r.id])
                }
                onClick={() => void save()}
              >
                Save reconciled draft
              </button>
            </>
          )}
          {saved && (
            <p role="status">
              Reconciled draft saved. Submit it in the review queue.
            </p>
          )}
        </>
      )}
    </fieldset>
  );
}

export function HistoryPanel({ api, path, currentRevision, canEdit, onSaved }) {
  const [rows, setRows] = useState([]);
  const [offset, setOffset] = useState(0);
  const [diff, setDiff] = useState(null);
  const [diffOffset, setDiffOffset] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [reason, setReason] = useState("");
  const [saved, setSaved] = useState(false);
  const withId = useRequestId();
  useEffect(() => {
    let active = true;
    api(`${path}/revisions?limit=10&offset=${offset}`)
      .then((value) => {
        if (active) setRows(value);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [api, path, offset, currentRevision]);
  async function compare(from, to, pageOffset = 0) {
    setBusy(true);
    setError("");
    try {
      setDiff(
        await api(
          `${path}/diff?from=${from}&to=${to}&limit=100&offset=${pageOffset}`,
        ),
      );
      setDiffOffset(pageOffset);
      setSaved(false);
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  async function restore() {
    setBusy(true);
    setError("");
    try {
      await api(
        `${path}/restore`,
        withId({
          baseRevision: diff.toRevision,
          targetRevision: diff.fromRevision,
          reason,
        }),
      );
      setSaved(true);
      await onSaved();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <fieldset disabled={busy}>
      <legend>Published revision history</legend>
      {error && <p role="alert">{error}</p>}
      {rows.map((r) => (
        <article key={r.revision}>
          <h3>
            Revision {r.revision}: {r.reason}
          </h3>
          <p>
            {r.actor} · <time dateTime={r.createdAt}>{r.createdAt}</time>
          </p>
          <button
            onClick={() =>
              void compare(Math.max(0, r.revision - 1), r.revision)
            }
          >
            Inspect revision {r.revision}
          </button>
          {r.revision < currentRevision && (
            <button onClick={() => void compare(r.revision, currentRevision)}>
              Compare revision {r.revision} with current
            </button>
          )}
        </article>
      ))}
      <button
        disabled={offset === 0}
        onClick={() => setOffset(Math.max(0, offset - 10))}
      >
        Newer revisions
      </button>
      <button
        disabled={rows.length < 10}
        onClick={() => setOffset(offset + 10)}
      >
        Older revisions
      </button>
      {diff && (
        <>
          <h3>
            Revision {diff.fromRevision} → {diff.toRevision}
          </h3>
          <p>
            {diff.total} changed records; showing{" "}
            {diffOffset + (diff.records.length ? 1 : 0)}–
            {diffOffset + diff.records.length}.
          </p>
          <RecordDiffs records={diff.records} />
          <button
            disabled={!diffOffset}
            onClick={() =>
              void compare(
                diff.fromRevision,
                diff.toRevision,
                Math.max(0, diffOffset - 100),
              )
            }
          >
            Previous changes
          </button>
          <button
            disabled={diffOffset + diff.records.length >= diff.total}
            onClick={() =>
              void compare(diff.fromRevision, diff.toRevision, diffOffset + 100)
            }
          >
            Next changes
          </button>
          {canEdit &&
            diff.toRevision === currentRevision &&
            diff.fromRevision < diff.toRevision &&
            diff.total > 0 && (
              <>
                <label>
                  Restore reason
                  <input
                    maxLength={2000}
                    value={reason}
                    onChange={(e) => {
                      setReason(e.target.value);
                      setSaved(false);
                    }}
                  />
                </label>
                <button
                  disabled={saved || !reason.trim() || diff.total > 100}
                  onClick={() => void restore()}
                >
                  Propose restoring revision {diff.fromRevision}
                </button>
                {diff.total > 100 && (
                  <p>
                    This restore exceeds the 100-operation proposal limit.
                    Prepare smaller, independently valid compensating proposals.
                  </p>
                )}
                {saved && (
                  <p role="status">
                    Restore draft saved for independent review. Published
                    history is preserved.
                  </p>
                )}
              </>
            )}
        </>
      )}
    </fieldset>
  );
}
