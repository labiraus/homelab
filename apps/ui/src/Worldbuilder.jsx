import { useEffect, useState } from "react";
import { parseWorldJSON } from "./worldTicks.js";
import { HistoryPanel, ReconciliationPanel } from "./WorldHistory.jsx";
import WorldRecordEditor from "./WorldRecordEditor.jsx";
import WorldAdministration from "./WorldAdministration.jsx";
import WorldStorySetup from "./WorldStorySetup.jsx";
import WorldGenerations from "./WorldGenerations.jsx";
import WorldConnections from "./WorldConnections.jsx";
import ProposalFindings, { Findings } from "./WorldFindings.jsx";

const base = import.meta.env.VITE_API_BASE_URL ?? "";
async function api(path, body) {
  const response = await fetch(`${base}/api/worlds${path}`, {
    method: body === undefined ? "GET" : "POST",
    headers:
      body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  // Successful responses must never hide a precision/parse failure as empty canon.
  let data;
  try {
    data = parseWorldJSON(text);
  } catch (error) {
    if (response.ok) throw error;
    data = {};
  }
  if (!response.ok)
    throw new Error(
      data.error ||
        (response.status === 404
          ? "Worldbuilder is not enabled, or you do not have access to this world."
          : "Worldbuilder request failed"),
    );
  return data;
}
const id = () => crypto.randomUUID();
const mergeRows = (before, after, key = "id") => [
  ...new Map([...before, ...after].map((row) => [row[key], row])).values(),
];
export default function Worldbuilder({ email }) {
  const [worlds, setWorlds] = useState([]),
    [selected, setSelected] = useState(""),
    [name, setName] = useState(""),
    [error, setError] = useState("");
  const [moreWorlds, setMoreWorlds] = useState(false);
  const [worldOffset, setWorldOffset] = useState(0);
  const [worldsBusy, setWorldsBusy] = useState(false);
  useEffect(() => {
    let active = true;
    api("")
      .then((rows) => {
        if (active) {
          setWorlds(rows);
          setWorldOffset(rows.length);
          setMoreWorlds(rows.length === 100);
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, []);
  async function create(e) {
    e.preventDefault();
    try {
      const w = await api("", { id: id(), name });
      const first = await api("");
      setWorlds(mergeRows(first, [w]));
      setWorldOffset(first.length);
      setMoreWorlds(first.length === 100);
      setSelected(w.id);
      setName("");
      setError("");
    } catch (e) {
      setError(e.message);
    }
  }
  return (
    <section className="workspace-shell worldbuilder">
      <header>
        <p className="workspace-eyebrow">Worldbuilder</p>
        <h1>Your worlds</h1>
        <p>Build shared canon, review changes and explore stories.</p>
      </header>
      {error && <p role="alert">{error}</p>}
      <form onSubmit={create}>
        <label>
          World name
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            maxLength={200}
          />
        </label>
        <button>Create world</button>
      </form>
      <label>
        Open world
        <select value={selected} onChange={(e) => setSelected(e.target.value)}>
          <option value="">Choose a world</option>
          {worlds.map((w) => (
            <option key={w.id} value={w.id}>
              {w.name}
              {w.archived ? " (archived)" : ""}
            </option>
          ))}
        </select>
      </label>
      {moreWorlds && (
        <button
          disabled={worldsBusy}
          onClick={async () => {
            setWorldsBusy(true);
            try {
              const rows = await api(`?offset=${worldOffset}&limit=100`);
              setWorlds(mergeRows(worlds, rows));
              setWorldOffset(worldOffset + rows.length);
              setMoreWorlds(rows.length === 100);
            } catch (e) {
              setError(e.message);
            } finally {
              setWorldsBusy(false);
            }
          }}
        >
          Load more worlds
        </button>
      )}
      {selected && (
        <World
          key={selected}
          worldId={selected}
          email={email}
          initialTimeline={
            worlds.find((w) => w.id === selected)?.defaultTimeline || "main"
          }
          onWorld={(w) => setWorlds((rows) => mergeRows(rows, [w]))}
        />
      )}
    </section>
  );
}
function World({ worldId, email, initialTimeline = "main", onWorld }) {
  const path = `/${worldId}`;
  const [canon, setCanon] = useState(null),
    [changes, setChanges] = useState([]),
    [sessions, setSessions] = useState([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const [timeline, setTimeline] = useState(initialTimeline),
    [revision, setRevision] = useState(""),
    [at, setAt] = useState(""),
    [query, setQuery] = useState("");
  const [operations, setOperations] = useState([]),
    [reason, setReason] = useState("");
  const [stagedRevision, setStagedRevision] = useState(null);
  const [stagedBefore, setStagedBefore] = useState({});
  const [reconcileTarget, setReconcileTarget] = useState(null);
  const [editing, setEditing] = useState(null);
  const [editorKey, setEditorKey] = useState(0);
  const [catalog, setCatalog] = useState([]);
  const [editingRevision, setEditingRevision] = useState(null);
  const [linkedRecord, setLinkedRecord] = useState(null);
  const [connectionScope, setConnectionScope] = useState(null);
  const [canonFilters, setCanonFilters] = useState(() =>
    new URLSearchParams({ timeline: initialTimeline, limit: "500" }).toString(),
  );
  const [listOffsets, setListOffsets] = useState({
    changes: 0,
    sessions: 0,
    turns: 0,
  });
  const [moreLists, setMoreLists] = useState({});
  const [reviewReason, setReviewReason] = useState("");
  const [session, setSession] = useState(null),
    [turns, setTurns] = useState([]),
    [premiseCandidate, setPremiseCandidate] = useState(""),
    [narration, setNarration] = useState("");
  const [question, setQuestion] = useState(""),
    [workflow, setWorkflow] = useState("canon_query"),
    [answer, setAnswer] = useState(null);
  async function reload() {
    const params = new URLSearchParams({ timeline, limit: "500" });
    if (revision !== "") params.set("revision", revision);
    if (at !== "") params.set("at", at);
    if (query) params.set("q", query);
    const [c, d, s] = await Promise.all([
      api(`${path}/${query ? "search" : "canon"}?${params}`),
      api(`${path}/changes`),
      api(`${path}/sessions`),
    ]);
    setCanon(c);
    onWorld?.(c.world);
    setCanonFilters(params.toString());
    setChanges(d);
    setSessions(s);
    setListOffsets((old) => ({
      ...old,
      changes: d.length,
      sessions: s.length,
    }));
    setMoreLists((old) => ({
      ...old,
      changes: d.length === 100,
      sessions: s.length === 100,
    }));
  }
  useEffect(() => {
    let active = true;
    Promise.all([
      api(
        `${path}/canon?${new URLSearchParams({ timeline: initialTimeline, limit: "500" })}`,
      ),
      api(`${path}/changes`),
      api(`${path}/sessions`),
    ])
      .then(([c, d, s]) => {
        if (active) {
          setCanon(c);
          setChanges(d);
          setSessions(s);
          setListOffsets((old) => ({
            ...old,
            changes: d.length,
            sessions: s.length,
          }));
          setMoreLists((old) => ({
            ...old,
            changes: d.length === 100,
            sessions: s.length === 100,
          }));
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [path]);
  useEffect(() => {
    if (!canon) return;
    let active = true;
    async function loadCatalog() {
      const result = [];
      let offset = 0;
      while (true) {
        const params = new URLSearchParams({
          timeline: canon.timeline || timeline,
          revision: String(canon.revision),
          kind: "type,entity,source,timeline",
          limit: "500",
          offset: String(offset),
        });
        const page = await api(`${path}/canon?${params}`);
        result.push(...page.records);
        offset += page.records.length;
        if (offset >= page.total || page.records.length === 0) break;
      }
      if (active) setCatalog(result);
    }
    void loadCatalog().catch((e) => {
      if (active) setError(e.message);
    });
    return () => {
      active = false;
    };
  }, [path, canon?.revision, canon?.timeline]);
  async function run(fn) {
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  function stage(operation) {
    if (operations.length && stagedRevision !== canon.revision) {
      setError(
        "Load the draft's original revision or clear staged changes before editing another revision.",
      );
      return;
    }
    if (!operations.length) {
      setStagedRevision(canon.revision);
      setStagedBefore(Object.fromEntries(canon.records.map((r) => [r.id, r])));
    }
    setOperations((current) => [
      ...current.filter((op) => op.id !== operation.id),
      operation,
    ]);
  }
  async function propose(
    ops = operations,
    baseRevision = stagedRevision ?? canon.revision,
  ) {
    await api(`${path}/changes`, {
      id: id(),
      baseRevision,
      reason,
      operations: ops,
    });
    setOperations([]);
    setStagedRevision(null);
    await reload();
  }
  async function resume(s) {
    setSession(s);
    const rows = await api(`${path}/sessions/${s.id}/turns`);
    setTurns(rows);
    setListOffsets((old) => ({ ...old, turns: rows.length }));
    setMoreLists((old) => ({ ...old, turns: rows.length === 100 }));
    setAnswer(null);
  }
  async function saveTurn(
    text = narration,
    ops = operations,
    candidate = null,
  ) {
    await api(`${path}/sessions/${session.id}/turns`, {
      requestId: id(),
      expectedVersion: candidate?.sessionVersion ?? session.version,
      narration: text,
      operations: ops,
      ...(candidate
        ? {
            memorySummary: candidate.memorySummary,
            choices: candidate.choices,
            direction: candidate.direction,
          }
        : {}),
    });
    await resume(await api(`${path}/sessions/${session.id}`));
    setNarration("");
    setOperations([]);
    await reload();
  }
  async function loadMoreCanon() {
    const params = new URLSearchParams(canonFilters);
    params.set("revision", String(canon.revision));
    params.set("offset", String(canon.records.length));
    const next = await api(
      `${path}/${params.get("q") ? "search" : "canon"}?${params}`,
    );
    setCanon({ ...canon, records: mergeRows(canon.records, next.records) });
  }
  async function loadMoreList(kind) {
    const suffix = kind === "turns" ? `sessions/${session.id}/turns` : kind;
    const rows = await api(
      `${path}/${suffix}?limit=100&offset=${listOffsets[kind]}`,
    );
    if (kind === "changes") setChanges(mergeRows(changes, rows));
    if (kind === "sessions") setSessions(mergeRows(sessions, rows));
    if (kind === "turns") setTurns(mergeRows(turns, rows, "version"));
    setListOffsets({ ...listOffsets, [kind]: listOffsets[kind] + rows.length });
    setMoreLists({ ...moreLists, [kind]: rows.length === 100 });
  }
  function listMore(kind, caption) {
    return (
      moreLists[kind] && (
        <button onClick={() => void run(() => loadMoreList(kind))}>
          Load more {caption}
        </button>
      )
    );
  }
  function editRecord(record, recordRevision = canon.revision) {
    setEditing(record);
    setEditingRevision(recordRevision);
    setEditorKey((key) => key + 1);
  }
  const canEdit =
    canon &&
    !canon.world.archived &&
    ["owner", "admin", "editor"].includes(canon.world.role);
  const canReview =
    canon &&
    !canon.world.archived &&
    ["owner", "admin", "reviewer"].includes(canon.world.role);
  const records = canon?.records ?? [];
  const staged = new Map([...catalog, ...records].map((r) => [r.id, r]));
  for (const op of operations) {
    if (op.action === "delete") staged.delete(op.id);
    else staged.set(op.id, op.record);
  }
  const editorRecords = [...staged.values()];
  return (
    <div
      aria-busy={busy}
      onClick={(e) => {
        const a = e.target.closest?.('a[href^="#canon-"]');
        if (a) {
          e.preventDefault();
          const target = document.getElementById(
            a.getAttribute("href").slice(1),
          );
          if (target) target.scrollIntoView({ behavior: "smooth" });
          else if (canon)
            void run(async () => {
              const recordId = a.getAttribute("href").slice("#canon-".length);
              const params = new URLSearchParams({
                timeline: canon.timeline || timeline,
                revision: String(canon.revision),
              });
              setLinkedRecord(
                await api(
                  `${path}/records/${encodeURIComponent(recordId)}?${params}`,
                ),
              );
            });
        }
      }}
    >
      {error && <p role="alert">{error}</p>}
      {canon && (
        <>
          <h2>
            {canon.world.name} · canon revision {canon.world.revision}
          </h2>
          <p>
            Your role: {canon.world.role}. Published history is preserved;
            drafts and story state remain separate.
          </p>
          <fieldset disabled={busy}>
            <legend>Wiki and history</legend>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void run(reload);
              }}
            >
              <label>
                Timeline
                <input
                  value={timeline}
                  onChange={(e) => setTimeline(e.target.value)}
                />
              </label>
              <label>
                Historical revision
                <input
                  type="number"
                  min="0"
                  placeholder="Current"
                  value={revision}
                  onChange={(e) => setRevision(e.target.value)}
                />
              </label>
              <label>
                Fictional time
                <input
                  type="text"
                  inputMode="numeric"
                  pattern="-?[0-9]+"
                  value={at}
                  onChange={(e) => setAt(e.target.value)}
                  placeholder="All times"
                />
              </label>
              <label>
                Search canon
                <input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
              </label>
              <button>Load canon</button>
            </form>
            <p>
              Showing {records.length} of {canon.total} records at revision{" "}
              {canon.revision}. Fictional validity includes its start and
              excludes its end.
            </p>
            <div className="world-records">
              {records.map((r) => (
                <article id={`canon-${r.id}`} key={r.id}>
                  <h3>
                    {r.label || r.id} <small>{r.kind}</small>
                  </h3>
                  <p>
                    {r.subject && (
                      <a href={`#canon-${r.subject}`}>{r.subject}</a>
                    )}{" "}
                    {r.predicate}{" "}
                    {r.target && <a href={`#canon-${r.target}`}>{r.target}</a>}{" "}
                    {r.value !== undefined && String(r.value)}
                  </p>
                  {r.source && (
                    <p>
                      Source: <a href={`#canon-${r.source}`}>{r.source}</a>
                    </p>
                  )}
                  {r.from !== undefined && (
                    <p>
                      From {r.from}
                      {r.until !== undefined ? ` until ${r.until}` : " onwards"}
                    </p>
                  )}
                  {r.fields && (
                    <p>
                      Fields:{" "}
                      {Object.entries(r.fields)
                        .map(([k, v]) => `${k} (${v})`)
                        .join(", ")}
                    </p>
                  )}
                  {r.attributes && (
                    <dl>
                      {Object.entries(r.attributes).map(([key, value]) => (
                        <div key={key}>
                          <dt>{key}</dt>
                          <dd>{value}</dd>
                        </div>
                      ))}
                    </dl>
                  )}
                  <button
                    type="button"
                    onClick={() =>
                      setConnectionScope({
                        id: r.id,
                        revision: canon.revision,
                        timeline:
                          new URLSearchParams(canonFilters).get("timeline") ||
                          timeline,
                        at: new URLSearchParams(canonFilters).get("at") || "",
                      })
                    }
                  >
                    Explore connections of {r.label || r.id}
                  </button>
                  {canEdit && r.kind !== "world_settings" && (
                    <button
                      type="button"
                      onClick={() => {
                        editRecord(staged.get(r.id) || r);
                      }}
                    >
                      Edit {r.label || r.id}
                    </button>
                  )}
                  {canEdit && (
                    <button
                      type="button"
                      onClick={() => stage({ action: "delete", id: r.id })}
                    >
                      Propose removal
                    </button>
                  )}
                </article>
              ))}
            </div>
          </fieldset>
          {canon.records.length < canon.total && (
            <button disabled={busy} onClick={() => void run(loadMoreCanon)}>
              Load more canon records
            </button>
          )}
          {connectionScope && (
            <WorldConnections
              key={JSON.stringify(connectionScope)}
              api={api}
              path={path}
              scope={connectionScope}
              onExplore={setConnectionScope}
              onClose={() => setConnectionScope(null)}
            />
          )}
          {linkedRecord && (
            <aside aria-label="Linked canon record">
              <h3>{linkedRecord.record.label || linkedRecord.record.id}</h3>
              <p>Revision {linkedRecord.revision}</p>
              <pre>{JSON.stringify(linkedRecord.record, null, 2)}</pre>
              {canEdit && (
                <button
                  onClick={() =>
                    editRecord(linkedRecord.record, linkedRecord.revision)
                  }
                >
                  Edit linked record
                </button>
              )}
              <button onClick={() => setLinkedRecord(null)}>
                Close linked record
              </button>
            </aside>
          )}
          <HistoryPanel
            api={api}
            path={path}
            currentRevision={canon.world.revision}
            canEdit={canEdit}
            onSaved={reload}
          />
          {reconcileTarget && (
            <ReconciliationPanel
              key={`${reconcileTarget.kind}/${reconcileTarget.id}`}
              api={api}
              path={path}
              target={reconcileTarget}
              canEdit={canEdit}
              onSaved={reload}
              onClose={() => setReconcileTarget(null)}
            />
          )}
          {canEdit && (
            <fieldset disabled={busy}>
              <legend>Structured editor</legend>
              <WorldRecordEditor
                key={editorKey}
                record={editing}
                onNew={() => {
                  setEditing(null);
                  setEditorKey((key) => key + 1);
                }}
                records={editorRecords}
                timeline={timeline}
                onStage={(record) => {
                  if (editing && editingRevision !== canon.revision) {
                    setError(
                      "Load the edited record's original revision before staging it.",
                    );
                    return;
                  }
                  stage({ action: "put", id: record.id, record });
                }}
              />
              <h3>Proposed changes</h3>
              {operations.length > 0 && (
                <p>
                  Draft based on revision {stagedRevision}. Refreshing canon
                  does not rebase staged changes.
                </p>
              )}
              {operations.map((op) => (
                <div key={op.id}>
                  <strong>
                    {op.action} {op.id}
                  </strong>
                  <details>
                    <summary>Before and after</summary>
                    <pre>
                      {JSON.stringify(
                        {
                          before: stagedBefore[op.id] ?? null,
                          after: op.record ?? null,
                        },
                        null,
                        2,
                      )}
                    </pre>
                  </details>
                  <button
                    onClick={() =>
                      setOperations(operations.filter((x) => x.id !== op.id))
                    }
                  >
                    Remove from draft
                  </button>
                </div>
              ))}
              <label>
                Reason for change
                <input
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  maxLength={2000}
                />
              </label>
              <button
                disabled={!operations.length || !reason}
                onClick={() => void run(() => propose())}
              >
                Save draft proposal
              </button>
            </fieldset>
          )}
          {canon.world.archived && (
            <p role="status">
              This world is archived. History remains readable; an owner can
              reopen it.
            </p>
          )}
          {canon.world.role === "owner" && (
            <WorldAdministration
              api={api}
              path={path}
              world={canon.world}
              onSaved={reload}
            />
          )}
          <fieldset disabled={busy}>
            <legend>Review queue</legend>
            <p>
              Policy: a different owner, admin or reviewer must approve.
              Publication rechecks current roles, canon revision and
              deterministic rules; AI cannot approve.
            </p>
            <label>
              Review reason
              <input
                value={reviewReason}
                onChange={(e) => setReviewReason(e.target.value)}
                maxLength={2000}
              />
            </label>
            {changes.map((c) => (
              <article key={c.id}>
                <h3>{c.reason}</h3>
                <p>
                  {c.status} · based on revision {c.baseRevision} · by{" "}
                  {c.author}
                </p>
                <details>
                  <summary>Inspect operations</summary>
                  <pre>{JSON.stringify(c.operations, null, 2)}</pre>
                </details>
                <button
                  onClick={() =>
                    setReconcileTarget({ kind: "changes", id: c.id })
                  }
                >
                  Compare proposal with current canon
                </button>
                <ProposalFindings
                  api={api}
                  path={path}
                  change={c}
                  timeline={timeline}
                  archived={canon.world.archived}
                />
                {c.status === "draft" && c.author === email && canEdit && (
                  <button
                    onClick={() =>
                      void run(async () => {
                        await api(`${path}/changes/${c.id}/submit`, {});
                        await reload();
                      })
                    }
                  >
                    Submit for review
                  </button>
                )}
                {c.status === "ready_for_review" &&
                  canReview &&
                  c.author !== email &&
                  ["approved", "rejected"].map((decision) => (
                    <button
                      key={decision}
                      disabled={!reviewReason}
                      onClick={() =>
                        void run(async () => {
                          await api(`${path}/changes/${c.id}/review`, {
                            decision,
                            reason: reviewReason,
                          });
                          await reload();
                        })
                      }
                    >
                      {decision === "approved" ? "Approve" : "Reject"}
                    </button>
                  ))}
                {c.status === "approved" && canReview && (
                  <button
                    onClick={() =>
                      void run(async () => {
                        await api(`${path}/changes/${c.id}/publish`, {});
                        await reload();
                      })
                    }
                  >
                    Publish approved change
                  </button>
                )}
                {c.reviewer && (
                  <p>
                    Reviewed by {c.reviewer}: {c.reviewReason}
                  </p>
                )}
              </article>
            ))}
            {listMore("changes", "proposals")}
          </fieldset>
          <fieldset disabled={busy}>
            <legend>Interactive stories</legend>
            <WorldStorySetup
              api={api}
              path={path}
              revision={canon.revision}
              timeline={timeline}
              archived={canon.world.archived}
              premiseCandidate={premiseCandidate}
              onStart={(s) =>
                run(async () => {
                  await resume(s);
                  await reload();
                })
              }
            />
            {sessions.map((s) => (
              <button key={s.id} onClick={() => void run(() => resume(s))}>
                Resume {s.title}
              </button>
            ))}
            {listMore("sessions", "stories")}
            {session && (
              <>
                <h3>{session.title}</h3>
                <p>
                  Pinned revision {session.pinnedRevision}, timeline{" "}
                  {session.timeline}, turn {session.version}. Your provisional
                  state is private to this session.
                </p>
                {session.definition && (
                  <p>Premise: {session.definition.premise}</p>
                )}
                {session.memory?.summary && (
                  <p>
                    Provisional summary through turn{" "}
                    {session.memory.throughVersion}: {session.memory.summary}
                  </p>
                )}
                {session.memory?.choices?.map((choice) => (
                  <button
                    key={choice.id}
                    disabled={canon.world.archived}
                    onClick={() => {
                      setWorkflow("story_turn");
                      setQuestion(choice.direction);
                      setAnswer(null);
                    }}
                  >
                    {choice.label}
                  </button>
                ))}
                {[...turns].reverse().map((t) => (
                  <article key={t.version}>
                    <h4>Turn {t.version}</h4>
                    <p>{t.narration}</p>
                  </article>
                ))}
                {listMore("turns", "turns")}
                <details>
                  <summary>Provisional state</summary>
                  <pre>{JSON.stringify(session.state, null, 2)}</pre>
                </details>
                <label>
                  Next narration
                  <textarea
                    value={narration}
                    onChange={(e) => setNarration(e.target.value)}
                    maxLength={16000}
                  />
                </label>
                <button
                  disabled={!narration || canon.world.archived}
                  onClick={() => void run(() => saveTurn())}
                >
                  Save turn with staged changes
                </button>
                {canEdit && (
                  <button
                    disabled={!Object.keys(session.state).length}
                    onClick={() =>
                      setReconcileTarget({ kind: "sessions", id: session.id })
                    }
                  >
                    Reconcile story outcome for review
                  </button>
                )}
                <p>
                  Use the editor to stage provisional facts, relationships or
                  events before saving a turn. An outcome requires a reason and
                  independent review.
                </p>
              </>
            )}
          </fieldset>
          <fieldset disabled={busy || canon.world.archived}>
            <legend>Canon-grounded AI</legend>
            <label>
              Workflow
              <select
                value={workflow}
                onChange={(e) => {
                  setWorkflow(e.target.value);
                  setAnswer(null);
                }}
              >
                {[
                  "canon_query",
                  "canon_proposal",
                  "consistency_check",
                  "story_turn",
                  "narrative_generate",
                ].map((x) => (
                  <option key={x} value={x}>
                    {x.replaceAll("_", " ")}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Question or creative direction
              <textarea
                value={question}
                onChange={(e) => setQuestion(e.target.value)}
                maxLength={4000}
              />
            </label>
            <button
              disabled={!question || (workflow === "story_turn" && !session)}
              onClick={() =>
                void run(async () => {
                  setAnswer(null);
                  const value = await api(`${path}/reason`, {
                    workflow,
                    question,
                    timeline:
                      workflow === "story_turn" ? session.timeline : timeline,
                    revision:
                      workflow === "story_turn"
                        ? session.pinnedRevision
                        : canon.revision,
                    ...(workflow === "story_turn"
                      ? {
                          sessionId: session.id,
                          expectedVersion: session.version,
                        }
                      : {}),
                  });
                  setAnswer({
                    ...value,
                    workflow,
                    direction: question,
                    timeline:
                      workflow === "story_turn" ? session.timeline : timeline,
                  });
                })
              }
            >
              Generate candidate
            </button>
            <WorldGenerations
              api={api}
              path={path}
              archived={canon.world.archived}
              request={{
                workflow,
                question,
                timeline:
                  workflow === "story_turn" ? session?.timeline : timeline,
                revision:
                  workflow === "story_turn"
                    ? session?.pinnedRevision
                    : canon.revision,
                ...(workflow === "story_turn"
                  ? {
                      sessionId: session?.id,
                      expectedVersion: session?.version,
                    }
                  : {}),
              }}
              onCandidate={(result, input) =>
                run(async () => {
                  if (input.workflow === "story_turn")
                    await resume(
                      await api(`${path}/sessions/${input.sessionId}`),
                    );
                  setAnswer({
                    ...result,
                    workflow: input.workflow,
                    direction: input.question,
                    timeline: input.timeline,
                  });
                })
              }
            />
            {answer && (
              <article>
                <p>{answer.answer}</p>
                {answer.workflow === "consistency_check" && (
                  <Findings result={answer} />
                )}
                {answer.evidenceScope?.coverage === "selected_records" && (
                  <p>
                    Based on {answer.evidenceScope.evidenceCount} selected canon
                    records out of {answer.evidenceScope.canonRecordCount}.
                    Other canon may affect this answer; this is not a
                    whole-world consistency check.
                  </p>
                )}
                {answer.unknown && (
                  <p>Uncertainty: this is not established in canon.</p>
                )}
                {answer.provenance?.requestId && (
                  <details>
                    <summary>Generation details</summary>
                    <p>Model: {answer.model || "not reported"}</p>
                    <p>Request: {answer.provenance.requestId}</p>
                    <p>Duration: {answer.provenance.latencyMs} ms</p>
                    <p>
                      Budget allocated:{" "}
                      {(answer.provenance.allocatedMicros / 1000000).toFixed(6)}{" "}
                      {answer.provenance.currency}. This is an allowance, not an
                      invoice.
                    </p>
                  </details>
                )}
                <ul>
                  {answer.citations.map((c) => (
                    <li key={c}>
                      <a
                        href={`${base}/api/worlds/${c.split(":")[0]}/records/${c.split(":").at(-1)}?revision=${c.split(":")[1]}&timeline=${c.split(":")[2]}`}
                        target="_blank"
                        rel="noreferrer"
                      >
                        {c}
                      </a>
                    </li>
                  ))}
                </ul>
                <details>
                  <summary>Candidate changes</summary>
                  <pre>{JSON.stringify(answer.operations, null, 2)}</pre>
                </details>
                {answer.memorySummary && (
                  <p>Proposed provisional summary: {answer.memorySummary}</p>
                )}
                {answer.choices?.length > 0 && (
                  <ul>
                    {answer.choices.map((choice) => (
                      <li key={choice.id}>
                        {choice.label}: {choice.direction}
                      </li>
                    ))}
                  </ul>
                )}
                {answer.workflow === "narrative_generate" && (
                  <button
                    onClick={() =>
                      setPremiseCandidate({
                        text: answer.answer,
                        revision: answer.baseRevision,
                        timeline: answer.timeline,
                      })
                    }
                  >
                    Use as a private story premise
                  </button>
                )}
                {answer.workflow === "canon_proposal" && canEdit && (
                  <button
                    disabled={!reason || !answer.operations.length}
                    onClick={() =>
                      void run(() =>
                        propose(answer.operations, answer.baseRevision),
                      )
                    }
                  >
                    Save AI candidate as draft
                  </button>
                )}
                {answer.workflow === "story_turn" && session && (
                  <button
                    onClick={() =>
                      void run(() =>
                        saveTurn(answer.answer, answer.operations, answer),
                      )
                    }
                  >
                    Save candidate as provisional turn
                  </button>
                )}
              </article>
            )}
          </fieldset>
        </>
      )}
    </div>
  );
}
