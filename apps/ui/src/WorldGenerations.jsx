import { useEffect, useRef, useState } from "react";

const merge = (first, second) =>
  [...new Map([...second, ...first].map((row) => [row.id, row])).values()].sort(
    (a, b) =>
      b.createdAt.localeCompare(a.createdAt) || a.id.localeCompare(b.id),
  );
export default function WorldGenerations({
  api,
  path,
  request,
  archived,
  onCandidate,
}) {
  const [items, setItems] = useState([]),
    [enabled, setEnabled] = useState(false),
    [selected, setSelected] = useState(""),
    [job, setJob] = useState(null),
    [parts, setParts] = useState(1),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [more, setMore] = useState(false),
    [refreshVersion, setRefreshVersion] = useState(0);
  const retry = useRef(null),
    offset = useRef(0);
  useEffect(() => {
    let cancelled = false,
      timer;
    async function refresh() {
      try {
        const response = await api(`${path}/generations`);
        if (cancelled) return;
        const rows = response.items ?? [];
        setEnabled(!!response.enabled);
        setItems((old) => merge(rows, old));
        if (!offset.current) {
          offset.current = rows.length;
          setMore(rows.length === 100);
        }
        let detail;
        if (selected) {
          detail = await api(`${path}/generations/${selected}`);
          if (!cancelled) setJob(detail);
        }
        if (
          !cancelled &&
          (rows.some((r) => ["queued", "running"].includes(r.status)) ||
            ["queued", "running"].includes(detail?.status))
        )
          timer = setTimeout(refresh, 5000);
      } catch (e) {
        if (!cancelled) setError(e.message);
      }
    }
    void refresh();
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [api, path, selected, busy, refreshVersion]);
  async function queue() {
    setBusy(true);
    setError("");
    try {
      const body = {
        ...request,
        parts: request.workflow === "story_turn" ? 1 : Number(parts),
      };
      const signature = JSON.stringify(body);
      if (retry.current?.signature !== signature)
        retry.current = { signature, id: crypto.randomUUID() };
      const result = await api(`${path}/generations`, {
        ...body,
        id: retry.current.id,
      });
      retry.current = null;
      setSelected(result.id);
      setJob(result);
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  const creative = ["story_turn", "narrative_generate"].includes(
    request.workflow,
  );
  return (
    <fieldset disabled={busy}>
      <legend>Durable generations</legend>
      <button
        type="button"
        onClick={() => {
          setError("");
          setRefreshVersion((v) => v + 1);
        }}
      >
        Refresh generations
      </button>
      {error && <p role="alert">{error}</p>}
      {!enabled ? (
        <p>Background generation is not enabled.</p>
      ) : (
        <>
          {request.workflow === "narrative_generate" && (
            <label>
              Narrative parts
              <select value={parts} onChange={(e) => setParts(e.target.value)}>
                {[1, 2, 3, 4].map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </label>
          )}
          <button
            type="button"
            disabled={
              archived ||
              !creative ||
              !request.question ||
              (request.workflow === "story_turn" && !request.sessionId)
            }
            onClick={() => void queue()}
          >
            Queue creative generation
          </button>
          <p>
            Uses the current creative direction at revision {request.revision}.
            Each part uses its own allowance. Generated output remains a private
            candidate.
          </p>
        </>
      )}
      {items.map((item) => (
        <button
          type="button"
          key={item.id}
          onClick={() => setSelected(item.id)}
        >
          {item.question} · {item.status} · {item.step}/{item.parts}
        </button>
      ))}
      {more && (
        <button
          type="button"
          onClick={async () => {
            try {
              const result = await api(
                `${path}/generations?offset=${offset.current}`,
              );
              offset.current += result.items.length;
              setItems((old) => merge(result.items, old));
              setMore(result.items.length === 100);
            } catch (e) {
              setError(e.message);
            }
          }}
        >
          Load more generations
        </button>
      )}
      {job && (
        <article>
          <h3>{job.request.question}</h3>
          <p>
            {job.status} · completed {job.step}/{job.parts} parts
          </p>
          {job.errorCode && (
            <p>
              Stopped: {job.errorCode}. Inspect completed parts before starting
              a new request. Previous provider work may have used allowance.
            </p>
          )}
          {["queued", "running"].includes(job.status) && (
            <button
              type="button"
              disabled={archived}
              onClick={async () => {
                setBusy(true);
                try {
                  setJob(await api(`${path}/generations/${job.id}/cancel`, {}));
                } catch (e) {
                  setError(e.message);
                } finally {
                  setBusy(false);
                }
              }}
            >
              Cancel remaining parts
            </button>
          )}
          {job.status === "running" && (
            <p>
              An active provider call may finish before cancellation takes
              effect.
            </p>
          )}
          {job.results.map((result, index) => (
            <section key={index}>
              <h4>Part {index + 1}</h4>
              <p>{result.answer}</p>
              <button
                type="button"
                onClick={() => onCandidate(result, job.request)}
              >
                Inspect candidate from part {index + 1}
              </button>
            </section>
          ))}
        </article>
      )}
    </fieldset>
  );
}
