import { useRef, useState } from "react";
import { Findings } from "./WorldFindings.jsx";
import { citationScope, humanLabel } from "./worldWorkspace.js";

export function RecordSummary({ record }) {
  if (!record) return <p>Absent</p>;
  return (
    <>
      <h4>{record.label || record.id}</h4>
      <p>
        {humanLabel(record.kind)}
        {record.type ? ` · ${record.type}` : ""}
      </p>
      {record.synopsis && <p className="wb-prose">{record.synopsis}</p>}
      {record.worldGuide && (
        <section>
          <h5>World guide</h5>
          <p className="wb-prose">{record.worldGuide}</p>
        </section>
      )}
      {record.styleGuide && (
        <section>
          <h5>Style guide</h5>
          <p className="wb-prose">{record.styleGuide}</p>
        </section>
      )}
      {record.attributes?.description && (
        <p className="wb-prose">{record.attributes.description}</p>
      )}
      <dl className="wb-facts">
        {Object.entries(record.attributes || {})
          .filter(([key]) => key !== "description")
          .map(([key, value]) => (
            <div key={key}>
              <dt>{humanLabel(key)}</dt>
              <dd>{value}</dd>
            </div>
          ))}
      </dl>
      <dl className="wb-facts">
        {Object.entries(record)
          .filter(
            ([key]) =>
              ![
                "id",
                "kind",
                "label",
                "attributes",
                "fields",
                "relations",
                "calendar",
                "synopsis",
                "worldGuide",
                "styleGuide",
              ].includes(key),
          )
          .map(([key, value]) => (
            <div key={key}>
              <dt>{humanLabel(key)}</dt>
              <dd>
                {typeof value === "object"
                  ? JSON.stringify(value)
                  : String(value)}
              </dd>
            </div>
          ))}
      </dl>
      <details>
        <summary>Complete record</summary>
        <pre>{JSON.stringify(record, null, 2)}</pre>
      </details>
    </>
  );
}
export function OperationCards({ operations = [] }) {
  return (
    <div className="wb-operations">
      {operations.map((op, i) => (
        <article key={`${op.id || op.record?.id}-${i}`}>
          <span className="wb-status">
            Draft · {op.action === "delete" ? "Remove" : "Update"}{" "}
            {op.record?.label || op.id || op.record?.id}
          </span>
          {op.record && <RecordSummary record={op.record} />}
        </article>
      ))}
    </div>
  );
}

export default function WorldResponse({
  message,
  canEdit,
  archived,
  session,
  onInspect,
  onEdit,
  onPropose,
  onSaveTurn,
  onPremise,
  onChoice,
}) {
  const result = message.result;
  const [text, setText] = useState(result.answer || "");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [saved, setSaved] = useState("");
  const retry = useRef(null);
  async function act(kind, action) {
    setBusy(true);
    setError("");
    const signature = JSON.stringify({ kind, text, reason });
    if (retry.current?.signature !== signature)
      retry.current = { signature, id: crypto.randomUUID() };
    try {
      await action(retry.current.id);
      setSaved(kind);
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  const proposal = result.workflow === "canon_proposal";
  const story = result.workflow === "story_turn";
  const narrative = result.workflow === "narrative_generate";
  const staleSession =
    story &&
    (session?.id !== result.sessionId ||
      session?.version !== result.sessionVersion);
  return (
    <article className="wb-response" aria-label={`${message.intent} response`}>
      <header>
        <span className="wb-status">
          {proposal || narrative
            ? "Draft · noncanonical"
            : story
              ? "Provisional story candidate"
              : "AI answer · grounded in published canon"}
        </span>
        <span>
          Revision {result.baseRevision} · {message.scope.timeline}
        </span>
      </header>
      <p className="wb-prose">{result.answer}</p>
      {!!result.establishedAccounts?.length && (
        <details>
          <summary>Effective established accounts in this scope</summary>
          <p>
            These published synopses take precedence over conflicting reports.
            Interpretation remains open.
          </p>
          {result.establishedAccounts.map((e) => (
            <article key={e.id}>
              <RecordSummary record={e.record} />
              <button
                onClick={() =>
                  onInspect({
                    worldId: e.worldId,
                    revision: e.revision,
                    timeline: e.timeline,
                    id: e.record.id,
                  })
                }
              >
                Inspect established citation · {e.record.label}
              </button>
            </article>
          ))}
        </details>
      )}
      {result.unknown && (
        <p role="note">Uncertainty: this is not established in canon.</p>
      )}
      {result.evidenceScope?.coverage === "selected_records" && (
        <p className="wb-muted">
          Based on {result.evidenceScope.evidenceCount} selected canon records
          out of {result.evidenceScope.canonRecordCount}. Other canon may affect
          this answer; this is not a whole-world consistency check.
        </p>
      )}
      {!result.citations?.length && (
        <p className="wb-muted">No published sources returned.</p>
      )}
      <div className="wb-chips" aria-label="Published sources">
        {result.citations?.map((c) => {
          const scope = citationScope(c, message.scope.worldId);
          return scope ? (
            <button key={c} onClick={() => onInspect(scope)}>
              Open source · {scope.id}
            </button>
          ) : (
            <span key={c}>Source unavailable</span>
          );
        })}
      </div>
      {result.workflow === "consistency_check" && <Findings result={result} />}
      <OperationCards operations={result.operations} />
      {error && <p role="alert">{error}</p>}
      {saved && (
        <p role="status">
          {saved === "proposal"
            ? "Draft saved. Open Drafts & Proposals to submit for human review."
            : "Turn saved as private session state. Canon is unchanged."}
        </p>
      )}
      {(proposal || story || narrative) && (
        <fieldset
          disabled={busy || archived || !!saved}
          className="wb-response-actions"
        >
          <legend>Next steps</legend>
          {proposal && canEdit && (
            <>
              <label>
                Proposal reason
                <input
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  maxLength={2000}
                />
              </label>
              <button
                disabled={!result.operations?.length || !reason.trim()}
                onClick={() =>
                  act("proposal", (id) => onPropose(result, reason, id))
                }
              >
                Save AI candidate as draft
              </button>
              <button
                disabled={!result.operations?.length}
                onClick={() => onEdit(result)}
              >
                Edit draft in structured editor
              </button>
            </>
          )}
          {proposal && !canEdit && (
            <p>Your world role cannot save canon proposals.</p>
          )}
          {(story || narrative) && (
            <label>
              Edit draft narration
              <textarea
                value={text}
                onChange={(e) => setText(e.target.value)}
                maxLength={16000}
              />
            </label>
          )}
          {narrative && (
            <button
              disabled={!text.trim()}
              onClick={() =>
                onPremise({
                  text,
                  revision: result.baseRevision,
                  timeline: message.scope.timeline,
                })
              }
            >
              Use as a private story premise
            </button>
          )}
          {story && (
            <>
              <button
                disabled={staleSession || !text.trim()}
                onClick={() =>
                  act("turn", (id) =>
                    onSaveTurn({ ...result, answer: text }, id),
                  )
                }
              >
                Save candidate as provisional turn
              </button>
              {staleSession && !saved && (
                <p>
                  This candidate belongs to an earlier or different story turn.
                  Generate again from the resumed session.
                </p>
              )}
            </>
          )}
        </fieldset>
      )}
      {story &&
        saved &&
        result.choices?.map((choice) => (
          <button key={choice.id} onClick={() => onChoice(choice.direction)}>
            {choice.label}
          </button>
        ))}
      {result.memorySummary && (
        <details>
          <summary>Proposed session memory</summary>
          <p>{result.memorySummary}</p>
        </details>
      )}
      <details>
        <summary>Generation details</summary>
        <p>Model: {result.model || "not reported"}</p>
        {result.provenance && (
          <>
            <p>Request: {result.provenance.requestId}</p>
            <p>Duration: {result.provenance.latencyMs} ms</p>
            {result.provenance.accountingStatus ? (
              <p>
                {result.provenance.accountingStatus === "local"
                  ? "Local inference · no usage charge"
                  : result.provenance.accountingStatus === "not_requested"
                    ? "No model inference was needed"
                    : result.provenance.accountingStatus === "estimated"
                    ? `Estimated cloud cost: $${((result.provenance.estimatedMicros || 0) / 1000000).toFixed(6)} USD`
                    : "Cloud accounting pending or uncertain; reservation retained"}
                {result.provenance.profile && ` · ${result.provenance.profile}`}
              </p>
            ) : (
              <p>Budget allocated: {(result.provenance.allocatedMicros / 1000000).toFixed(6)} {result.provenance.currency}. This is an allowance, not an invoice.</p>
            )}
          </>
        )}
      </details>
    </article>
  );
}
