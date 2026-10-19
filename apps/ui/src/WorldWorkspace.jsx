import { createPortal } from "react-dom";
import { useEffect, useRef, useState, useMemo } from "react";
import WorldResponse from "./WorldResponse.jsx";
import { RecordSummary } from "./WorldResponse.jsx";
import WorldGenerations from "./WorldGenerations.jsx";
import useLocalWriting from "./useLocalWriting.js";
import { parseWorldJSON } from "./worldTicks.js";
import { readWorldRoute, worldHref } from "./worldWorkspace.js";
import WorldViews from "./WorldViews.jsx";
import {
  conversationQuestion,
  detectIntent,
  workflows,
} from "./worldWorkspace.js";

export default function WorldWorkspace({
  api,
  email,
  path,
  historyHost,
  onConversation,
  scope,
  canon,
  canEdit,
  session,
  prompt,
  references,
  onRemoveReference,
  onInspect,
  onEdit,
  onPropose,
  onSaveTurn,
  onPremise,
  onStories,
  onExplore,
  onReviewStory,
}) {
  const [threads, setThreads] = useState([
    { id: crypto.randomUUID(), title: "New conversation", messages: [] },
  ]);
  const [selected, setSelected] = useState(threads[0].id);
  const [question, setQuestion] = useState(""),
    [intent, setIntent] = useState("auto");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [clarify, setClarify] = useState(false);
  const [progress, setProgress] = useState("");
  const [visuals, setVisuals] = useState(false);
  const controller = useRef(null),
    composer = useRef(null),
    log = useRef(null);
  const follow = useRef(true);
  const thread = threads.find((t) => t.id === selected) || threads[0];
  const shared = !!canon.world.capabilities?.conversations;
  const writing = useMemo(() => ({ question, intent }), [question, intent]);
  const { localSaved, localError } = useLocalWriting(
    `${path}/composer`,
    writing,
    (v) => {
      setQuestion(v.question || "");
      setIntent(v.intent || "auto");
    },
  );
  async function loadThread(c) {
    const rows = [];
    let page;
    do {
      page = await api(
        `${path}/conversations/${c.id}/messages?offset=${rows.length}`,
      );
      rows.push(...page);
    } while (page.length === 100);
    const messages = rows.flatMap((m) => {
      const input = m.request;
      const pin = {
        worldId: scope.worldId,
        revision: input.revision,
        timeline: input.timeline,
        at: input.at,
      };
      return [
        {
          id: m.id,
          role: "user",
          actor: m.actor,
          text: input.question,
          intent: Object.keys(workflows).find(
            (k) => workflows[k] === input.workflow,
          ),
          scope: pin,
          status: m.status,
        },
        ...(m.result
          ? [
              {
                id: `${m.id}-result`,
                role: "assistant",
                text: m.result.answer,
                scope: pin,
                result: {
                  ...m.result,
                  workflow: input.workflow,
                  timeline: input.timeline,
                },
              },
            ]
          : []),
      ];
    });
    setThreads((old) =>
      old.map((t) =>
        t.id === c.id
          ? {
              ...t,
              messages,
              saved: true,
              scope: {
                worldId: scope.worldId,
                revision: c.revision ?? t.scope?.revision,
                timeline: c.timeline ?? t.scope?.timeline,
                at: c.at ?? t.scope?.at,
              },
            }
          : t,
      ),
    );
  }
  useEffect(() => {
    if (!shared) return;
    let active = true;
    const load = async () => {
      const rows = [];
      let page;
      do {
        page = await api(`${path}/conversations?offset=${rows.length}`);
        rows.push(...page);
      } while (page.length === 100);
      if (!active) return;
      setThreads((old) => [
        ...old.filter((t) => !rows.some((c) => c.id === t.id)),
        ...rows.map((c) => ({
          ...c,
          saved: true,
          messages: [],
          scope: {
            worldId: scope.worldId,
            revision: c.revision,
            timeline: c.timeline,
            at: c.at,
          },
        })),
      ]);
      const link = readWorldRoute().params?.get("conversation");
      const c = rows.find((c) => c.id === link);
      if (c) {
        setSelected(c.id);
        await loadThread(c);
      }
    };
    void load().catch((e) => setError(e.message));
    return () => {
      active = false;
    };
  }, [path, shared]);
  useEffect(() => {
    if (!shared || !thread.saved || busy) return;
    const timer = setInterval(
      () => void loadThread(thread).catch((e) => setError(e.message)),
      5000,
    );
    return () => clearInterval(timer);
  }, [shared, selected, thread.saved, busy]);
  async function streamResult(route, signal) {
    const response = await fetch(`/api/worlds${route}`, {
      method: "POST",
      credentials: "same-origin",
      redirect: "error",
      headers: { "Content-Type": "application/json" },
      body: "{}",
      signal,
    });
    if (!response.ok)
      throw Error(
        (await response.json()).error || "Could not start generation.",
      );
    const reader = response.body.getReader(),
      decoder = new TextDecoder();
    let buffer = "",
      result;
    while (true) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
      let end;
      while ((end = buffer.indexOf("\n\n")) >= 0) {
        const block = buffer.slice(0, end);
        buffer = buffer.slice(end + 2);
        const event = block.match(/^event: (.+)$/m)?.[1],
          raw = block.match(/^data: (.+)$/m)?.[1];
        if (!raw) continue;
        const payload = parseWorldJSON(raw);
        if (event === "progress") setProgress(payload.message);
        if (event === "error") throw Error(payload.error);
        if (event === "result") result = payload;
      }
      if (done) break;
    }
    if (!result)
      throw Error(
        "Connection ended before a validated result. Refresh the saved conversation to recover it.",
      );
    return result;
  }
  const resolved = intent === "auto" ? detectIntent(question) : intent;
  const workflow = workflows[resolved] || "canon_query";
  const appliedScope =
    resolved === "play" && session
      ? {
          ...scope,
          revision: session.pinnedRevision,
          timeline: session.timeline,
          at: "",
        }
      : thread.scope || scope;
  useEffect(() => () => controller.current?.abort(), []);
  useEffect(() => {
    if (thread.messages.length && follow.current && log.current)
      log.current.scrollTop = log.current.scrollHeight;
  }, [thread.messages, busy, error, clarify]);
  useEffect(() => {
    if (!prompt) return;
    setQuestion(prompt.text);
    setIntent(prompt.intent || "auto");
    if (prompt.scope?.worldId === scope.worldId) {
      setThreads((rows) =>
        rows.map((t) =>
          t.id === selected ? { ...t, scope: prompt.scope } : t,
        ),
      );
    }
    setClarify(false);
    composer.current?.focus();
  }, [prompt]);
  function addMessages(id, messages) {
    setThreads((rows) =>
      rows.map((t) =>
        t.id === id
          ? {
              ...t,
              title: t.messages.length
                ? t.title
                : messages[0].text.slice(0, 48),
              messages: [...t.messages, ...messages],
            }
          : t,
      ),
    );
  }
  function candidate(value, input, threadId = selected) {
    const candidateScope = {
      worldId: scope.worldId,
      timeline: input.timeline,
      revision: value.baseRevision,
    };
    addMessages(threadId, [
      {
        id: crypto.randomUUID(),
        role: "assistant",
        intent:
          Object.keys(workflows).find((k) => workflows[k] === input.workflow) ||
          "ask",
        scope: candidateScope,
        text: value.answer,
        result: {
          ...value,
          workflow: input.workflow,
          direction: input.question,
          timeline: input.timeline,
          sessionId: input.sessionId,
        },
      },
    ]);
  }
  async function send(e) {
    e.preventDefault();
    if (!resolved) {
      setClarify(true);
      return;
    }
    if (resolved === "play" && !session) {
      onStories(question);
      return;
    }
    setError("");
    let input;
    try {
      input = {
        workflow,
        question: conversationQuestion(
          question.trim(),
          resolved === "play" ? [] : thread.messages,
          references,
        ),
        revision: appliedScope.revision,
        timeline: appliedScope.timeline,
        ...(appliedScope.at ? { at: appliedScope.at } : {}),
        ...(resolved === "play"
          ? { sessionId: session.id, expectedVersion: session.version }
          : {}),
      };
    } catch (e) {
      setError(e.message);
      return;
    }
    follow.current = true;
    let threadId = selected;
    if (resolved === "play" && !thread.private) {
      threadId = crypto.randomUUID();
      setThreads((rows) => [
        ...rows,
        {
          id: threadId,
          title: "Private story conversation",
          messages: [],
          private: true,
        },
      ]);
      setSelected(threadId);
    }

    addMessages(threadId, [
      {
        id: crypto.randomUUID(),
        role: "user",
        intent: resolved,
        text: question.trim(),
        scope: appliedScope,
      },
    ]);
    setQuestion("");
    setBusy(true);
    setClarify(false);
    const request = new AbortController();
    controller.current = request;
    try {
      let result;
      if (shared && resolved !== "play" && !thread.private) {
        if (!thread.saved) {
          await api(`${path}/conversations`, {
            id: threadId,
            title: question.trim().slice(0, 48),
            revision: input.revision,
            timeline: input.timeline,
            ...(input.at ? { at: input.at } : {}),
          });
          setThreads((rows) =>
            rows.map((t) =>
              t.id === threadId
                ? { ...t, saved: true, scope: appliedScope }
                : t,
            ),
          );
        }
        const messageId = crypto.randomUUID();
        await api(`${path}/conversations/${threadId}/messages`, {
          id: messageId,
          request: input,
        });
        onConversation?.(threadId);
        result = await streamResult(
          `${path}/conversations/${threadId}/messages/${messageId}/generate`,
          request.signal,
        );
      } else
        result = await api(`${path}/reason`, input, { signal: request.signal });
      if (!request.signal.aborted) candidate(result, input, threadId);
    } catch (e) {
      if (!request.signal.aborted)
        setError(
          `${e.message} Your message is retained; retry only when ready to use another AI attempt.`,
        );
    } finally {
      setBusy(false);
      controller.current = null;
    }
  }
  const starter = (text, mode) => {
    setQuestion(text);
    setIntent(mode);
    composer.current?.focus();
  };
  const conversationTools = (
    <div className="wb-thread-tools">
      <button
        disabled={busy}
        onClick={() => {
          const next = {
            id: crypto.randomUUID(),
            title: "New conversation",
            messages: [],
          };
          setThreads((old) => [...old, next]);
          setSelected(next.id);
          setQuestion("");
          setError("");
          onConversation?.();
        }}
      >
        New conversation
      </button>
      <label>
        Recent conversations
        <select
          value={selected}
          disabled={busy}
          onChange={(e) => {
            setSelected(e.target.value);
            setError("");
            const c = threads.find((t) => t.id === e.target.value);
            if (c?.saved) void loadThread(c).catch((e) => setError(e.message));
            onConversation?.(c?.saved ? c.id : undefined);
          }}
        >
          {threads.map((t) => (
            <option key={t.id} value={t.id}>
              {t.title}
            </option>
          ))}
        </select>
      </label>
    </div>
  );
  return (
    <section className="wb-conversation" aria-label="AI workspace">
      {historyHost && createPortal(conversationTools, historyHost)}
      <div
        ref={log}
        onScroll={() => {
          const node = log.current;
          follow.current =
            node.scrollHeight - node.scrollTop - node.clientHeight < 80;
        }}
        className="wb-thread"
        role="log"
        aria-label="Conversation"
        aria-live="polite"
        aria-relevant="additions"
      >
        {!historyHost && conversationTools}
        <details className="wb-conversation-notice">
          <summary>Conversation context and history</summary>
          <p className="wb-muted">
            {shared
              ? "World conversations are shared with all current members and saved with attribution. Play stays private."
              : "Conversations last while this world stays open. Stories and saved proposals persist."}{" "}
            AI answers use the last three requests and attached references as
            context; earlier answers are not canon.
          </p>
        </details>
        {session && (
          <details>
            <summary>
              Active story · {session.title} · turn {session.version}
            </summary>
            <p>
              Private session state · pinned revision {session.pinnedRevision} ·{" "}
              {session.timeline}. Outcomes require human review.
            </p>
            {session.memory?.summary && <p>{session.memory.summary}</p>}
            {session.definition?.participants?.map((id) => (
              <button
                key={id}
                onClick={() =>
                  onInspect({
                    worldId: scope.worldId,
                    id,
                    revision: session.pinnedRevision,
                    timeline: session.timeline,
          at: "",
                  })
                }
              >
                Inspect character · {id}
              </button>
            ))}
            {Object.values(session.state || {}).map((record) => (
              <article key={record.id}>
                <span className="wb-status">Session state</span>
                <RecordSummary record={record} />
              </article>
            ))}
            {canEdit && !!Object.keys(session.state || {}).length && (
              <button onClick={onReviewStory}>Review story outcomes</button>
            )}
          </details>
        )}
        {!thread.messages.length && (
          <div className="wb-welcome">
            <p className="workspace-eyebrow">Your world, in conversation</p>
            <h2>What happens next?</h2>
            <p>Ask, imagine and shape this world.</p>
            <div className="wb-starters">
              <button
                onClick={() =>
                  starter("What do we know about this world?", "ask")
                }
              >
                Ask about canon
              </button>
              <button
                disabled={!canEdit}
                onClick={() => starter("Create a character who ", "create")}
              >
                Create a character
              </button>
              <button
                disabled={!canEdit}
                onClick={() => starter("Suggest a change to ", "change")}
              >
                Suggest a change
              </button>
              <button onClick={() => onStories("")}>Start an adventure</button>
            </div>
          </div>
        )}
        {threads.map((conversation) => (
          <div
            key={conversation.id}
            className="wb-messages"
            hidden={conversation.id !== selected}
          >
            {conversation.messages.map((message) =>
              message.role === "user" ? (
                <article key={message.id} className="wb-user-message">
                  <span>
                    {message.actor || "You"} · {message.intent} · revision{" "}
                    {message.scope.revision} · {message.scope.timeline}
                  </span>
                  <p className="wb-prose">{message.text}</p>
                  {message.status&&<small>{message.status==='complete'?'Reply saved':`Reply ${message.status}`}</small>}
                  {message.actor===email&&message.status==='saved'&&<button disabled={busy} onClick={async()=>{setBusy(true);setError('');const request=new AbortController();controller.current=request;try{await streamResult(`${path}/conversations/${thread.id}/messages/${message.id}/generate`,request.signal);await loadThread(thread)}catch(e){setError(e.message)}finally{setBusy(false);controller.current=null}}}>Generate reply to saved message</button>}
                </article>
              ) : (
                <WorldResponse
                  key={message.id}
                  message={message}
                  canEdit={canEdit}
                  archived={canon.world.archived}
                  session={session}
                  onInspect={onInspect}
                  onEdit={onEdit}
                  onPropose={onPropose}
                  onSaveTurn={onSaveTurn}
                  onPremise={onPremise}
                  onChoice={(text) => starter(text, "play")}
                />
              ),
            )}
          </div>
        ))}
        {error && (
          <div role="alert">
            <p>{error}</p>
            <button
              onClick={() =>
                setQuestion(
                  thread.messages.filter((m) => m.role === "user").at(-1)
                    ?.text || "",
                )
              }
            >
              Edit and retry message
            </button>
          </div>
        )}
        {busy && (
          <p role="status">
            {progress || "Consulting this world…"}{" "}
            <button
              onClick={() => {
                controller.current?.abort();
                setError(
                  "Generation stopped in this browser. Provider work may already have used an allowance.",
                );
              }}
            >
              Stop waiting
            </button>
          </p>
        )}
        <div className="wb-context-actions">
          <button onClick={() => setVisuals((v) => !v)} aria-expanded={visuals}>
            Show relationships and timeline
          </button>
          <button onClick={onExplore}>Explore published canon</button>
        </div>
        {visuals && (
          <WorldViews
            records={canon.records}
            total={canon.total}
            calendar={canon.calendar}
            scope={scope}
          />
        )}
        <details>
          <summary>Long-running generation</summary>
          <WorldGenerations
            api={api}
            path={path}
            archived={canon.world.archived}
            request={{
              workflow,
              question,
              revision: appliedScope.revision,
              timeline: appliedScope.timeline,
              ...(appliedScope.at ? { at: appliedScope.at } : {}),
              ...(resolved === "play"
                ? { sessionId: session?.id, expectedVersion: session?.version }
                : {}),
            }}
            onCandidate={candidate}
          />
        </details>
        {clarify && (
          <div role="status">
            <p>Do you want to ask about published canon or prepare a change?</p>
            <button type="button" onClick={() => setIntent("ask")}>
              Ask about this
            </button>
            <button
              type="button"
              disabled={!canEdit}
              onClick={() => setIntent("change")}
            >
              Prepare a draft change
            </button>
          </div>
        )}
      </div>
      <form className="wb-composer" onSubmit={send}>
        {localSaved && (
          <small>
            Saved on this device · AI prompts are never sent automatically
          </small>
        )}
        {localError && <p role="alert">{localError}</p>}
        {shared && (
          <small>
            {resolved === "play" || thread.private
              ? "Private conversation"
              : "Shared world conversation"}
            {thread.saved && (
              <>
                {" "}
                ·{" "}
                <a
                  href={worldHref(scope.worldId, "workspace", {
                    conversation: thread.id,
                    revision: thread.scope.revision,
                    timeline: thread.scope.timeline,
                  })}
                >
                  Link to conversation
                </a>{" "}
                <button
                  type="button"
                  onClick={() =>
                    void loadThread(thread).catch((e) => setError(e.message))
                  }
                >
                  Refresh saved results
                </button>
              </>
            )}
          </small>
        )}
        <div className="wb-chips">
          {references.map((r) => (
            <button
              type="button"
              key={`${r.id}:${r.revision}:${r.timeline}`}
              onClick={() => onRemoveReference(r)}
            >
              Remove context · {r.label || r.id} · r{r.revision} · {r.timeline}
            </button>
          ))}
        </div>
        {thread.scope && !thread.saved && resolved !== "play" && (
          <button
            type="button"
            onClick={() =>
              setThreads((rows) =>
                rows.map((t) =>
                  t.id === selected ? { ...t, scope: undefined } : t,
                ),
              )
            }
          >
            Use displayed world context
          </button>
        )}
        <p className="wb-muted">
          {resolved === "play" && session
            ? `Session state · ${session.title} · turn ${session.version}`
            : "Published context"}{" "}
          · revision {appliedScope.revision} · {appliedScope.timeline}
        </p>
        <label className="wb-composer-input">
          Question or creative direction
          <textarea
            ref={composer}
            placeholder="Ask, create, change or explore your world…"
            value={question}
            maxLength={4000}
            onChange={(e) => {
              setQuestion(e.target.value);
              setClarify(false);
            }}
            onKeyDown={(e) => {
              if ((e.ctrlKey || e.metaKey) && e.key === "Enter")
                e.currentTarget.form.requestSubmit();
            }}
            disabled={busy || canon.world.archived}
          />
        </label>
        <label>
          Intent
          <select
            value={intent}
            onChange={(e) => {
              setIntent(e.target.value);
              setClarify(false);
            }}
            disabled={busy}
          >
            <option value="auto">Auto{resolved ? ` · ${resolved}` : ""}</option>
            <option value="ask">Ask</option>
            <option value="create" disabled={!canEdit}>
              Create
            </option>
            <option value="change" disabled={!canEdit}>
              Change
            </option>
            <option value="play">Play</option>
            <option value="narrative">Draft narrative</option>
            <option value="check">Check consistency</option>
          </select>
        </label>
        <button
          className="wb-primary"
          disabled={
            busy ||
            canon.world.archived ||
            !question.trim() ||
            (["create", "change"].includes(resolved) && !canEdit)
          }
        >
          Send message
        </button>
        <small>
          Ctrl/⌘ + Enter to send. Generated changes always need human review.
        </small>
      </form>
    </section>
  );
}
