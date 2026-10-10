import { useState } from "react";

export function Findings({ result }) {
  if (!result) return null;
  return (
    <section aria-label="Consistency findings">
      {result.deterministic && (
        <p>
          Deterministic rules passed at revision {result.deterministic.revision}
          {result.deterministic.changeId ? " for this proposal" : ""}.
          Publication checks them again.
        </p>
      )}
      <p>
        AI findings are advisory and cannot approve or reject a proposal. An
        empty list does not prove consistency.
      </p>
      {result.evidenceScope?.coverage === "selected_records" && (
        <p>
          Semantic review covers selected evidence only; other canon may affect
          these findings.
        </p>
      )}
      {(result.findings ?? []).map((finding, index) => (
        <article key={index}>
          <h4>
            {finding.severity} · uncertainty: {finding.uncertainty}
          </h4>
          <p>{finding.message}</p>
          <ul>
            {finding.citations.map((id) => {
              const [world, revision, timeline, record] = id.split(":");
              const url = `${import.meta.env.VITE_API_BASE_URL ?? ""}/api/worlds/${encodeURIComponent(world)}/records/${encodeURIComponent(record)}?revision=${encodeURIComponent(revision)}&timeline=${encodeURIComponent(timeline)}`;
              return (
                <li key={id}>
                  <a href={url} target="_blank" rel="noreferrer">
                    {id}
                  </a>
                </li>
              );
            })}
          </ul>
        </article>
      ))}
    </section>
  );
}
export default function ProposalFindings({
  api,
  path,
  change,
  timeline,
  archived,
}) {
  const [result, setResult] = useState(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  return (
    <section>
      <button
        type="button"
        disabled={busy || archived}
        onClick={async () => {
          setBusy(true);
          setError("");
          setResult(null);
          try {
            setResult(
              await api(`${path}/reason`, {
                workflow: "consistency_check",
                changeId: change.id,
                revision: change.baseRevision,
                timeline,
                question:
                  "Review this proposed change for semantic contradictions with the supplied published evidence. Explain uncertainty.",
              }),
            );
          } catch (e) {
            setError(e.message);
          } finally {
            setBusy(false);
          }
        }}
      >
        Check proposal consistency
      </button>
      {error && <p role="alert">{error}</p>}
      {result && (
        <>
          <p>{result.answer}</p>
          <Findings result={result} />
        </>
      )}
    </section>
  );
}
