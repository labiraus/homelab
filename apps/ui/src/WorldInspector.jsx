import { useEffect, useRef, useState } from "react";
import { RecordSummary } from "./WorldResponse.jsx";
import WorldConnections from "./WorldConnections.jsx";
import { worldHref } from "./worldWorkspace.js";

export default function WorldInspector({
  api,
  path,
  scope,
  onClose,
  onAsk,
  onEdit,
  canEdit,
}) {
  const [data, setData] = useState(null),
    [error, setError] = useState(""),
    [retry, setRetry] = useState(0),
    [connections, setConnections] = useState(null);
  const container = useRef(null);
  const [sheet, setSheet] = useState(
    () => window.matchMedia?.("(max-width: 1100px)").matches ?? false,
  );
  useEffect(() => {
    const media = window.matchMedia?.("(max-width: 1100px)");
    if (!media) return;
    const update = () => setSheet(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    if (!sheet) return;
    const siblings = [...container.current.parentElement.children].filter(
      (node) => node !== container.current,
    );
    const previous = siblings.map((node) => node.inert);
    siblings.forEach((node) => {
      node.inert = true;
    });
    return () =>
      siblings.forEach((node, i) => {
        node.inert = previous[i];
      });
  }, [sheet]);
  useEffect(() => {
    let active = true;
    setData(null);
    setError("");
    setConnections(null);
    const params = new URLSearchParams({
      revision: String(scope.revision),
      timeline: scope.timeline,
      ...(scope.at ? { at: scope.at } : {}),
    });
    api(`${path}/records/${encodeURIComponent(scope.id)}?${params}`)
      .then((value) => {
        if (active) setData(value);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [api, path, scope, retry]);
  return (
    <aside
      ref={container}
      className="wb-inspector"
      role={sheet ? "dialog" : undefined}
      aria-modal={sheet || undefined}
      aria-label="Source inspector"
      tabIndex={-1}
      onKeyDown={(e) => {
        if (e.key === "Escape") onClose();
        if (sheet && e.key === "Tab") {
          const items = [
            ...container.current.querySelectorAll(
              "button:not(:disabled),a[href],input,select,textarea,summary",
            ),
          ].filter((node) => node.getClientRects().length);
          if (e.shiftKey && document.activeElement === items[0]) {
            e.preventDefault();
            items.at(-1)?.focus();
          } else if (!e.shiftKey && document.activeElement === items.at(-1)) {
            e.preventDefault();
            items[0]?.focus();
          }
        }
      }}
    >
      <button onClick={onClose} autoFocus>
        Close inspector
      </button>
      <h2>Source inspector</h2>
      {data && <span className="wb-status">Published canon</span>}
      <p>
        Revision {scope.revision} · {scope.timeline}
        {scope.at ? ` · time ${scope.at}` : ""}
      </p>
      {!data && !error && <p role="status">Loading source…</p>}
      {error && (
        <div role="alert">
          {error}
          <button onClick={() => setRetry((v) => v + 1)}>Retry source</button>
        </div>
      )}
      {data && (
        <>
          <RecordSummary record={data.record} />
          <button onClick={() => onAsk({ ...scope, label: data.record.label })}>
            Ask AI about this
          </button>
          <button onClick={() => setConnections(scope)}>
            Show relationships
          </button>
          {canEdit && (
            <button onClick={() => onEdit(data.record, scope.revision)}>
              Edit source record
            </button>
          )}
          <a
            href={worldHref(scope.worldId, "explore", {
              record: scope.id,
              revision: scope.revision,
              timeline: scope.timeline,
              at: scope.at,
            })}
          >
            Link to this source
          </a>
        </>
      )}
      {connections && (
        <WorldConnections
          api={api}
          path={path}
          scope={connections}
          onExplore={setConnections}
          onClose={() => setConnections(null)}
        />
      )}
    </aside>
  );
}
