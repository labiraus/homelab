import { useEffect, useState } from "react";
import { RecordSummary } from "./WorldResponse.jsx";
import useLocalWriting from "./useLocalWriting.js";
import { worldHref } from "./worldWorkspace.js";
import { exactTick } from "./worldTicks.js";

export default function WorldEstablished({
  api,
  path,
  revision,
  timeline,
  canEdit,
  onStage,
  seed,
}) {
  const [origins, setOrigins] = useState({}),
    [accounts, setAccounts] = useState([]),
    [successors, setSuccessors] = useState({}),
    [draft, setDraft] = useState(null),
    [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    api(
      `${path}/established-accounts?timeline=${encodeURIComponent(timeline)}&revision=${revision}`,
    )
      .then((value) => {
        if (active) {
          setAccounts(value.records);
          setOrigins(value.origins || {});
          setSuccessors(value.successors);
          setError("");
        }
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [api, path, revision, timeline]);
  const start = (record, kind) =>
    setDraft({
      id: `account_${crypto.randomUUID().replaceAll("-", "")}`,
      kind: "established_account",
      label:
        kind === "supersedes"
          ? `Correction: ${record.label}`
          : kind === "clarifies"
            ? `Clarification: ${record.label}`
            : "",
      synopsis: "",
      reason: "",
      timeline,
      revision,
      ...(record ? { [kind]: record.id } : {}),
    });
  useEffect(() => {
    if (!seed) return;
    setDraft({
      id: `account_${crypto.randomUUID().replaceAll("-", "")}`,
      kind: "established_account",
      label: seed.record ? `Witnessed: ${seed.record.label}` : "",
      synopsis: "",
      reason: "",
      timeline: seed.timeline,
      revision: seed.revision,
      ...(seed.record
        ? {
            references: [seed.record.id],
            from: seed.record.from,
            until: seed.record.until,
          }
        : {}),
    });
  }, [seed]);
  const { localSaved, localError } = useLocalWriting(
    `${path}/establish/${timeline}/${revision}`,
    draft,
    setDraft,
  );
  return (
    <section aria-label="Established canon">
      <h2>Established canon</h2>
      <p>
        Keep witnessed events distinct from reported history and interpretation.
        Published synopses cannot be overwritten; corrections explicitly
        supersede them.
      </p>
      {canEdit && <button onClick={() => start()}>Establish an account</button>}
      {localSaved && draft && <p>Saved on this device</p>}
      {localError && <p role="alert">{localError}</p>}
      {error && <p role="alert">{error}</p>}
      {draft && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            try {
              if (draft.revision !== revision || draft.timeline !== timeline)
                throw Error(
                  "Scope changed. Return to the original revision before staging this account.",
                );
              const { revision: base, reason, ...record } = draft;
              if (record.from !== undefined && record.from !== "")
                record.from = exactTick(record.from);
              else delete record.from;
              if (record.until !== undefined && record.until !== "")
                record.until = exactTick(record.until);
              else delete record.until;
              onStage(record, base, reason);
              setDraft(null);
            } catch (e) {
              setError(e.message);
            }
          }}
        >
          <p>
            Draft at revision {draft.revision}. Only this account becomes
            immutable; linked records remain editable.
          </p>
          {draft.supersedes && (
            <section>
              <h3>Original synopsis</h3>
              <RecordSummary
                record={accounts.find((a) => a.id === draft.supersedes)}
              />
            </section>
          )}
          <label>
            Account title
            <input
              required
              maxLength={500}
              value={draft.label}
              onChange={(e) => setDraft({ ...draft, label: e.target.value })}
            />
          </label>
          <label>
            Witnessed synopsis
            <textarea
              required
              maxLength={16000}
              value={draft.synopsis}
              onChange={(e) => setDraft({ ...draft, synopsis: e.target.value })}
            />
          </label>
          <label>
            Event start tick (optional)
            <input
              value={draft.from || ""}
              onChange={(e) => setDraft({ ...draft, from: e.target.value })}
            />
          </label>
          <label>
            Event end tick (optional)
            <input
              value={draft.until || ""}
              onChange={(e) => setDraft({ ...draft, until: e.target.value })}
            />
          </label>
          <label>
            Published event/source references (comma separated, optional)
            <input
              value={(draft.references || []).join(", ")}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  references: e.target.value
                    .split(",")
                    .map((v) => v.trim())
                    .filter(Boolean),
                })
              }
            />
          </label>
          <label>
            Reason for establishing or superseding
            <textarea
              required
              maxLength={2000}
              value={draft.reason}
              onChange={(e) => setDraft({ ...draft, reason: e.target.value })}
            />
          </label>
          <button>Stage account for review</button>
          <button type="button" onClick={() => setDraft(null)}>
            Cancel
          </button>
        </form>
      )}
      {!accounts.length && !error && (
        <p>No established accounts in this timeline at this revision.</p>
      )}
      {accounts.map((record) => (
        <article className="wb-established" key={record.id}>
          <span className="wb-status" data-status="established">
            Established account ·{" "}
            {successors[record.id]
              ? "Superseded"
              : record.clarifies
                ? "Clarification"
                : "Current"}
          </span>
          <RecordSummary record={record} />
          {record.references?.map((id) => (
            <p key={id}>
              <a
                href={worldHref(path.slice(1), "explore", {
                  record: id,
                  revision: origins[record.id]?.revision ?? revision,
                  timeline,
                })}
              >
                Original reference · {id}
              </a>
            </p>
          ))}
          <p>
            Published by{" "}
            {origins[record.id]?.actor || "attribution unavailable offline"} ·
            original revision {origins[record.id]?.revision ?? "unavailable"}
          </p>
          <a
            href={`#${path.replace(/^\//, "/worlds/")}/explore?record=${record.id}&revision=${revision}&timeline=${encodeURIComponent(timeline)}`}
          >
            Cite this account · {path.slice(1)}:{revision}:{timeline}:
            {record.id}
          </a>
          {successors[record.id] && (
            <p>
              Superseded by{" "}
              {accounts.find((a) => a.id === successors[record.id])?.label ||
                successors[record.id]}
              . This original remains part of history.
            </p>
          )}
          {canEdit && (
            <div className="wb-device-actions">
              <button onClick={() => start(record, "clarifies")}>
                Add clarification
              </button>
              {!successors[record.id] && !record.clarifies && (
                <button onClick={() => start(record, "supersedes")}>
                  Supersede account
                </button>
              )}
            </div>
          )}
        </article>
      ))}
    </section>
  );
}
