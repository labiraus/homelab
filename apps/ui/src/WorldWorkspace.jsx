import { createPortal } from "react-dom";
import { useEffect, useRef, useState } from "react";
import WorldResponse from "./WorldResponse.jsx";
import { RecordSummary } from "./WorldResponse.jsx";
import WorldGenerations from "./WorldGenerations.jsx";
import WorldViews from "./WorldViews.jsx";
import {
  conversationQuestion,
  detectIntent,
  workflows,
} from "./worldWorkspace.js";

export default function WorldWorkspace({
  api,
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
  const [visuals, setVisuals] = useState(false);
  const controller = useRef(null),
    composer = useRef(null),
    log = useRef(null);
  const follow = useRef(true);
  const thread = threads.find((t) => t.id === selected);
  const resolved = intent === "auto" ? detectIntent(question) : intent;
  const workflow = workflows[resolved] || "canon_query";
  const appliedScope =
    resolved === "play" && session
      ? {
          ...scope,
          revision: session.pinnedRevision,
          timeline: session.timeline,
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
          thread.messages,
          references,
        ),
        revision: appliedScope.revision,
        timeline: appliedScope.timeline,
        ...(resolved === "play"
          ? { sessionId: session.id, expectedVersion: session.version }
          : {}),
      };
    } catch (e) {
      setError(e.message);
      return;
    }
    follow.current = true;
    const threadId = selected;
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
      const result = await api(`${path}/reason`, input, {
        signal: request.signal,
      });
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
            onConversation?.();
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
            Conversations last while this world stays open. Stories and saved
            proposals persist. AI answers use the last three requests and
            attached references as context; earlier answers are not canon.
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
                    You · {message.intent} · revision {message.scope.revision} ·{" "}
                    {message.scope.timeline}
                  </span>
                  <p className="wb-prose">{message.text}</p>
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
            Consulting this world…{" "}
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
        {thread.scope && resolved !== "play" && (
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
