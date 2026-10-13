import WorldEntityPicker from "./WorldEntityPicker.jsx";
import { useEffect, useRef, useState } from "react";

export default function WorldStorySetup({
  api,
  path,
  revision,
  timeline,
  archived,
  onStart,
  premiseCandidate = "",
}) {
  const [definitions, setDefinitions] = useState([]),
    [selected, setSelected] = useState("");
  const [title, setTitle] = useState(""),
    [premise, setPremise] = useState(""),
    [objective, setObjective] = useState(""),
    [style, setStyle] = useState(""),
    [cast, setCast] = useState([]);
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [more, setMore] = useState(false);
  const retry = useRef(null);
  useEffect(() => {
    let current = true;
    api(`${path}/story-definitions`)
      .then((rows) => {
        if (current) {
          setDefinitions(rows);
          setMore(rows.length === 100);
        }
      })
      .catch((e) => {
        if (current) setError(e.message);
      });
    return () => {
      current = false;
    };
  }, [api, path]);
  useEffect(() => {
    if (premiseCandidate?.text) {
      setSelected("");
      setPremise(premiseCandidate.text);
    }
  }, [premiseCandidate]);
  const definition = definitions.find((d) => d.id === selected);
  async function start(e) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const input = {
        title,
        premise,
        objective,
        style,
        participants: cast.map((record) => record.id),
        pinnedRevision: premiseCandidate?.revision ?? revision,
        timeline: premiseCandidate?.timeline ?? timeline,
      };
      const signature = JSON.stringify(
        definition
          ? {
              definitionId: definition.id,
              title: definition.title,
              pinnedRevision: definition.pinnedRevision,
              timeline: definition.timeline,
            }
          : input,
      );
      if (retry.current?.signature !== signature)
        retry.current = {
          signature,
          definitionId: crypto.randomUUID(),
          sessionId: crypto.randomUUID(),
        };
      const d =
        definition ??
        (await api(`${path}/story-definitions`, {
          ...input,
          id: retry.current.definitionId,
        }));
      const session = await api(`${path}/sessions`, {
        id: retry.current.sessionId,
        title: d.title,
        definitionId: d.id,
        pinnedRevision: d.pinnedRevision,
        timeline: d.timeline,
      });
      setDefinitions((old) =>
        old.some((x) => x.id === d.id) ? old : [d, ...old],
      );
      setSelected(d.id);
      retry.current = null;
      await onStart(session);
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <fieldset disabled={busy}>
      <legend>Private story setup</legend>
      {error && <p role="alert">{error}</p>}
      <label>
        Reusable definition
        <select value={selected} onChange={(e) => setSelected(e.target.value)}>
          <option value="">New definition</option>
          {definitions.map((d) => (
            <option key={d.id} value={d.id}>
              {d.title} · revision {d.pinnedRevision}
            </option>
          ))}
        </select>
      </label>
      {more && (
        <button
          type="button"
          onClick={async () => {
            try {
              const rows = await api(
                `${path}/story-definitions?offset=${definitions.length}`,
              );
              setDefinitions((old) => [
                ...old,
                ...rows.filter((r) => !old.some((d) => d.id === r.id)),
              ]);
              setMore(rows.length === 100);
            } catch (e) {
              setError(e.message);
            }
          }}
        >
          Load more definitions
        </button>
      )}
      <form onSubmit={start}>
        {definition ? (
          <p>
            {definition.premise} — pinned revision {definition.pinnedRevision},
            timeline {definition.timeline}. To revise this immutable setup,
            create a new definition.
          </p>
        ) : (
          <>
            <label>
              Story title
              <input
                required
                maxLength={200}
                value={title}
                onChange={(e) => setTitle(e.target.value)}
              />
            </label>
            <label>
              Premise
              <textarea
                maxLength={16000}
                value={premise}
                onChange={(e) => setPremise(e.target.value)}
              />
            </label>
            <label>
              Story objective
              <textarea
                maxLength={2000}
                value={objective}
                onChange={(e) => setObjective(e.target.value)}
              />
            </label>
            <label>
              Narrative style
              <textarea
                maxLength={2000}
                value={style}
                onChange={(e) => setStyle(e.target.value)}
              />
            </label>
            <WorldEntityPicker
              api={api}
              path={path}
              revision={premiseCandidate?.revision ?? revision}
              timeline={premiseCandidate?.timeline ?? timeline}
              value={cast}
              onChange={setCast}
            />
          </>
        )}
        <button disabled={archived}>
          Start at revision{" "}
          {definition?.pinnedRevision ?? premiseCandidate?.revision ?? revision}
        </button>
      </form>
      <p>
        Definitions and summaries are private creative guidance. They do not
        establish canon.
      </p>
    </fieldset>
  );
}
