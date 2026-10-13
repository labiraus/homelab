import { humanLabel } from "./worldWorkspace.js";
import { useEffect, useState } from "react";

export default function WorldEntityPicker({
  api,
  path,
  revision,
  timeline,
  value,
  onChange,
}) {
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(null);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  const [offset, setOffset] = useState(0);
  useEffect(() => {
    let active = true;
    setError("");
    setPage(null);
    const timer = setTimeout(() => {
      const params = new URLSearchParams({
        kind: "entity",
        revision: String(revision),
        timeline,
        q: query,
        limit: "30",
        offset: String(offset),
      });
      api(`${path}/canon?${params}`)
        .then((result) => {
          if (active) setPage(result);
        })
        .catch((error) => {
          if (active) setError(error.message);
        });
    }, 180);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [api, path, revision, timeline, query, offset, retry]);
  // A changed pin needs a fresh cast selection, never silently reuse old IDs.
  useEffect(() => {
    onChange([]);
    setOffset(0);
  }, [revision, timeline]);
  return (
    <section aria-label="Story characters">
      <label>
        Find characters or places
        <input
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setOffset(0);
          }}
          placeholder="Search published names…"
        />
      </label>
      <p className="wb-muted">
        Published at revision {revision} · {timeline}. Choose up to 20.
      </p>
      {value.length > 0 && (
        <div className="wb-chips">
          {value.map((record) => (
            <button
              type="button"
              key={record.id}
              onClick={() => onChange(value.filter((r) => r.id !== record.id))}
            >
              Remove {record.label || "selected character"}
            </button>
          ))}
        </div>
      )}
      {error && (
        <p role="alert">
          {error}{" "}
          <button type="button" onClick={() => setRetry((n) => n + 1)}>
            Retry characters
          </button>
        </p>
      )}
      {!page && !error && <p role="status">Loading published characters…</p>}
      {page && (
        <>
          {!page.records.length && (
            <p>No matching characters or places at this revision.</p>
          )}
          <div className="wb-picker-results">
            {page.records
              .filter((record) => record.kind === "entity")
              .map((record) => (
                <label key={record.id}>
                  <input
                    type="checkbox"
                    checked={value.some((r) => r.id === record.id)}
                    disabled={
                      value.length >= 20 &&
                      !value.some((r) => r.id === record.id)
                    }
                    onChange={(e) =>
                      onChange(
                        e.target.checked
                          ? [...value, record]
                          : value.filter((r) => r.id !== record.id),
                      )
                    }
                  />
                  <span>
                    {record.label || "Unnamed entity"}
                    <small>{humanLabel(record.type || "Entity")}</small>
                  </span>
                </label>
              ))}
          </div>
          <div className="wb-chips">
            {offset > 0 && (
              <button
                type="button"
                onClick={() => setOffset(Math.max(0, offset - 30))}
              >
                Previous characters
              </button>
            )}
            {offset + page.records.length < page.total && (
              <button type="button" onClick={() => setOffset(offset + 30)}>
                More characters
              </button>
            )}
          </div>
        </>
      )}
    </section>
  );
}
