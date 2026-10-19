import useLocalWriting from "./useLocalWriting.js";
import { humanLabel } from "./worldWorkspace.js";
import { useEffect, useRef, useState } from "react";
import WorldResponse, { RecordSummary } from "./WorldResponse.jsx";

export default function WorldStoryScene({
  api,
  path,
  worldId,
  session,
  turns,
  archived,
  canEdit,
  onSaveTurn,
  onInspect,
  onReview,
  onLibrary,
  onWorkspace,
  moreTurns,
}) {
  const [action, setAction] = useState("");
  const { localSaved, localError } = useLocalWriting(
    `${path}/sessions/${session.id}/action`,
    action,
    setAction,
  );
  const [candidate, setCandidate] = useState(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [cast, setCast] = useState([]);
  const controller = useRef(null);
  const input = useRef(null);
  const heading = useRef(null);
  useEffect(() => {
    heading.current?.focus({ preventScroll: true });
    return () => controller.current?.abort();
  }, []);
  useEffect(() => {
    let active = true;
    const scope = new URLSearchParams({
      revision: String(session.pinnedRevision),
      timeline: session.timeline,
    });
    Promise.all(
      [
        ...new Set(
          [
            ...(session.definition?.participants || []),
            session.scene?.location,
            ...(session.scene?.inventory || []).map((item) => item.entityId),
          ].filter(Boolean),
        ),
      ].map(async (id) => {
        try {
          if (session.state?.[id]) return { id, record: session.state[id] };
          return {
            id,
            record: (
              await api(`${path}/records/${encodeURIComponent(id)}?${scope}`)
            ).record,
          };
        } catch {
          return { id, unavailable: true };
        }
      }),
    ).then((rows) => {
      if (active) setCast(rows);
    });
    return () => {
      active = false;
    };
  }, [
    api,
    path,
    session.id,
    session.pinnedRevision,
    session.timeline,
    session.version,
  ]);
  async function continueScene(event) {
    event.preventDefault();
    if (!action.trim() || busy || archived) return;
    const direction = action.trim();
    if (new TextEncoder().encode(direction).length > 4000) {
      setError("Keep your action within 4,000 bytes.");
      return;
    }
    setBusy(true);
    setError("");
    const request = new AbortController();
    controller.current = request;
    try {
      const result = await api(
        `${path}/reason`,
        {
          workflow: "story_turn",
          question: direction,
          revision: session.pinnedRevision,
          timeline: session.timeline,
          sessionId: session.id,
          expectedVersion: session.version,
        },
        { signal: request.signal },
      );
      if (!request.signal.aborted)
        setCandidate({
          id: crypto.randomUUID(),
          intent: "play",
          scope: {
            worldId,
            revision: session.pinnedRevision,
            timeline: session.timeline,
          },
          result: {
            ...result,
            workflow: "story_turn",
            sessionId: session.id,
            direction,
            timeline: session.timeline,
          },
        });
    } catch (error) {
      if (!request.signal.aborted)
        setError(
          `${error.message}. Your action is retained; retry when ready to use another AI attempt.`,
        );
    } finally {
      setBusy(false);
      controller.current = null;
    }
  }
  const ordered = [...turns].sort((a, b) => b.version - a.version);
  const latest = ordered.find((turn) => turn.version === session.version);
  const choose = (direction) => {
    setAction(direction);
    input.current?.focus();
  };
  return (
    <section className="wb-story-scene" aria-label="Current story scene">
      <button onClick={onLibrary}>Back to story library</button>
      <h2 ref={heading} tabIndex={-1}>
        {session.title}
      </h2>
      <span className="wb-status">Private session state · provisional</span>
      <p>
        Pinned revision {session.pinnedRevision}, timeline {session.timeline},
        turn {session.version}. Outcomes require human review.
      </p>
      {session.mode === "sandbox" && (
        <p className="wb-status">
          Sandbox · fictional consistency may diverge; outcomes require
          reconciliation.
        </p>
      )}
      <section aria-label="Location and inventory">
        <h3>Location and inventory</h3>
        <p>
          Location:{" "}
          {session.scene?.location
            ? cast.find((item) => item.id === session.scene.location)?.record
                ?.label || session.scene.location
            : "Not set"}
        </p>
        {(session.scene?.inventory || []).length ? (
          <ul>
            {session.scene.inventory.map((item) => (
              <li key={item.entityId}>
                {cast.find((row) => row.id === item.entityId)?.record?.label ||
                  item.entityId}{" "}
                × {item.quantity}
              </li>
            ))}
          </ul>
        ) : (
          <p>No inventory recorded.</p>
        )}
      </section>
      <article aria-label="Saved scene">
        <h3>Turn {session.version}</h3>
        {latest ? (
          <p className="wb-prose">{latest.narration}</p>
        ) : session.version === 0 ? (
          <p className="wb-prose">
            {session.definition?.premise ||
              "Choose your first action to begin."}
          </p>
        ) : (
          <p role="status">
            The current scene is not loaded yet. Load more turns below.
          </p>
        )}
      </article>
      <div className="wb-chips">
        {session.memory?.choices?.map((choice) => (
          <button
            key={choice.id}
            disabled={archived || busy}
            onClick={() => choose(choice.direction)}
          >
            {choice.label}
          </button>
        ))}
      </div>
      {candidate && (
        <WorldResponse
          key={candidate.id}
          message={candidate}
          session={session}
          canEdit={canEdit}
          archived={archived}
          onSaveTurn={onSaveTurn}
          onInspect={onInspect}
          onChoice={choose}
        />
      )}
      {error && <p role="alert">{error}</p>}
      {localError && <p role="alert">{localError}</p>}
      {localSaved && (
        <p className="wb-sync-status">
          Saved on this device · AI actions are never sent automatically.
        </p>
      )}
      {busy && (
        <p role="status">
          Preparing the next scene…{" "}
          <button
            onClick={() => {
              controller.current?.abort();
              setError(
                "Stopped waiting. Provider work may already have used an allowance.",
              );
            }}
          >
            Stop waiting for scene
          </button>
        </p>
      )}
      <form className="wb-player-action" onSubmit={continueScene}>
        <label>
          Your action
          <textarea
            ref={input}
            value={action}
            onChange={(e) => setAction(e.target.value)}
            disabled={busy || archived}
            maxLength={4000}
            placeholder="What do you do next?"
          />
        </label>
        <button
          className="wb-primary"
          disabled={busy || archived || !action.trim()}
        >
          Continue scene
        </button>
      </form>
      <p className="wb-muted">
        AI prepares a provisional candidate. Save it explicitly to advance your
        private story.
      </p>
      <details>
        <summary>Characters and session details</summary>
        {cast.map((item) =>
          item.record ? (
            <button
              key={item.id}
              onClick={() =>
                onInspect({
                  worldId,
                  id: item.id,
                  revision: session.pinnedRevision,
                  timeline: session.timeline,
                })
              }
            >
              {item.record.label || "Character"} ·{" "}
              {humanLabel(item.record.type || "Entity")}
            </button>
          ) : (
            <p key={item.id}>
              A character could not be loaded at this story's revision.
            </p>
          ),
        )}
        {session.definition && <p>Premise: {session.definition.premise}</p>}
        {session.memory?.summary && (
          <p>
            Provisional summary through turn {session.memory.throughVersion}:{" "}
            {session.memory.summary}
          </p>
        )}
        {Object.values(session.state || {}).map((record) => (
          <article key={record.id}>
            <RecordSummary record={record} />
          </article>
        ))}
        <button onClick={onWorkspace}>Continue story in workspace</button>
      </details>
      <details>
        <summary>Earlier turns</summary>
        {ordered
          .filter((turn) => turn !== latest)
          .map((turn) => (
            <article key={turn.version}>
              <h4>Turn {turn.version}</h4>
              <p className="wb-prose">{turn.narration}</p>
            </article>
          ))}
        {moreTurns}
      </details>
      {canEdit && (
        <button
          disabled={!Object.keys(session.state || {}).length}
          onClick={onReview}
        >
          Review story outcomes
        </button>
      )}
    </section>
  );
}
