import WorldConfiguration from "./WorldConfiguration.jsx";
import WorldStoryScene from "./WorldStoryScene.jsx";
import useWorldDialog from "./useWorldDialog.js";
import { createPortal } from "react-dom";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { parseWorldJSON } from "./worldTicks.js";
import { createCalendarFormatter } from "./worldCalendar.js";
import { HistoryPanel, ReconciliationPanel } from "./WorldHistory.jsx";
import WorldRecordEditor from "./WorldRecordEditor.jsx";
import WorldAdministration from "./WorldAdministration.jsx";
import WorldStorySetup from "./WorldStorySetup.jsx";
import WorldWorkspace from "./WorldWorkspace.jsx";
import WorldInspector from "./WorldInspector.jsx";
import { RecordSummary } from "./WorldResponse.jsx";
import {
  readWorldRoute,
  worldHref,
  sections,
  humanLabel,
} from "./worldWorkspace.js";
import WorldConnections from "./WorldConnections.jsx";
import WorldViews from "./WorldViews.jsx";
import WorldUsers from "./WorldUsers.jsx";
import WorldTimelineBranch from "./WorldTimelineBranch.jsx";
import ProposalFindings from "./WorldFindings.jsx";

const base = import.meta.env.VITE_API_BASE_URL ?? "";
async function api(path, body, options = {}) {
  const response = await fetch(`${base}/api/worlds${path}`, {
    signal: options.signal,
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
  if (!response.ok) {
    const error = new Error(
      data.error ||
        (response.status === 404
          ? "Worldbuilder is not enabled, or you do not have access to this world."
          : "Worldbuilder request failed"),
    );
    error.status = response.status;
    throw error;
  }
  return data;
}
const id = () => crypto.randomUUID();
const mergeRows = (before, after, key = "id") => [
  ...new Map([...before, ...after].map((row) => [row[key], row])).values(),
];
export default function Worldbuilder({ email }) {
  const [headerHost, setHeaderHost] = useState(null);
  const worldMenu = useRef(null);
  useLayoutEffect(() => {
    setHeaderHost(document.getElementById("world-header-tools"));
  }, []);
  const [worlds, setWorlds] = useState([]),
    [selected, setSelected] = useState(() => readWorldRoute().worldId || ""),
    [name, setName] = useState(""),
    [error, setError] = useState("");
  const [moreWorlds, setMoreWorlds] = useState(false);
  const [worldOffset, setWorldOffset] = useState(0);
  const [worldsBusy, setWorldsBusy] = useState(false);
  const [available, setAvailable] = useState(false);
  const [listRetry, setListRetry] = useState(0);
  const selectedWorld = worlds.find((w) => w.id === selected);
  const [worldLookupRetry, setWorldLookupRetry] = useState(0);
  useEffect(() => {
    if (!available || !selected || selectedWorld) return;
    let active = true;
    setError("");
    api(`/${encodeURIComponent(selected)}`)
      .then((world) => {
        if (active) setWorlds((rows) => mergeRows(rows, [world]));
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, [available, selected, !!selectedWorld, worldLookupRetry]);
  useEffect(() => {
    let active = true;
    api("")
      .then((rows) => {
        if (active) {
          setAvailable(true);
          setError("");
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
  }, [listRetry]);
  useEffect(() => {
    const sync = () => setSelected(readWorldRoute().worldId || "");
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, []);
  function selectWorld(value) {
    if (worldMenu.current) worldMenu.current.open = false;
    setSelected(value);
    window.location.hash = value ? worldHref(value) : "#/worlds";
  }
  async function create(e) {
    e.preventDefault();
    try {
      const w = await api("", { id: id(), name });
      const first = await api("");
      setWorlds(mergeRows(first, [w]));
      setWorldOffset(first.length);
      setMoreWorlds(first.length === 100);
      selectWorld(w.id);
      setName("");
      setError("");
    } catch (e) {
      setError(e.message);
    }
  }
  const worldTools = (
    <div className="wb-world-tools worldbuilder">
      {error && (
        <p role="alert">
          {error}
          {!available && (
            <button onClick={() => setListRetry((v) => v + 1)}>
              Retry worlds
            </button>
          )}
          {available && selected && !selectedWorld && (
            <button onClick={() => setWorldLookupRetry((v) => v + 1)}>
              Retry selected world
            </button>
          )}
        </p>
      )}
      <details ref={worldMenu} className="wb-world-menu">
        <summary>World menu</summary>
        {available && (
          <details>
            <summary>Account administration</summary>
            <WorldUsers api={api} />
          </details>
        )}
        <h3>Create a world</h3>
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
          <button disabled={!available}>Create world</button>
        </form>
      </details>

      <label>
        <span className="wb-sr-only">Open world</span>
        <select
          title={selectedWorld?.name || "Choose a world"}
          value={selected}
          onChange={(e) => selectWorld(e.target.value)}
        >
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
    </div>
  );
  return (
    <section className="workspace-shell worldbuilder">
      <header hidden={!!selected}>
        <p className="workspace-eyebrow">Worldbuilder</p>
        <h1>Worldbuilder</h1>
        <p>A place to imagine, discover and shape your worlds.</p>
      </header>
      {headerHost ? createPortal(worldTools, headerHost) : worldTools}
      {selected && available && !selectedWorld && !error && (
        <p role="status">Loading selected world…</p>
      )}
      {selected && selectedWorld && (
        <World
          key={`${email}:${selected}`}
          worldId={selected}
          email={email}
          initialTimeline={selectedWorld.defaultTimeline || "main"}
          onWorld={(w) => setWorlds((rows) => mergeRows(rows, [w]))}
        />
      )}
    </section>
  );
}
function World({ worldId, email, initialTimeline = "main", onWorld }) {
  const path = `/${encodeURIComponent(worldId)}`;
  const [section, setSection] = useState(
    () => readWorldRoute().section || "workspace",
  );
  const [navOpen, setNavOpen] = useState(false);
  const navigation = useRef(null);
  const closeNavigation = useCallback(() => setNavOpen(false), []);
  useWorldDialog(navigation, navOpen, closeNavigation);
  const [compact, setCompact] = useState(
    () => window.matchMedia?.("(max-width: 1100px)").matches ?? false,
  );
  useEffect(() => {
    const media = window.matchMedia?.("(max-width: 1100px)");
    if (!media) return;
    const update = () => {
      setCompact(media.matches);
      setNavOpen(false);
    };
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  const [railCollapsed, setRailCollapsed] = useState(false);
  const [historyHost, setHistoryHost] = useState(null);
  const [inspector, setInspector] = useState(null);
  const [references, setReferences] = useState([]);
  const [prompt, setPrompt] = useState(null);
  const [proposalFilter, setProposalFilter] = useState("all");
  const heading = useRef(null);
  const inspectorReturn = useRef(null);
  const layout = useRef(null);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const proposalRetry = useRef(null),
    turnRetry = useRef(null);
  function navigate(next, params = {}) {
    if (!mounted.current) return;
    setSection(next);
    setNavOpen(false);
    setInspector(null);
    setConnectionScope(null);
    setLinkedRecord(null);
    window.scrollTo?.(0, 0);
    window.location.hash = worldHref(worldId, next, params);
  }
  useEffect(() => {
    const sync = () => {
      const route = readWorldRoute();
      if (route.worldId !== worldId) return;
      setSection(route.section);
      setNavOpen(false);
      setInspector(null);
      setConnectionScope(null);
      setLinkedRecord(null);
      const p = route.params;
      if (
        p.get("record") &&
        /^\d+$/.test(p.get("revision") || "") &&
        p.get("timeline")
      )
        setInspector({
          worldId,
          id: p.get("record"),
          revision: Number(p.get("revision")),
          timeline: p.get("timeline"),
          at: p.get("at") || "",
        });
      if (p.get("proposal"))
        setReconcileTarget({ kind: "changes", id: p.get("proposal") });
      if (p.get("session"))
        void run(async () =>
          resume(
            await api(
              `${path}/sessions/${encodeURIComponent(p.get("session"))}`,
            ),
          ),
        );
    };
    sync();
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, [worldId]);
  useEffect(() => {
    if (section === "workspace" && prompt)
      layout.current?.querySelector(".wb-composer textarea")?.focus();
    else heading.current?.focus({ preventScroll: true });
  }, [section]);
  const [canon, setCanon] = useState(null),
    [changes, setChanges] = useState([]),
    [sessions, setSessions] = useState([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const [timeline, setTimeline] = useState(initialTimeline),
    [revision, setRevision] = useState(""),
    [at, setAt] = useState(""),
    [query, setQuery] = useState(""),
    [recordKind, setRecordKind] = useState(""),
    [searchMode, setSearchMode] = useState("lexical");
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
  const [setupOpen, setSetupOpen] = useState(false);
  const [sceneOpen, setSceneOpen] = useState(false);
  const [session, setSession] = useState(null),
    [turns, setTurns] = useState([]),
    [premiseCandidate, setPremiseCandidate] = useState(""),
    [narration, setNarration] = useState("");

  useEffect(() => {
    const node = layout.current;
    if (!node) return;
    const viewport = window.visualViewport;
    let resizeFrame;
    const update = () => {
      const bottom = viewport
        ? viewport.height + viewport.offsetTop
        : window.innerHeight;
      node.style.setProperty(
        "--wb-available-height",
        `${Math.max(200, bottom - node.getBoundingClientRect().top - 12)}px`,
      );
    };
    const resize = () => {
      update();
      cancelAnimationFrame(resizeFrame);
      resizeFrame = requestAnimationFrame(() => {
        const composer = document.activeElement?.closest(".wb-composer");
        if (!composer || !node.contains(composer)) return;
        const bottom = viewport
          ? viewport.height + viewport.offsetTop
          : window.innerHeight;
        const overflow = composer.getBoundingClientRect().bottom - bottom;
        if (overflow > 0) {
          window.scrollBy({ top: overflow + 12, behavior: "instant" });
          update();
        }
      });
    };
    update();
    const observer =
      typeof ResizeObserver === "undefined" ? null : new ResizeObserver(update);
    observer?.observe(document.querySelector(".topbar") || node);
    window.addEventListener("resize", resize);
    window.addEventListener("scroll", update, { passive: true });
    viewport?.addEventListener("resize", resize);
    viewport?.addEventListener("scroll", update);
    return () => {
      observer?.disconnect();
      cancelAnimationFrame(resizeFrame);
      window.removeEventListener("resize", resize);
      window.removeEventListener("scroll", update);
      viewport?.removeEventListener("resize", resize);
      viewport?.removeEventListener("scroll", update);
    };
  }, [!!canon, section]);

  function loadCanonPage(params, token) {
    if (params.get("kind")) return api(`${path}/canon?${params}`);
    if (params.get("mode") === "hybrid" && params.get("q")) {
      return api(`${path}/search`, {
        revision: Number(params.get("revision") || canon?.world.revision || 0),
        timeline: params.get("timeline"),
        query: params.get("q"),
        ...(params.has("at") ? { at: params.get("at") } : {}),
        limit: Number(params.get("limit") || 500),
        offset: Number(params.get("offset") || 0),
        ...(token ? { searchToken: token } : {}),
      });
    }
    return api(`${path}/${params.get("q") ? "search" : "canon"}?${params}`);
  }
  async function reload(interactive = false) {
    const params =
      interactive === true
        ? new URLSearchParams({ timeline, limit: "500" })
        : new URLSearchParams(canonFilters);
    if (interactive === true && revision !== "")
      params.set("revision", revision);
    if (interactive === true && at !== "") params.set("at", at);
    if (interactive === true && recordKind) params.set("kind", recordKind);
    if (interactive === true && query) {
      params.set("q", query);
      // Save callbacks refresh without spending another inference attempt.
      params.set("mode", interactive === true ? searchMode : "lexical");
    }
    if (interactive !== true) params.delete("mode");
    const [c, d, s] = await Promise.all([
      loadCanonPage(params),
      api(`${path}/changes`),
      api(`${path}/sessions`),
    ]);
    if (!mounted.current) return;
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
          kind: "type,entity,source,timeline,world_settings",
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
      setStagedBefore(
        Object.fromEntries(
          [...catalog, ...canon.records].map((r) => [r.id, r]),
        ),
      );
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
    const body = { baseRevision, reason, operations: ops };
    const signature = JSON.stringify(body);
    if (proposalRetry.current?.signature !== signature)
      proposalRetry.current = { signature, id: id() };
    await api(`${path}/changes`, { ...body, id: proposalRetry.current.id });
    proposalRetry.current = null;
    setOperations([]);
    setStagedRevision(null);
    await reload();
  }
  async function resume(s) {
    const rows = await api(`${path}/sessions/${s.id}/turns`);
    setSession(s);
    setSceneOpen(true);
    setSetupOpen(false);
    setTurns(rows);
    setListOffsets((old) => ({ ...old, turns: rows.length }));
    setMoreLists((old) => ({ ...old, turns: rows.length === 100 }));
  }
  async function saveTurn(
    text = narration,
    ops = operations,
    candidate = null,
    requestId,
  ) {
    if (
      candidate &&
      (candidate.sessionId !== session?.id ||
        candidate.sessionVersion !== session?.version)
    )
      throw new Error(
        "Resume the candidate’s original story version or generate a fresh turn.",
      );
    const body = {
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
    };
    const signature = JSON.stringify({ sessionId: session.id, ...body });
    if (turnRetry.current?.signature !== signature)
      turnRetry.current = { signature, id: requestId || id() };
    await api(`${path}/sessions/${session.id}/turns`, {
      requestId: turnRetry.current.id,
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
    turnRetry.current = null;
    await resume(await api(`${path}/sessions/${session.id}`));
    setNarration("");
    setOperations([]);
    await reload();
  }
  async function loadMoreCanon() {
    const params = new URLSearchParams(canonFilters);
    params.set("revision", String(canon.revision));
    params.set("offset", String(canon.records.length));
    const next = await loadCanonPage(params, canon.searchToken);
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
    navigate("drafts");
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
  const canSelfReview = canon && !canon.world.archived && canon.world.superuser;
  const records = canon?.records ?? [];
  const formatTick = useMemo(
    () => createCalendarFormatter(canon?.calendar),
    [canon?.calendar],
  );
  const staged = new Map([...catalog, ...records].map((r) => [r.id, r]));
  for (const op of operations) {
    if (op.action === "delete") staged.delete(op.id);
    else staged.set(op.id, op.record);
  }
  const editorRecords = [...staged.values()];
  return (
    <div
      className="wb-world"
      data-section={section}
      aria-busy={busy}
      onClick={(e) => {
        const a = e.target.closest?.('a[href^="#canon-"]');
        if (a) {
          e.preventDefault();
          const target = document.getElementById(
            a.getAttribute("href").slice(1),
          );
          if (target && section === "explore")
            target.scrollIntoView({ behavior: "auto" });
          else if (canon)
            void run(async () => {
              const recordId = a.getAttribute("href").slice("#canon-".length);
              const applied = new URLSearchParams(canonFilters);
              const params = new URLSearchParams({
                timeline: canon.timeline || applied.get("timeline"),
                revision: String(canon.revision),
              });
              if (applied.has("at")) params.set("at", applied.get("at"));
              setLinkedRecord(
                await api(
                  `${path}/records/${encodeURIComponent(recordId)}?${params}`,
                ),
              );
            });
        }
      }}
    >
      {error && (
        <p role="alert">
          {error} <button onClick={() => void run(reload)}>Retry world</button>
        </p>
      )}
      {!canon && !error && <p role="status">Loading published world…</p>}
      {canon && (
        <>
          <h2 ref={heading} tabIndex={-1}>
            {canon.world.name} · canon revision {canon.world.revision}
          </h2>
          <p className="wb-muted">
            Your role: {canon.world.role} · Drafts and story state remain
            separate from published canon.
          </p>
          {document.getElementById("world-header-search") &&
            createPortal(
              <form
                className="wb-header-search"
                onSubmit={(e) => {
                  e.preventDefault();
                  navigate("explore");
                  void run(() => reload(true));
                }}
              >
                <label>
                  <span className="wb-sr-only">Search this world</span>
                  <input
                    placeholder="Search this world…"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                  />
                </label>
                <button aria-label="Search this world">Search</button>
              </form>,
              document.getElementById("world-header-search"),
            )}
          {canon.world.archived && (
            <p className="wb-archive-banner" role="status">
              This world is archived. You can browse its history; creating or
              publishing changes is paused.
              {canon.world.role === "owner" && (
                <button onClick={() => navigate("settings")}>
                  Open world administration to reopen
                </button>
              )}
            </p>
          )}
          <div className="wb-scope-bar">
            <span className="wb-status">
              Published context ·{" "}
              {canon.timeline ||
                new URLSearchParams(canonFilters).get("timeline")}{" "}
              · revision {canon.revision}
            </span>
            <button onClick={() => navigate("explore")}>Change context</button>
            <button onClick={() => navigate("drafts")}>
              Pending approvals ·{" "}
              {changes.filter((c) => c.status === "ready_for_review").length}
            </button>
            <button
              className="wb-nav-toggle"
              aria-expanded={compact ? navOpen : !railCollapsed}
              aria-controls="world-navigation"
              onClick={() => {
                if (window.matchMedia?.("(max-width: 1100px)").matches)
                  setNavOpen((v) => !v);
                else setRailCollapsed((v) => !v);
              }}
            >
              World navigation
            </button>
          </div>
          <div
            ref={layout}
            className={`wb-layout ${inspector ? "has-inspector" : ""} ${railCollapsed ? "rail-collapsed" : ""}`}
          >
            <nav
              ref={navigation}
              role={navOpen ? "dialog" : undefined}
              aria-modal={navOpen || undefined}
              tabIndex={-1}
              id="world-navigation"
              className={`wb-nav ${navOpen ? "is-open" : ""} ${railCollapsed ? "is-collapsed" : ""}`}
              aria-label="World"
            >
              <button
                className="wb-drawer-close"
                onClick={() => setNavOpen(false)}
              >
                Close navigation
              </button>
              {Object.entries(sections).map(([key, label]) => (
                <a
                  key={key}
                  href={worldHref(worldId, key)}
                  aria-current={section === key ? "page" : undefined}
                  onClick={(e) => {
                    e.preventDefault();
                    navigate(key);
                  }}
                >
                  {label}
                </a>
              ))}
              <div ref={setHistoryHost} />
              <p className="wb-muted">Your role: {canon.world.role}</p>
            </nav>
            <div className="wb-main">
              <div hidden={section !== "workspace"}>
                <WorldWorkspace
                  historyHost={historyHost}
                  onConversation={() => navigate("workspace")}
                  api={api}
                  path={path}
                  scope={{
                    worldId,
                    revision: canon.revision,
                    timeline:
                      canon.timeline ||
                      new URLSearchParams(canonFilters).get("timeline"),
                    at: new URLSearchParams(canonFilters).get("at") || "",
                    query: new URLSearchParams(canonFilters).get("q") || "",
                  }}
                  canon={canon}
                  canEdit={canEdit}
                  session={session}
                  prompt={prompt}
                  references={references}
                  onRemoveReference={(r) =>
                    setReferences((rows) => rows.filter((x) => x !== r))
                  }
                  onInspect={(source) => {
                    inspectorReturn.current = document.activeElement;
                    setInspector(source);
                  }}
                  onExplore={() => navigate("explore")}
                  onStories={(text) => {
                    if (text)
                      setPremiseCandidate({
                        text,
                        revision: canon.revision,
                        timeline:
                          canon.timeline ||
                          new URLSearchParams(canonFilters).get("timeline"),
                      });
                    setSetupOpen(true);
                    setSceneOpen(false);
                    navigate("stories");
                  }}
                  onPremise={(value) => {
                    setPremiseCandidate(value);
                    setSetupOpen(true);
                    setSceneOpen(false);
                    navigate("stories");
                  }}
                  onPropose={async (result, candidateReason, requestId) => {
                    await api(`${path}/changes`, {
                      id: requestId,
                      baseRevision: result.baseRevision,
                      reason: candidateReason,
                      operations: result.operations,
                    });
                    await reload();
                  }}
                  onReviewStory={() => {
                    setReconcileTarget({ kind: "sessions", id: session.id });
                    navigate("drafts");
                  }}
                  onSaveTurn={(result, requestId) =>
                    saveTurn(
                      result.answer,
                      result.operations,
                      result,
                      requestId,
                    )
                  }
                  onEdit={async (result) => {
                    if (operations.length) {
                      setError(
                        "Save or remove your staged changes before opening another candidate.",
                      );
                      navigate("drafts");
                      return;
                    }
                    try {
                      const original = await api(
                        `${path}/canon?${new URLSearchParams({ revision: String(result.baseRevision), timeline: result.timeline, limit: "500" })}`,
                      );
                      // Fetch every affected before-state, even when it lies beyond the first page.
                      const before = {};
                      const differences = await Promise.all(
                        result.operations.map(async (op) => {
                          const recordId = op.id || op.record?.id;
                          const record = original.records.find(
                            (r) => r.id === recordId,
                          );
                          if (record) return [recordId, record];
                          if (original.total <= original.records.length)
                            return [recordId, null];
                          const response = await api(
                            `${path}/records/${encodeURIComponent(recordId)}?${new URLSearchParams({ revision: String(result.baseRevision), timeline: result.timeline })}`,
                          ).catch((error) => {
                            if (error.status === 404) return null;
                            throw error;
                          });
                          return [recordId, response?.record || null];
                        }),
                      );
                      for (const [key, value] of differences)
                        before[key] = value;
                      setOperations(
                        result.operations.map((op) => ({
                          ...op,
                          id: op.id || op.record?.id,
                        })),
                      );
                      setStagedRevision(result.baseRevision);
                      setStagedBefore(before);
                      setReason(result.answer.slice(0, 2000));
                      setCanon(original);
                      setTimeline(result.timeline);
                      setRevision(String(result.baseRevision));
                      setAt("");
                      setQuery("");
                      setCanonFilters(
                        new URLSearchParams({
                          timeline: result.timeline,
                          revision: String(result.baseRevision),
                          limit: "500",
                        }).toString(),
                      );
                      navigate("drafts");
                    } catch (e) {
                      setError(e.message);
                    }
                  }}
                />
              </div>
              <section
                hidden={section !== "explore"}
                aria-label="Explore canon"
              >
                <fieldset disabled={busy}>
                  <legend>Wiki and history</legend>
                  <form
                    className="wb-explore-filters"
                    onSubmit={(e) => {
                      e.preventDefault();
                      void run(() => reload(true));
                    }}
                  >
                    <label>
                      Search canon
                      <input
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                      />
                    </label>
                    <label>
                      Record type
                      <select
                        value={recordKind}
                        onChange={(e) => setRecordKind(e.target.value)}
                      >
                        <option value="">All records</option>
                        {[
                          "entity",
                          "event",
                          "fact",
                          "relationship",
                          "source",
                          "type",
                          "timeline",
                          "world_settings",
                          "rule",
                        ].map((kind) => (
                          <option key={kind} value={kind}>
                            {humanLabel(kind)}
                          </option>
                        ))}
                      </select>
                    </label>
                    <details className="wb-advanced-filters">
                      <summary>Timeline and history filters</summary>
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
                    </details>
                    {canon.world.semanticSearch && (
                      <label>
                        Search method
                        <select
                          value={searchMode}
                          disabled={!!recordKind}
                          onChange={(e) => setSearchMode(e.target.value)}
                        >
                          <option value="lexical">Words</option>
                          <option value="hybrid">
                            Words and meaning · uses AI allowance
                          </option>
                        </select>
                      </label>
                    )}
                    {recordKind && (
                      <p>Type filtering uses published canon word matches.</p>
                    )}
                    <button>Load canon</button>
                  </form>
                  <p>
                    Showing {records.length} of {canon.total} records at
                    revision {canon.revision}. Dates show when something is true
                    in this world.
                  </p>
                  <details>
                    <summary>Advanced date help</summary>
                    <p>
                      A record applies from its start up to, but not including,
                      its end.
                    </p>
                  </details>
                  {canon.mode === "hybrid" && (
                    <p>
                      Combined word matches and up to 100 meaning-based
                      candidates from current canon. Each search or additional
                      page uses one AI attempt.
                    </p>
                  )}
                  <WorldViews
                    records={records}
                    total={canon.total}
                    calendar={canon.calendar}
                    scope={{
                      revision: canon.revision,
                      timeline:
                        canon.timeline ||
                        new URLSearchParams(canonFilters).get("timeline"),
                      at: new URLSearchParams(canonFilters).get("at") || "",
                      query: new URLSearchParams(canonFilters).get("q") || "",
                    }}
                  />
                  <div className="world-records">
                    {records.map((r) => (
                      <article id={`canon-${r.id}`} key={r.id}>
                        <h3>
                          {r.label || r.id} <small>{humanLabel(r.kind)}</small>
                        </h3>
                        {r.attributes?.description && (
                          <p className="wb-prose">{r.attributes.description}</p>
                        )}
                        <p>
                          {r.subject && (
                            <a href={`#canon-${r.subject}`}>
                              {catalog.find((x) => x.id === r.subject)?.label ||
                                r.subject}
                            </a>
                          )}{" "}
                          {r.predicate}{" "}
                          {r.target && (
                            <a href={`#canon-${r.target}`}>
                              {catalog.find((x) => x.id === r.target)?.label ||
                                r.target}
                            </a>
                          )}{" "}
                          {r.value !== undefined &&
                            (r.valueType === "tick"
                              ? formatTick(r.value)
                              : String(r.value))}
                        </p>
                        {r.source && (
                          <p>
                            Source:{" "}
                            <a href={`#canon-${r.source}`}>
                              {catalog.find((x) => x.id === r.source)?.label ||
                                r.source}
                            </a>
                          </p>
                        )}
                        {r.from !== undefined && (
                          <p>
                            From {formatTick(r.from)}
                            {r.until !== undefined
                              ? ` until ${formatTick(r.until)} (exclusive)`
                              : " onwards"}
                          </p>
                        )}
                        {(r.fields || r.calendar) && (
                          <details>
                            <summary>Record structure and calendar</summary>
                            {r.fields && (
                              <p>
                                Fields:{" "}
                                {Object.entries(r.fields)
                                  .map(([k, v]) => `${k} (${v})`)
                                  .join(", ")}
                              </p>
                            )}
                            {r.calendar && (
                              <p>
                                Calendar: {r.calendar.name}. Epoch tick{" "}
                                {r.calendar.epochTick}, year{" "}
                                {r.calendar.epochYear}; {r.calendar.ticksPerDay}{" "}
                                ticks per day; {r.calendar.cycleYears}-year
                                cycle. Months:{" "}
                                {r.calendar.months
                                  .map(
                                    (m) =>
                                      `${m.name}: ${m.days} days${m.leapDays ? ` + ${m.leapDays} in leap years` : ""}`,
                                  )
                                  .join("; ")}
                                . Leap positions:{" "}
                                {r.calendar.leapYears?.join(", ") || "none"}.
                                Weekdays:{" "}
                                {r.calendar.weekdays?.join(", ") || "none"};
                                epoch position {r.calendar.weekdayAtEpoch}.
                              </p>
                            )}
                          </details>
                        )}
                        {r.kind === "world_settings" && (
                          <p>
                            Human review policy:{" "}
                            {r.reviewPolicy?.requiredApprovals ?? 1} independent
                            approval(s).
                          </p>
                        )}
                        {r.attributes && (
                          <dl>
                            {Object.entries(r.attributes).map(
                              ([key, value]) =>
                                key !== "description" && (
                                  <div key={key}>
                                    <dt>{humanLabel(key)}</dt>
                                    <dd>{value}</dd>
                                  </div>
                                ),
                            )}
                          </dl>
                        )}
                        <button
                          type="button"
                          onClick={() =>
                            setConnectionScope({
                              id: r.id,
                              revision: canon.revision,
                              timeline:
                                new URLSearchParams(canonFilters).get(
                                  "timeline",
                                ) || timeline,
                              at:
                                new URLSearchParams(canonFilters).get("at") ||
                                "",
                            })
                          }
                        >
                          Explore connections of {r.label || r.id}
                        </button>
                        <span className="wb-status">Published canon</span>
                        <button
                          onClick={() => (
                            (inspectorReturn.current = document.activeElement),
                            setInspector({
                              worldId,
                              id: r.id,
                              revision: canon.revision,
                              timeline:
                                canon.timeline ||
                                new URLSearchParams(canonFilters).get(
                                  "timeline",
                                ),
                              at:
                                new URLSearchParams(canonFilters).get("at") ||
                                "",
                            })
                          )}
                        >
                          Inspect {r.label || r.id}
                        </button>
                        <button
                          onClick={() => {
                            const reference = {
                              worldId,
                              id: r.id,
                              label: r.label,
                              revision: canon.revision,
                              timeline:
                                canon.timeline ||
                                new URLSearchParams(canonFilters).get(
                                  "timeline",
                                ),
                            };
                            setReferences((rows) =>
                              [
                                ...rows.filter((x) => x.id !== r.id),
                                reference,
                              ].slice(-8),
                            );
                            setPrompt({
                              text: `Tell me about ${r.label || r.id}`,
                              intent: "ask",
                            });
                            navigate("workspace");
                          }}
                        >
                          Ask AI about {r.label || r.id}
                        </button>
                        {canEdit && (
                          <button
                            type="button"
                            onClick={() => {
                              editRecord(staged.get(r.id) || r);
                            }}
                          >
                            Edit {r.label || r.id}
                          </button>
                        )}
                        {canEdit && r.kind !== "world_settings" && (
                          <button
                            type="button"
                            onClick={() =>
                              stage({ action: "delete", id: r.id })
                            }
                          >
                            Propose removal
                          </button>
                        )}
                      </article>
                    ))}
                  </div>
                </fieldset>
                {canon.records.length < canon.total && (
                  <button
                    disabled={busy}
                    onClick={() => void run(loadMoreCanon)}
                  >
                    Load more canon records
                  </button>
                )}
              </section>
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
              <section
                hidden={section !== "settings"}
                aria-label="World configuration"
              >
                <WorldConfiguration
                  record={
                    catalog.find((r) => r.kind === "world_settings") ||
                    records.find((r) => r.kind === "world_settings")
                  }
                  revision={canon.revision}
                  timelines={catalog.filter((r) => r.kind === "timeline")}
                  canEdit={canEdit}
                  onAdvanced={editRecord}
                  onExplore={() => navigate("explore")}
                  onStage={(record, originalRevision) => {
                    if (originalRevision !== canon.revision)
                      throw new Error(
                        "Canon changed while you edited. Load the original revision or begin a fresh edit.",
                      );
                    if (
                      operations.length &&
                      stagedRevision !== originalRevision
                    )
                      throw new Error(
                        "Save or clear your earlier draft before editing configuration.",
                      );
                    stage({ action: "put", id: record.id, record });
                    setReason(`Update ${record.label}`);
                    navigate("drafts");
                  }}
                />
                <details>
                  <summary>Advanced timelines and branches</summary>
                  <WorldTimelineBranch
                    api={api}
                    path={path}
                    timeline={
                      canon.timeline ||
                      new URLSearchParams(canonFilters).get("timeline")
                    }
                    revision={canon.revision}
                    currentRevision={canon.world.revision}
                    canEdit={canEdit}
                    onSaved={reload}
                  />
                </details>
                {canon.world.role === "owner" && (
                  <details id="world-administration">
                    <summary>
                      World administration · members, ownership and archive
                    </summary>
                    <WorldAdministration
                      api={api}
                      path={path}
                      world={canon.world}
                      onSaved={reload}
                    />
                  </details>
                )}
                <button onClick={() => navigate("drafts")}>
                  View configuration and publication history
                </button>
              </section>
              <section
                hidden={section !== "drafts"}
                aria-label="Drafts and proposals"
              >
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
                      timeline={
                        canon.timeline ||
                        new URLSearchParams(canonFilters).get("timeline")
                      }
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
                        Draft based on revision {stagedRevision}. Refreshing
                        canon does not rebase staged changes.
                      </p>
                    )}
                    {operations.map((op) => (
                      <div key={op.id}>
                        <strong>
                          {op.action} {op.id}
                        </strong>
                        <details>
                          <summary>Before and after</summary>
                          <div className="world-diff">
                            <section>
                              <h4>Before</h4>
                              <RecordSummary record={stagedBefore[op.id]} />
                            </section>
                            <section>
                              <h4>After</h4>
                              <RecordSummary record={op.record} />
                            </section>
                          </div>
                        </details>
                        {op.record && (
                          <button
                            onClick={() =>
                              editRecord(op.record, stagedRevision)
                            }
                          >
                            Edit draft {op.record.label || op.id}
                          </button>
                        )}
                        <button
                          onClick={() =>
                            setOperations(
                              operations.filter((x) => x.id !== op.id),
                            )
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

                <fieldset disabled={busy}>
                  <legend>Review queue</legend>
                  <details>
                    <summary>How approval works</summary>
                    <p>
                      Displayed revision policy:{" "}
                      {canon.reviewPolicy?.requiredApprovals ?? 1} distinct
                      human approval(s), from owners, admins or reviewers other
                      than the author. Superusers may approve their own
                      proposals. Each proposal retains its required count;
                      policy changes use the stricter count. Publication
                      rechecks current membership authority, canon revision and
                      deterministic rules; AI cannot approve.
                    </p>
                  </details>
                  <label>
                    Review reason
                    <input
                      value={reviewReason}
                      onChange={(e) => setReviewReason(e.target.value)}
                      maxLength={2000}
                    />
                  </label>
                  <label>
                    Filter proposals
                    <select
                      value={proposalFilter}
                      onChange={(e) => setProposalFilter(e.target.value)}
                    >
                      <option value="all">All proposals</option>
                      <option value="mine">My drafts</option>
                      <option value="review">Needs review</option>
                      <option value="conflicts">Conflicts</option>
                      <option value="published">Published</option>
                      <option value="rejected">Rejected</option>
                    </select>
                  </label>
                  {!changes.length && (
                    <p>
                      No proposals yet. Create one in conversation or use the
                      structured editor.
                    </p>
                  )}
                  {changes
                    .filter(
                      (c) =>
                        proposalFilter === "all" ||
                        (proposalFilter === "mine"
                          ? c.author === email && c.status === "draft"
                          : proposalFilter === "review"
                            ? ["ready_for_review", "approved"].includes(
                                c.status,
                              )
                            : proposalFilter === "conflicts"
                              ? c.baseRevision !== canon.world.revision &&
                                !["published", "rejected"].includes(c.status)
                              : c.status === proposalFilter),
                    )
                    .map((c) => (
                      <article key={c.id}>
                        <h3>{c.reason}</h3>
                        <a
                          href={worldHref(worldId, "drafts", {
                            proposal: c.id,
                          })}
                        >
                          Link to proposal
                        </a>
                        {c.baseRevision !== canon.world.revision &&
                          !["published", "rejected"].includes(c.status) && (
                            <p role="status">
                              Conflict: canon has advanced. Compare and
                              reconcile before review.
                            </p>
                          )}
                        <p>
                          <span className="wb-status">
                            {humanLabel(c.status)}
                          </span>{" "}
                          · based on revision {c.baseRevision} · by {c.author}
                        </p>
                        <div
                          className="wb-proposal-summary"
                          aria-label="Proposed changes"
                        >
                          {c.operations.map((op, i) => (
                            <p key={i}>
                              {op.action === "delete" ? "Remove" : "Update"}{" "}
                              {op.record?.label ||
                                catalog.find((r) => r.id === op.id)?.label ||
                                op.id}
                              {op.record?.attributes &&
                                Object.entries(op.record.attributes).map(
                                  ([key, value]) => (
                                    <span key={key}>
                                      {" "}
                                      · {humanLabel(key)}: {value}
                                    </span>
                                  ),
                                )}
                            </p>
                          ))}
                        </div>
                        <details>
                          <summary>Advanced operation details</summary>
                          <pre>{JSON.stringify(c.operations, null, 2)}</pre>
                        </details>
                        <p>
                          {c.status === "published"
                            ? "Approvals at publication: "
                            : ""}
                          {c.approvals?.length ??
                            (c.status === "approved" ? 1 : 0)}{" "}
                          of {c.requiredApprovals ?? 1} required approvals.
                        </p>
                        {c.reviews?.length > 0 && (
                          <ul aria-label={`Reviews for ${c.reason}`}>
                            {c.reviews.map((review, i) => (
                              <li key={i}>
                                {review.reviewer}: {review.decision} —{" "}
                                {review.reason}.
                                {review.superuserSelfReview &&
                                  " Superuser self-approval."}
                                {review.createdAt
                                  ? ` Reviewed ${review.createdAt}.`
                                  : " Legacy review; original time unavailable."}
                                {!review.eligible &&
                                  " Membership authority changed; this review cannot support a new publication."}
                              </li>
                            ))}
                          </ul>
                        )}
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
                          timeline={
                            canon.timeline ||
                            new URLSearchParams(canonFilters).get("timeline")
                          }
                          archived={canon.world.archived}
                        />
                        {c.status === "draft" &&
                          c.author === email &&
                          canEdit && (
                            <button
                              onClick={() =>
                                void run(async () => {
                                  await api(
                                    `${path}/changes/${c.id}/submit`,
                                    {},
                                  );
                                  await reload();
                                })
                              }
                            >
                              Submit for review
                            </button>
                          )}
                        {["ready_for_review", "approved"].includes(c.status) &&
                          (canReview ||
                            (canSelfReview && c.author === email)) &&
                          (c.author !== email || canSelfReview) &&
                          !c.reviews?.some(
                            (r) => r.reviewer === email && r.eligible,
                          ) &&
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
                        {c.status === "draft" &&
                          !canEdit &&
                          !canon.world.archived && (
                            <p>
                              Only an editor with access to this world can
                              submit their own draft.
                            </p>
                          )}
                        {["ready_for_review", "approved"].includes(c.status) &&
                          c.author === email &&
                          !canSelfReview && (
                            <p>Another reviewer must review your proposal.</p>
                          )}
                        {c.status === "approved" && (
                          <p>
                            {c.baseRevision !== canon.world.revision
                              ? "Compare and reconcile with the latest canon before publishing."
                              : (c.approvals?.length ?? 1) <
                                  (c.requiredApprovals ?? 1)
                                ? "More human approvals are needed before publishing."
                                : "Publication is available to an authorized reviewer."}
                          </p>
                        )}
                        {c.status === "approved" &&
                          (canReview ||
                            (canSelfReview && c.author === email)) && (
                            <button
                              disabled={
                                c.baseRevision !== canon.world.revision ||
                                (c.approvals?.length ?? 1) <
                                  (c.requiredApprovals ?? 1)
                              }
                              onClick={() =>
                                void run(async () => {
                                  await api(
                                    `${path}/changes/${c.id}/publish`,
                                    {},
                                  );
                                  await reload();
                                })
                              }
                            >
                              Publish approved change
                            </button>
                          )}
                        {c.reviewer && !c.reviews?.length && (
                          <p>
                            Reviewed by {c.reviewer}: {c.reviewReason}
                          </p>
                        )}
                      </article>
                    ))}
                  {listMore("changes", "proposals")}
                </fieldset>
              </section>
              <section
                hidden={section !== "stories"}
                aria-label="Story library"
              >
                {sceneOpen && session ? (
                  <>
                    <WorldStoryScene
                      key={session.id}
                      api={api}
                      path={path}
                      worldId={worldId}
                      session={session}
                      turns={turns}
                      archived={canon.world.archived}
                      canEdit={canEdit}
                      onSaveTurn={(result, requestId) =>
                        saveTurn(
                          result.answer,
                          result.operations,
                          result,
                          requestId,
                        )
                      }
                      onInspect={(source) => {
                        inspectorReturn.current = document.activeElement;
                        setInspector(source);
                      }}
                      onReview={() => {
                        setReconcileTarget({
                          kind: "sessions",
                          id: session.id,
                        });
                        navigate("drafts");
                      }}
                      onLibrary={() => {
                        setSceneOpen(false);
                        setInspector(null);
                      }}
                      onWorkspace={() => {
                        setPrompt({
                          text: "Continue the story",
                          intent: "play",
                        });
                        navigate("workspace");
                      }}
                      moreTurns={listMore("turns", "turns")}
                    />
                    <a
                      href={worldHref(worldId, "stories", {
                        session: session.id,
                      })}
                    >
                      Link to story
                    </a>
                    <details>
                      <summary>Manual narration authoring</summary>
                      <p>
                        Write narration yourself and save it with any staged
                        provisional changes. This authoring control does not
                        process a player action.
                      </p>
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
                    </details>
                  </>
                ) : (
                  <>
                    <h2>Stories</h2>
                    <p className="wb-muted">
                      Resume a private story or begin with a published world.
                    </p>
                    <div className="wb-story-library">
                      {sessions.map((s) => (
                        <article key={s.id}>
                          <h3>{s.title}</h3>
                          <p>
                            Private session state · turn {s.version} · revision{" "}
                            {s.pinnedRevision}
                          </p>
                          <button
                            disabled={busy}
                            onClick={() =>
                              void run(async () =>
                                resume(await api(`${path}/sessions/${s.id}`)),
                              )
                            }
                          >
                            Resume {s.title}
                          </button>
                        </article>
                      ))}
                    </div>
                    {listMore("sessions", "stories")}
                    {!sessions.length && <p>No saved stories yet.</p>}
                    <button
                      disabled={canon.world.archived}
                      aria-expanded={setupOpen}
                      onClick={() => setSetupOpen((v) => !v)}
                    >
                      Create story
                    </button>
                    <button onClick={() => setSetupOpen(true)}>
                      Browse story definitions
                    </button>
                    <div hidden={!setupOpen}>
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
                    </div>
                  </>
                )}
              </section>
            </div>
            {inspector && (
              <WorldInspector
                key={`${inspector.id}:${inspector.revision}:${inspector.timeline}`}
                api={api}
                path={path}
                scope={inspector}
                returnFocus={inspectorReturn}
                canEdit={canEdit}
                onClose={() => {
                  setInspector(null);
                  (inspectorReturn.current?.isConnected
                    ? inspectorReturn.current
                    : heading.current
                  )?.focus({ preventScroll: true });
                }}
                onEdit={editRecord}
                onAsk={(reference) => {
                  setReferences((rows) =>
                    [
                      ...rows.filter((r) => r.id !== reference.id),
                      reference,
                    ].slice(-8),
                  );
                  setPrompt({
                    text: `Tell me about ${reference.label || reference.id}`,
                    intent: "ask",
                    scope: reference,
                  });
                  setInspector(null);
                  navigate("workspace");
                }}
              />
            )}
          </div>
        </>
      )}
    </div>
  );
}
