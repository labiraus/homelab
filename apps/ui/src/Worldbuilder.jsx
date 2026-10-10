import { useEffect, useState } from "react";
import { HistoryPanel, ReconciliationPanel } from "./WorldHistory.jsx";

const base = import.meta.env.VITE_API_BASE_URL ?? "";
async function api(path, body) {
  const response = await fetch(`${base}/api/worlds${path}`, {
    method: body === undefined ? "GET" : "POST",
    headers:
      body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await response.json().catch(() => ({}));
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
export default function Worldbuilder({ email }) {
  const [worlds, setWorlds] = useState([]),
    [selected, setSelected] = useState(""),
    [name, setName] = useState(""),
    [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    api("")
      .then((rows) => {
        if (active) setWorlds(rows);
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
      setWorlds(await api(""));
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
            </option>
          ))}
        </select>
      </label>
      {selected && <World key={selected} worldId={selected} email={email} />}
    </section>
  );
}
function World({ worldId, email }) {
  const path = `/${worldId}`;
  const [canon, setCanon] = useState(null),
    [changes, setChanges] = useState([]),
    [sessions, setSessions] = useState([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const [timeline, setTimeline] = useState("main"),
    [revision, setRevision] = useState(""),
    [at, setAt] = useState(""),
    [query, setQuery] = useState("");
  const [operations, setOperations] = useState([]),
    [reason, setReason] = useState("");
  const [stagedRevision, setStagedRevision] = useState(null);
  const [stagedBefore, setStagedBefore] = useState({});
  const [reconcileTarget, setReconcileTarget] = useState(null);
  const [member, setMember] = useState(""),
    [role, setRole] = useState("editor"),
    [reviewReason, setReviewReason] = useState("");
  const [session, setSession] = useState(null),
    [turns, setTurns] = useState([]),
    [storyTitle, setStoryTitle] = useState(""),
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
      api(`${path}/canon?${params}`),
      api(`${path}/changes`),
      api(`${path}/sessions`),
    ]);
    setCanon(c);
    setChanges(d);
    setSessions(s);
  }
  useEffect(() => {
    let active = true;
    Promise.all([
      api(`${path}/canon?timeline=main&limit=500`),
      api(`${path}/changes`),
      api(`${path}/sessions`),
    ])
      .then(([c, d, s]) => {
        if (active) {
          setCanon(c);
          setChanges(d);
          setSessions(s);
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [path]);
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
    setTurns(await api(`${path}/sessions/${s.id}/turns`));
    setAnswer(null);
  }
  async function saveTurn(text = narration, ops = operations) {
    await api(`${path}/sessions/${session.id}/turns`, {
      requestId: id(),
      expectedVersion: session.version,
      narration: text,
      operations: ops,
    });
    await resume(await api(`${path}/sessions/${session.id}`));
    setNarration("");
    setOperations([]);
    await reload();
  }
  const canEdit =
    canon && ["owner", "admin", "editor"].includes(canon.world.role);
  const canReview =
    canon && ["owner", "admin", "reviewer"].includes(canon.world.role);
  const records = canon?.records ?? [];
  const staged = new Map(records.map((r) => [r.id, r]));
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
          document
            .getElementById(a.getAttribute("href").slice(1))
            ?.scrollIntoView({ behavior: "smooth" });
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
                  type="number"
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
              <RecordEditor
                records={editorRecords}
                timeline={timeline}
                onStage={(record) =>
                  stage({ action: "put", id: record.id, record })
                }
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
          {canon.world.role === "owner" && (
            <fieldset disabled={busy}>
              <legend>World membership</legend>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  void run(async () => {
                    await api(`${path}/members`, { principal: member, role });
                    setMember("");
                  });
                }}
              >
                <label>
                  Member email
                  <input
                    type="email"
                    required
                    value={member}
                    onChange={(e) => setMember(e.target.value)}
                  />
                </label>
                <label>
                  Role
                  <select
                    value={role}
                    onChange={(e) => setRole(e.target.value)}
                  >
                    {["editor", "reviewer", "reader", "admin"].map((r) => (
                      <option key={r}>{r}</option>
                    ))}
                  </select>
                </label>
                <button>Set membership</button>
              </form>
              <p>
                Add an independent reviewer before publishing your first
                proposal.
              </p>
            </fieldset>
          )}
          <fieldset disabled={busy}>
            <legend>Review queue</legend>
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
                {c.status === "draft" && c.author === email && (
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
          </fieldset>
          <fieldset disabled={busy}>
            <legend>Interactive stories</legend>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void run(async () => {
                  const s = await api(`${path}/sessions`, {
                    id: id(),
                    title: storyTitle,
                    pinnedRevision: canon.revision,
                    timeline,
                  });
                  await resume(s);
                  await reload();
                });
              }}
            >
              <label>
                Story title
                <input
                  required
                  value={storyTitle}
                  onChange={(e) => setStoryTitle(e.target.value)}
                  maxLength={200}
                />
              </label>
              <button>Start at revision {canon.revision}</button>
            </form>
            {sessions.map((s) => (
              <button key={s.id} onClick={() => void run(() => resume(s))}>
                Resume {s.title}
              </button>
            ))}
            {session && (
              <>
                <h3>{session.title}</h3>
                <p>
                  Pinned revision {session.pinnedRevision}, timeline{" "}
                  {session.timeline}, turn {session.version}. Your provisional
                  state is private to this session.
                </p>
                {[...turns].reverse().map((t) => (
                  <article key={t.version}>
                    <h4>Turn {t.version}</h4>
                    <p>{t.narration}</p>
                  </article>
                ))}
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
                  disabled={!narration}
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
          <fieldset disabled={busy}>
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
                      ? { sessionId: session.id }
                      : {}),
                  });
                  setAnswer({ ...value, workflow });
                })
              }
            >
              Generate candidate
            </button>
            {answer && (
              <article>
                <p>{answer.answer}</p>
                {answer.unknown && (
                  <p>Uncertainty: this is not established in canon.</p>
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
                      void run(() => saveTurn(answer.answer, answer.operations))
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
function RecordEditor({ records, timeline, onStage }) {
  const [kind, setKind] = useState("entity"),
    [key, setKey] = useState(""),
    [label, setLabel] = useState(""),
    [type, setType] = useState(""),
    [subject, setSubject] = useState(""),
    [target, setTarget] = useState(""),
    [source, setSource] = useState(""),
    [predicate, setPredicate] = useState(""),
    [valueType, setValueType] = useState("string"),
    [value, setValue] = useState(""),
    [from, setFrom] = useState(""),
    [until, setUntil] = useState("");
  const choose = (caption, v, set, kind) => (
    <label>
      {caption}
      <select value={v} onChange={(e) => set(e.target.value)}>
        <option value="">Choose {kind}</option>
        {records
          .filter((r) => r.kind === kind)
          .map((r) => (
            <option key={r.id} value={r.id}>
              {r.label || r.id}
            </option>
          ))}
      </select>
    </label>
  );
  function stage(e) {
    e.preventDefault();
    const r = { id: key, kind, ...(label ? { label } : {}) };
    if (kind === "entity") r.type = type;
    if (["fact", "relationship", "event"].includes(kind)) {
      Object.assign(r, { timeline, source });
      if (subject) r.subject = subject;
      if (from !== "") r.from = Number(from);
      if (until !== "") r.until = Number(until);
    }
    if (kind === "fact")
      Object.assign(r, {
        predicate,
        valueType,
        value:
          valueType === "number"
            ? Number(value)
            : valueType === "boolean"
              ? value === "true"
              : value,
      });
    if (kind === "relationship") Object.assign(r, { predicate, target });
    if (kind === "type") {
      r.fields = predicate ? { [predicate]: valueType } : {};
      r.relations = target ? { [value]: target } : {};
    }
    onStage(r);
  }
  return (
    <form onSubmit={stage}>
      <label>
        Record kind
        <select value={kind} onChange={(e) => setKind(e.target.value)}>
          {[
            "entity",
            "type",
            "source",
            "timeline",
            "fact",
            "relationship",
            "event",
            "rule",
          ].map((k) => (
            <option key={k}>{k}</option>
          ))}
        </select>
      </label>
      <label>
        Stable ID
        <input
          required
          pattern="[a-zA-Z0-9_-]{1,64}"
          value={key}
          onChange={(e) => setKey(e.target.value)}
        />
      </label>
      <label>
        Label
        <input
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          maxLength={500}
        />
      </label>
      {kind === "entity" && choose("Entity type", type, setType, "type")}
      {["fact", "relationship", "event"].includes(kind) && (
        <>
          {choose("Subject", subject, setSubject, "entity")}
          {choose("Source", source, setSource, "source")}
          <label>
            Valid from
            <input
              type="number"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
            />
          </label>
          <label>
            Valid until
            <input
              type="number"
              value={until}
              onChange={(e) => setUntil(e.target.value)}
            />
          </label>
        </>
      )}
      {kind === "relationship" && choose("Target", target, setTarget, "entity")}
      {["fact", "relationship", "type"].includes(kind) && (
        <label>
          Predicate
          <input
            value={predicate}
            onChange={(e) => setPredicate(e.target.value)}
          />
        </label>
      )}
      {["fact", "type"].includes(kind) && (
        <>
          <label>
            Value type
            <select
              value={valueType}
              onChange={(e) => setValueType(e.target.value)}
            >
              {["string", "number", "boolean", "entity"].map((v) => (
                <option key={v}>{v}</option>
              ))}
            </select>
          </label>
          <label>
            {kind === "type" ? "Relationship name (optional)" : "Value"}
            <input value={value} onChange={(e) => setValue(e.target.value)} />
          </label>
        </>
      )}
      {kind === "type" &&
        choose(
          "Relationship target type (optional)",
          target,
          setTarget,
          "type",
        )}
      <button>Stage record</button>
      <p>
        Using an existing ID proposes a complete replacement. Published records
        change only after review.
      </p>
    </form>
  );
}
