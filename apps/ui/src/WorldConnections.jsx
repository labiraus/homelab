import { useEffect, useState } from "react";
export default function WorldConnections({
  api,
  path,
  scope,
  onExplore,
  onClose,
}) {
  const [rows, setRows] = useState([]),
    [total, setTotal] = useState(0),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const url = (offset) => {
    const query = new URLSearchParams({
      revision: String(scope.revision),
      timeline: scope.timeline,
      limit: "100",
      offset: String(offset),
    });
    if (scope.at) query.set("at", scope.at);
    return `${path}/records/${encodeURIComponent(scope.id)}/connections?${query}`;
  };
  useEffect(() => {
    let active = true;
    api(url(0))
      .then((result) => {
        if (active) {
          setRows(result.records);
          setTotal(result.total);
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [api, path, scope]);
  return (
    <aside aria-label="Canon connections">
      <h3>Connections of {scope.id}</h3>
      <p>
        Revision {scope.revision}, timeline {scope.timeline}
        {scope.at ? `, fictional time ${scope.at}` : ""}. Explicit references
        only.
      </p>
      {error && <p role="alert">{error}</p>}
      <ul>
        {rows.map((record) => (
          <li key={record.id}>
            <button
              type="button"
              onClick={() => onExplore({ ...scope, id: record.id })}
            >
              {record.label || record.id} · {record.kind}
            </button>{" "}
            {record.subject} {record.predicate} {record.target}{" "}
            {record.value === undefined ? "" : String(record.value)}
          </li>
        ))}
      </ul>
      {rows.length < total && (
        <button
          type="button"
          disabled={busy}
          onClick={async () => {
            setBusy(true);
            try {
              const result = await api(url(rows.length));
              setRows((old) => [...old, ...result.records]);
              setTotal(result.total);
            } catch (e) {
              setError(e.message);
            } finally {
              setBusy(false);
            }
          }}
        >
          Load more connections
        </button>
      )}
      <button type="button" onClick={onClose}>
        Close connections
      </button>
    </aside>
  );
}
