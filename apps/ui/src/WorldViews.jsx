import { useId, useMemo, useState } from "react";
import { exactTick } from "./worldTicks.js";
import { createCalendarFormatter } from "./worldCalendar.js";

const tick = (value) => BigInt(exactTick(value));
const link = (id, label = id) => <a href={`#canon-${id}`}>{label}</a>;
const compareID = (a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0);

function inScope(record, scope) {
  if (record.timeline && record.timeline !== scope.timeline) return false;
  if (scope.at === "") return true;
  // The API's decimal query syntax also accepts leading zeroes/a plus sign.
  // Normalise that already accepted filter without a floating-point conversion.
  const at = tick(
    typeof scope.at === "string" ? BigInt(scope.at).toString() : scope.at,
  );
  return (
    (record.from == null || tick(record.from) <= at) &&
    (record.until == null || at < tick(record.until))
  );
}

export default function WorldViews({ records, total, scope, calendar }) {
  const marker = useId();
  const formatTick = useMemo(
    () => createCalendarFormatter(calendar),
    [calendar],
  );
  const [focus, setFocus] = useState("");
  // Only published records returned for this inspected scope enter a view.
  // In particular, editor drafts and its separately fetched reference catalog do not.
  const visible = records.filter((record) => inScope(record, scope));
  const names = new Map(
    visible.map((record) => [record.id, record.label || record.id]),
  );
  const label = (id) => names.get(id) || id;
  const relationships = visible
    .filter((r) => r.kind === "relationship")
    .sort(compareID);
  const endpoints = [
    ...new Set(relationships.flatMap((r) => [r.subject, r.target])),
  ].sort();
  const activeFocus = endpoints.includes(focus) ? focus : "";
  const selected = relationships.filter(
    (r) =>
      !activeFocus || r.subject === activeFocus || r.target === activeFocus,
  );
  // Keep the diagram legible; the table and focus control still expose every
  // relationship in the loaded scope, including edges outside these bounds.
  const ids = new Set();
  const diagram = [];
  for (const edge of selected) {
    const next = new Set([...ids, edge.subject, edge.target]);
    if (next.size > 24 || diagram.length >= 36) continue;
    next.forEach((id) => ids.add(id));
    diagram.push(edge);
  }
  const columns = Math.max(1, Math.min(4, Math.ceil(Math.sqrt(ids.size))));
  const rows = Math.ceil(ids.size / columns);
  const height = Math.max(240, (rows - 1) * 96 + 160);
  const nodes = [...ids].sort().map((id, index) => ({
    id,
    x: (800 - (columns - 1) * 180) / 2 + (index % columns) * 180,
    y: (height - (rows - 1) * 96) / 2 + Math.floor(index / columns) * 96,
  }));
  const positions = new Map(nodes.map((node) => [node.id, node]));
  const events = visible
    .filter((r) => r.kind === "event" && r.from != null)
    .sort((a, b) => {
      const left = tick(a.from),
        right = tick(b.from);
      return left < right ? -1 : left > right ? 1 : compareID(a, b);
    });
  return (
    <section className="world-views" aria-label="Canon visualisations">
      <p>
        Revision {scope.revision}, timeline {scope.timeline}, fictional time{" "}
        {scope.at === "" ? "all times" : scope.at}.
        {scope.query ? ` Search: ${scope.query}.` : ""} Views cover{" "}
        {records.length} of {total} matching canon records.
        {records.length < total &&
          " Load more canon records to extend these views."}
        {calendar &&
          ` Dates use ${calendar.name} from revision ${scope.revision}; original ticks are shown alongside.`}
      </p>
      <details>
        <summary>Relationship graph</summary>
        <section aria-label="Relationship graph">
          <p>
            Arrows follow explicit subject → target relationships. Reference
            names come only from loaded canon; an unloaded reference is shown by
            its ID.
          </p>
          <label>
            Focus on entity
            <select
              value={activeFocus}
              onChange={(e) => setFocus(e.target.value)}
            >
              <option value="">All related entities</option>
              {endpoints.map((id) => (
                <option key={id} value={id}>
                  {label(id)} ({id})
                </option>
              ))}
            </select>
          </label>
          {selected.length === 0 ? (
            <p>No relationships in the loaded scope.</p>
          ) : (
            <>
              <p>
                Diagram shows {diagram.length} of {selected.length}{" "}
                relationships. The table lists all {selected.length} in this
                focus.
              </p>
              <div
                className="world-graph-scroll"
                tabIndex="0"
                aria-label="Relationship diagram; scroll horizontally on small screens"
              >
                <svg
                  className="world-graph"
                  viewBox={`0 0 800 ${height}`}
                  role="group"
                  aria-label="Directed relationships; record links also available in the table"
                >
                  <defs>
                    <marker
                      id={marker}
                      viewBox="0 0 10 10"
                      refX="10"
                      refY="5"
                      markerWidth="7"
                      markerHeight="7"
                      orient="auto-start-reverse"
                    >
                      <path d="M 0 0 L 10 5 L 0 10 z" fill="currentColor" />
                    </marker>
                  </defs>
                  {diagram.map((edge, index) => {
                    const a = positions.get(edge.subject),
                      b = positions.get(edge.target);
                    // Curves separate parallel/reverse edges; self-edges remain visible.
                    const dx = b.x - a.x,
                      dy = b.y - a.y;
                    const length = Math.hypot(dx, dy) || 1;
                    const bend = 22 + (index % 4) * 14;
                    const path =
                      edge.subject === edge.target
                        ? `M ${a.x - 8} ${a.y} C ${a.x - 80} ${a.y - 80}, ${a.x + 80} ${a.y - 80}, ${a.x + 8} ${a.y}`
                        : `M ${a.x} ${a.y} Q ${(a.x + b.x) / 2 - (dy / length) * bend} ${(a.y + b.y) / 2 + (dx / length) * bend}, ${b.x - (dx / length) * 11} ${b.y - (dy / length) * 11}`;
                    return (
                      <a
                        key={edge.id}
                        href={`#canon-${edge.id}`}
                        aria-label={`Inspect ${edge.predicate}: ${label(edge.subject)} to ${label(edge.target)}`}
                      >
                        <title>
                          {label(edge.subject)} → {label(edge.target)}:{" "}
                          {edge.predicate} ({edge.id})
                        </title>
                        <path className="world-graph-hit" d={path} />
                        <path
                          className="world-graph-edge"
                          d={path}
                          markerEnd={`url(#${marker})`}
                        />
                        <text
                          className="world-graph-edge-label"
                          x={
                            edge.subject === edge.target
                              ? a.x
                              : (a.x + b.x) / 2 - ((dy / length) * bend) / 2
                          }
                          y={
                            edge.subject === edge.target
                              ? a.y - 60
                              : (a.y + b.y) / 2 + ((dx / length) * bend) / 2
                          }
                          textAnchor="middle"
                        >
                          {edge.predicate.length > 20
                            ? `${edge.predicate.slice(0, 17)}…`
                            : edge.predicate}
                        </text>
                      </a>
                    );
                  })}
                  {nodes.map((node) => (
                    <a
                      key={node.id}
                      href={`#canon-${node.id}`}
                      aria-label={`Open ${label(node.id)}`}
                    >
                      <rect
                        x={node.x - 80}
                        y={node.y - 15}
                        width="160"
                        height="50"
                        fill="transparent"
                      />
                      <circle cx={node.x} cy={node.y} r="9" />
                      <text x={node.x} y={node.y + 25} textAnchor="middle">
                        {label(node.id).length > 20
                          ? `${label(node.id).slice(0, 17)}…`
                          : label(node.id)}
                      </text>
                      <title>
                        {label(node.id)} ({node.id})
                      </title>
                    </a>
                  ))}
                </svg>
              </div>
              <div className="world-view-table">
                <table>
                  <caption>Relationships in loaded canon</caption>
                  <thead>
                    <tr>
                      <th>Subject</th>
                      <th>Relationship</th>
                      <th>Target</th>
                      <th>Valid from</th>
                      <th>Valid until (exclusive)</th>
                      <th>Source</th>
                    </tr>
                  </thead>
                  <tbody>
                    {selected.map((r) => (
                      <tr key={r.id}>
                        <td>{link(r.subject, label(r.subject))}</td>
                        <td>{link(r.id, r.predicate)}</td>
                        <td>{link(r.target, label(r.target))}</td>
                        <td>
                          {r.from == null ? "Unbounded" : formatTick(r.from)}
                        </td>
                        <td>
                          {r.until == null ? "Unbounded" : formatTick(r.until)}
                        </td>
                        <td>{r.source && link(r.source, label(r.source))}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </section>
      </details>
      <details>
        <summary>Chronology</summary>
        <section aria-label="Chronology">
          <p>
            Events ordered by exact fictional start tick, then stable ID.
            Validity includes the start and excludes the end; spacing does not
            imply elapsed duration.
          </p>
          {events.length === 0 ? (
            <p>No events in the loaded scope.</p>
          ) : (
            <ol className="world-chronology">
              {events.map((event) => (
                <li key={event.id}>
                  <span className="world-chronology-tick">
                    {formatTick(event.from)}
                  </span>
                  <div>
                    {link(event.id, label(event.id))}
                    <p>
                      Until{" "}
                      {event.until == null
                        ? "unbounded"
                        : `${formatTick(event.until)} (exclusive)`}
                      {event.subject && (
                        <>
                          {" "}
                          · Participant:{" "}
                          {link(event.subject, label(event.subject))}
                        </>
                      )}
                      {event.source && (
                        <>
                          {" "}
                          · Source: {link(event.source, label(event.source))}
                        </>
                      )}
                    </p>
                  </div>
                </li>
              ))}
            </ol>
          )}
        </section>
      </details>
    </section>
  );
}
