import { useState } from "react";
import WorldRecordEditor from "./WorldRecordEditor.jsx";
import { RecordSummary } from "./WorldResponse.jsx";

// Edits are working material only. The existing change-set API validates saves.
export default function WorldDraftArtifact({
  operations,
  original,
  onChange,
  api,
  path,
  scope,
  disabled,
  onEditing,
}) {
  const [editing, setEditing] = useState(null);
  const [catalog, setCatalog] = useState([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  function update(index, record) {
    onChange(operations.map((op, i) => (i === index ? { ...op, record } : op)));
  }
  async function open(index) {
    setError("");
    setLoading(true);
    try {
      const records = [];
      let page;
      do {
        page = await api(
          `${path}/canon?${new URLSearchParams({ revision: String(scope.revision), timeline: scope.timeline, limit: "500", offset: String(records.length) })}`,
        );
        records.push(...page.records);
      } while (page.records.length && records.length < page.total);
      const merged = new Map(records.map((r) => [r.id, r]));
      for (const op of operations) {
        if (op.action === "delete") merged.delete(op.id);
        else merged.set(op.record.id, op.record);
      }
      setCatalog([...merged.values()]);
      setEditing(index);
      onEditing(true);
    } catch (e) {
      setError(
        `Could not load the draft’s published context: ${e.message}. Your edits are retained; retry when ready.`,
      );
    } finally {
      setLoading(false);
    }
  }
  function close() {
    setEditing(null);
    onEditing(false);
  }
  return (
    <section className="wb-draft-artifact" aria-label="Working draft">
      <p className="wb-muted">
        Unsaved working draft · edits stay in this open world. Save explicitly
        before refreshing or switching worlds.
      </p>
      {error && <p role="alert">{error}</p>}
      {loading && <p role="status">Loading the original published context…</p>}
      <fieldset disabled={disabled || loading}>
        <legend>Shape this draft</legend>
        {operations.map((op, index) => (
          <article key={`${op.id || op.record?.id}-${index}`}>
            {editing === index ? (
              <>
                <WorldRecordEditor
                  record={op.record}
                  records={catalog}
                  revision={scope.revision}
                  timeline={scope.timeline}
                  onStage={(record) => {
                    update(index, record);
                    close();
                  }}
                  onNew={close}
                  submitLabel="Apply to working draft"
                  cancelLabel="Cancel detailed editing"
                />
                <p>
                  Apply these fields before saving or refining the candidate.
                </p>
              </>
            ) : (
              <>
                <span className="wb-status">
                  Draft ·{" "}
                  {op.action === "delete"
                    ? "Proposed removal"
                    : "Proposed record"}
                </span>
                {op.action === "put" && op.record.kind === "entity" ? (
                  <div className="wb-draft-fields">
                    <label>
                      Name
                      <input
                        value={op.record.label || ""}
                        maxLength={500}
                        onChange={(e) =>
                          update(index, { ...op.record, label: e.target.value })
                        }
                      />
                    </label>
                    <label>
                      Description
                      <textarea
                        value={op.record.attributes?.description || ""}
                        onChange={(e) =>
                          update(index, {
                            ...op.record,
                            attributes: {
                              ...op.record.attributes,
                              description: e.target.value,
                            },
                          })
                        }
                      />
                    </label>
                    {Object.entries(op.record.attributes || {})
                      .filter(([key]) => key !== "description")
                      .map(([key, value]) => (
                        <label key={key}>
                          {key}
                          <input
                            value={value}
                            onChange={(e) =>
                              update(index, {
                                ...op.record,
                                attributes: {
                                  ...op.record.attributes,
                                  [key]: e.target.value,
                                },
                              })
                            }
                          />
                        </label>
                      ))}
                  </div>
                ) : op.record ? (
                  <RecordSummary record={op.record} />
                ) : (
                  <p>{op.id}</p>
                )}
                {op.record &&
                  op.record.kind !== "established_account" &&
                  api && (
                    <button
                      disabled={editing !== null}
                      onClick={() => open(index)}
                    >
                      Edit details · {op.record.label || op.record.id}
                    </button>
                  )}
                {op.record?.kind === "established_account" && (
                  <p>
                    Published established accounts cannot be overwritten. Use
                    the existing account history and reviewed clarification
                    flow.
                  </p>
                )}
              </>
            )}
          </article>
        ))}
      </fieldset>
      <details>
        <summary>Compare generated and working versions</summary>
        {operations.map((op, i) => (
          <div className="world-diff" key={i}>
            <section>
              <h4>Generated draft</h4>
              <RecordSummary record={original[i]?.record} />
            </section>
            <section>
              <h4>Your working draft</h4>
              <RecordSummary record={op.record} />
            </section>
          </div>
        ))}
      </details>
    </section>
  );
}
