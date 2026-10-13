import { useState } from "react";
import { RecordSummary } from "./WorldResponse.jsx";

export default function WorldConfiguration({
  record,
  revision,
  timelines,
  canEdit,
  onStage,
  onAdvanced,
  onExplore,
}) {
  const [draft, setDraft] = useState(null);
  const [error, setError] = useState("");
  return (
    <section aria-label="Published world configuration">
      <h2>World guide and style</h2>
      <p>
        A dedicated world guide and style guide are not yet supported by the
        canon API. Stories can have their own private premise and narrative
        style.
      </p>
      <h3>World configuration</h3>
      <p className="wb-muted">
        Name, description, default timeline, calendar and approval policy belong
        to published canon. Changes go through a draft and human review.
      </p>
      {record ? (
        <>
          <span className="wb-status">
            Published canon · revision {revision}
          </span>
          <RecordSummary record={record} />
          {canEdit && (
            <button
              onClick={() => {
                setDraft({
                  record,
                  revision,
                  name: record.label,
                  description: record.attributes?.description || "",
                  timeline: record.timeline,
                });
                setError("");
              }}
            >
              Edit name and description
            </button>
          )}
          {draft && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                try {
                  onStage(
                    {
                      ...draft.record,
                      label: draft.name,
                      timeline: draft.timeline,
                      attributes: {
                        ...draft.record.attributes,
                        description: draft.description,
                      },
                    },
                    draft.revision,
                  );
                  setDraft(null);
                } catch (error) {
                  setError(error.message);
                }
              }}
            >
              <p>Draft · based on revision {draft.revision}</p>
              <label>
                World name
                <input
                  required
                  maxLength={200}
                  value={draft.name}
                  onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                />
              </label>
              <label>
                World description
                <textarea
                  maxLength={16000}
                  value={draft.description}
                  onChange={(e) =>
                    setDraft({ ...draft, description: e.target.value })
                  }
                />
              </label>
              <label>
                Default timeline
                <select
                  value={draft.timeline}
                  onChange={(e) =>
                    setDraft({ ...draft, timeline: e.target.value })
                  }
                >
                  {timelines.map((t) => (
                    <option value={t.id} key={t.id}>
                      {t.label || t.id}
                    </option>
                  ))}
                </select>
              </label>
              <button>Stage configuration draft</button>
              <button type="button" onClick={() => setDraft(null)}>
                Cancel configuration edit
              </button>
              {error && <p role="alert">{error}</p>}
            </form>
          )}
          <h3>Members and approval policy</h3>
          <p>
            {record.reviewPolicy?.requiredApprovals ?? 1} human approval(s)
            required. Members and roles are managed by the world owner.
          </p>
          {canEdit && (
            <button onClick={() => onAdvanced(record, revision)}>
              Edit calendar and approval policy
            </button>
          )}
        </>
      ) : (
        <p>
          No published configuration in this snapshot.{" "}
          <button onClick={onExplore}>Browse world records</button>
        </p>
      )}
    </section>
  );
}
