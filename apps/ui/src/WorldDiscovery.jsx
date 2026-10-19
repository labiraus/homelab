import { useState } from "react";
import { worldHref } from "./worldWorkspace.js";
export default function WorldDiscovery({ api }) {
  const [query, setQuery] = useState(""),
    [results, setResults] = useState(null),
    [activity, setActivity] = useState(null),
    [error, setError] = useState("");
  return (
    <details>
      <summary>Search all worlds and activity</summary>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setError("");
          try {
            setResults(await api(`/search?q=${encodeURIComponent(query)}`));
          } catch (e) {
            setError(e.message);
          }
        }}
      >
        <label>
          Search across your worlds
          <input
            required
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
        <button>Search all worlds</button>
      </form>
      {results && (
        <>
          <p>
            Search covers {results.searchedWorlds} accessible worlds and their
            default timelines.
            {results.limited ? " World coverage is limited." : ""}
          </p>
          {results.worlds.map((group) => (
            <section key={group.world.id}>
              <h3>{group.world.name}</h3>
              {group.citations.map((c) => (
                <p key={c.record.id}>
                  <a
                    href={worldHref(group.world.id, "explore", {
                      record: c.record.id,
                      revision: group.world.revision,
                      timeline: group.world.defaultTimeline,
                    })}
                  >
                    {c.record.label || c.record.id}
                  </a>
                </p>
              ))}
            </section>
          ))}
        </>
      )}
      <button
        onClick={async () => {
          try {
            setActivity(await api("/activity"));
            setError("");
          } catch (e) {
            setError(e.message);
          }
        }}
      >
        Recent world activity
      </button>
      {activity && (
        <ul>
          {activity.map((a) => (
            <li key={a.id}>
              <a
                href={worldHref(
                  a.worldId,
                  a.action === "world.create" ? "workspace" : "drafts",
                  a.action === "world.create" ? {} : { proposal: a.resource },
                )}
              >
                {a.worldName} · {a.action.replaceAll(".", " ")} · {a.actor}
              </a>
            </li>
          ))}
        </ul>
      )}
      {error && <p role="alert">{error}</p>}
    </details>
  );
}
