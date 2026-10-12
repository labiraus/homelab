import { useRef, useState } from "react";

export default function WorldTimelineBranch({
  api,
  path,
  timeline,
  revision,
  currentRevision,
  canEdit,
  onSaved,
}) {
  const [scope, setScope] = useState(null);
  const [destination, setDestination] = useState("");
  const [label, setLabel] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [origin, setOrigin] = useState(null);
  const retry = useRef(null);
  const unchangedRetry = scope && retry.current?.signature === JSON.stringify({ ...scope, timeline: destination, label, reason });

  async function submit(event) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const request = { ...scope, timeline: destination, label, reason };
    const signature = JSON.stringify(request);
    if (retry.current?.signature !== signature)
      retry.current = { signature, id: crypto.randomUUID() };
    try {
      await api(`${path}/timeline-branches`, { ...request, id: retry.current.id });
      setNotice("Branch draft created. Review and publish it from the change list.");
      setScope(null);
      await onSaved();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <section aria-label="Timeline branches">
      <h3>Timeline branches</h3>
      <p>
        A branch copies a timeline's records at a chosen revision. Later parent
        edits do not change those copies. World-wide entities, types, sources,
        settings and rules remain shared unless they are explicitly scoped to a
        timeline.
      </p>
      {error && <p role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}
      <button
        disabled={busy}
        onClick={async () => {
          setBusy(true);
          setError("");
          try {
            const result = await api(
              `${path}/timelines/${encodeURIComponent(timeline)}/origin?revision=${revision}`,
            );
            setOrigin({ timeline, revision, result });
          } catch (e) {
            setError(e.message);
          } finally {
            setBusy(false);
          }
        }}
      >
        View timeline origin
      </button>
      {origin && (
        <p>
          {origin.timeline} at revision {origin.revision}: {origin.result
            ? `created from ${origin.result.request.parentTimeline} at revision ${origin.result.request.parentRevision}, published in revision ${origin.result.publishedRevision}, with ${Object.keys(origin.result.records).length} copied records. These are creation details; later reviewed edits may change the branch.`
            : "no recorded branch origin."}
        </p>
      )}
      {canEdit && !scope && (
        <button
          disabled={busy}
          onClick={() => {
            setScope({ parentTimeline: timeline, parentRevision: revision, baseRevision: currentRevision });
            setDestination("");
            setLabel("");
            setReason("");
            setError("");
            setNotice("");
          }}
        >
          Prepare timeline branch
        </button>
      )}
      {canEdit && scope && (
        <form onSubmit={submit}>
          <fieldset disabled={busy}>
            <legend>Create a branch proposal</legend>
            <p>
              Parent {scope.parentTimeline} at revision {scope.parentRevision};
              proposal based on current revision {scope.baseRevision}. This
              includes all parent timeline records and dates, regardless of the
              displayed search or time filter. Up to 99 records can be copied in
              one reviewed proposal; larger copies are refused in full.
            </p>
            {scope.baseRevision !== currentRevision && (
              <p role="alert">Canon changed. Retry only an unchanged request already attempted, or cancel and prepare a new branch proposal after inspecting the current world.</p>
            )}
            <label>
              New timeline ID
              <input value={destination} onChange={(e) => setDestination(e.target.value)} required pattern={"[a-zA-Z0-9_\\-]{1,64}"} maxLength={64} />
            </label>
            <label>
              New timeline name
              <input value={label} onChange={(e) => setLabel(e.target.value)} required maxLength={500} />
            </label>
            <label>
              Branch reason
              <textarea value={reason} onChange={(e) => setReason(e.target.value)} required maxLength={2000} />
            </label>
            <button disabled={scope.baseRevision !== currentRevision && !unchangedRetry}>Create branch draft</button>
            <button type="button" onClick={() => setScope(null)}>Cancel branch draft</button>
          </fieldset>
        </form>
      )}
    </section>
  );
}
