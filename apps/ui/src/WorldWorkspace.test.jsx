import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import WorldWorkspace from "./WorldWorkspace.jsx";
import WorldResponse from "./WorldResponse.jsx";
import {
  citationScope,
  conversationQuestion,
  readWorldRoute,
  worldHref,
} from "./worldWorkspace.js";

const scope = { worldId: "world-a", revision: 7, timeline: "main" };
const canon = { world: { role: "editor" }, records: [], total: 0 };
const result = {
  answer: "Aldric is king.",
  baseRevision: 7,
  citations: ["world-a:7:main:aldric"],
  operations: [],
};
function setup(extra = {}) {
  const props = {
    api: vi.fn(async (path) =>
      path.endsWith("/reason") ? result : { enabled: false, items: [] },
    ),
    path: "/world-a",
    scope,
    canon,
    canEdit: true,
    references: [],
    onInspect: vi.fn(),
    onStories: vi.fn(),
    onExplore: vi.fn(),
    onPropose: vi.fn(),
    onEdit: vi.fn(),
    onSaveTurn: vi.fn(),
    onPremise: vi.fn(),
    ...extra,
  };
  const view = render(<WorldWorkspace {...props} />);
  return { ...props, rerender: view.rerender };
}
async function send(text) {
  await act(async () => {
    fireEvent.change(screen.getByLabelText("Question or creative direction"), {
      target: { value: text },
    });
    fireEvent.click(screen.getByRole("button", { name: "Send message" }));
  });
}
describe("AI workspace", () => {
  it("retains fictional time zero when following up in a loaded shared conversation", async () => {
    const saved = {
      id: "zero-thread",
      title: "At the epoch",
      revision: 7,
      timeline: "main",
      at: 0,
    };
    const api = vi.fn(async (path, body) => {
      if (path.split("?")[0].endsWith("/conversations")) return body || [saved];
      if (path.includes("/messages?")) return [];
      return { items: [] };
    });
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(`event: result\ndata: ${JSON.stringify(result)}\n\n`),
      );
    try {
      setup({
        api,
        canon: { ...canon, world: { capabilities: { conversations: true } } },
      });
      await screen.findByRole("option", { name: "At the epoch" });
      fireEvent.change(screen.getByLabelText("Recent conversations"), {
        target: { value: saved.id },
      });
      await waitFor(() =>
        expect(
          api.mock.calls.some(([path]) =>
            path.includes("zero-thread/messages?"),
          ),
        ).toBe(true),
      );
      await send("Who ruled at this time?");
      expect(
        api.mock.calls.find(
          ([path, body]) => path.endsWith("/messages") && body,
        )[1].request.at,
      ).toBe("0");
      expect(
        screen.getByRole("article", { name: "ask response" }),
      ).toHaveTextContent("time 0");
    } finally {
      fetchMock.mockRestore();
    }
  });
  it("starts a distinct source follow-up instead of changing an immutable shared pin", async () => {
    const saved = {
      id: "saved-thread",
      title: "Original task",
      revision: 7,
      timeline: "main",
    };
    const api = vi.fn(async (path, body) => {
      if (path.split("?")[0].endsWith("/conversations")) return body || [saved];
      if (path.includes("/messages?")) return [];
      return { items: [] };
    });
    const props = setup({
      api,
      canon: { ...canon, world: { capabilities: { conversations: true } } },
    });
    await screen.findByRole("option", { name: "Original task" });
    fireEvent.change(screen.getByLabelText("Recent conversations"), {
      target: { value: saved.id },
    });
    await waitFor(() =>
      expect(
        api.mock.calls.some(([path]) =>
          path.includes("saved-thread/messages?"),
        ),
      ).toBe(true),
    );
    props.rerender(
      <WorldWorkspace
        {...props}
        prompt={{
          text: "Tell me about Aldric",
          scope: {
            ...scope,
            revision: 2,
            timeline: "fork",
            at: "9223372036854775807",
          },
        }}
      />,
    );
    await screen.findByRole("option", { name: "Source follow-up" });
    expect(screen.getByLabelText("Recent conversations")).not.toHaveValue(
      saved.id,
    );
    expect(
      screen.getByText(/Shared world conversation · revision 2 · fork/),
    ).toBeVisible();
    fireEvent.change(screen.getByLabelText("Recent conversations"), {
      target: { value: saved.id },
    });
    expect(
      screen.getByText(/Shared world conversation · revision 7 · main/),
    ).toBeVisible();
    expect(api.mock.calls.filter(([, body]) => body)).toHaveLength(0);
  });
  it("keeps a historical source pin for follow-ups until the user changes context", async () => {
    const props = setup({
      prompt: {
        text: "Who is Aldric?",
        intent: "ask",
        scope: { worldId: "world-a", revision: 2, timeline: "fork" },
      },
    });
    await send("Who is Aldric?");
    expect(props.api).toHaveBeenCalledWith(
      "/world-a/reason",
      expect.objectContaining({ revision: 2, timeline: "fork" }),
      expect.any(Object),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Use displayed world context" }),
    );
    await send("What is known now?");
    expect(props.api).toHaveBeenCalledWith(
      "/world-a/reason",
      expect.objectContaining({ revision: 7, timeline: "main" }),
      expect.any(Object),
    );
  });
  it("asks with the applied scope, follows sources and retains conversation during follow-up", async () => {
    const props = setup();
    await send("Who is Aldric?");
    expect(await screen.findByText("Aldric is king.")).toBeVisible();
    expect(props.api).toHaveBeenCalledWith(
      "/world-a/reason",
      expect.objectContaining({
        workflow: "canon_query",
        question: "Who is Aldric?",
        revision: 7,
        timeline: "main",
      }),
      expect.any(Object),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Open source · aldric" }),
    );
    expect(props.onInspect).toHaveBeenCalledWith({ ...scope, id: "aldric" });
    await send("Who ruled before him?");
    await waitFor(() =>
      expect(
        props.api.mock.calls.filter(([p]) => p.endsWith("/reason")),
      ).toHaveLength(2),
    );
    const input = props.api.mock.calls.filter(([p]) =>
      p.endsWith("/reason"),
    )[1][1];
    expect(input.question).toContain("Who is Aldric?");
    expect(input.question).not.toContain("Aldric is king.");
    expect(await screen.findAllByText("Aldric is king.")).toHaveLength(2);
  });
  it("clarifies an ambiguous assertion before preparing any write candidate", async () => {
    const props = setup();
    await send("Aldric survived the battle");
    expect(
      screen.getByText(
        "Do you want to ask about published canon or prepare a change?",
      ),
    ).toBeVisible();
    expect(props.api.mock.calls.some(([p]) => p.endsWith("/reason"))).toBe(
      false,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Prepare a draft change" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Send message" }));
    await waitFor(() =>
      expect(props.api).toHaveBeenCalledWith(
        "/world-a/reason",
        expect.objectContaining({ workflow: "canon_proposal" }),
        expect.any(Object),
      ),
    );
    expect(props.onPropose).not.toHaveBeenCalled();
  });
  it("offers story setup from Play, then pins generated turns to the resumed session", async () => {
    const props = setup();
    await send("Start an adventure in Ashfall");
    expect(props.onStories).toHaveBeenCalledWith(
      "Start an adventure in Ashfall",
    );
    expect(props.api.mock.calls.some(([p]) => p.endsWith("/reason"))).toBe(
      false,
    );
  });
  it("reports failure without inventing a response and exposes an explicit retry", async () => {
    const props = setup({
      api: vi.fn(async (path) => {
        if (path.endsWith("/reason")) throw Error("AI is disabled");
        return { enabled: false, items: [] };
      }),
    });
    await send("Who is Aldric?");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "AI is disabled",
    );
    expect(screen.queryByText("Aldric is king.")).toBeNull();
    fireEvent.click(
      screen.getByRole("button", { name: "Edit and retry message" }),
    );
    expect(screen.getByLabelText("Question or creative direction")).toHaveValue(
      "Who is Aldric?",
    );
    expect(
      props.api.mock.calls.filter(([p]) => p.endsWith("/reason")),
    ).toHaveLength(1);
  });
  it("keeps distinct in-memory conversations and explicitly states persistence limits", async () => {
    setup();
    await send("Who is Aldric?");
    await screen.findByText("Aldric is king.");
    fireEvent.click(screen.getByRole("button", { name: "New conversation" }));
    expect(screen.getByText("Aldric is king.")).not.toBeVisible();
    const select = screen.getByLabelText("Recent conversations");
    fireEvent.change(select, {
      target: {
        value: within(select).getByRole("option", { name: "Who is Aldric?" })
          .value,
      },
    });
    expect(screen.getByText("Aldric is king.")).toBeVisible();
    fireEvent.click(screen.getByText("Conversation context and history"));
    expect(
      screen.getByText(/Conversations last while this world stays open/),
    ).toBeVisible();
  });
  it("cancels browser waiting and ignores a late response", async () => {
    let finish;
    setup({
      api: vi.fn((path) =>
        path.endsWith("/reason")
          ? new Promise((resolve) => {
              finish = resolve;
            })
          : Promise.resolve({ items: [] }),
      ),
    });
    await send("Who is Aldric?");
    fireEvent.click(screen.getByRole("button", { name: "Stop waiting" }));
    await act(async () => finish(result));
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Send message" }),
      ).not.toHaveTextContent("waiting"),
    );
    expect(screen.queryByText("Aldric is king.")).toBeNull();
  });
});
describe("candidate safeguards", () => {
  it("saves inline edits with exact retry identity, and gives changed payloads a new identity", async () => {
    const onPropose = vi
      .fn()
      .mockRejectedValueOnce(Error("Response lost"))
      .mockResolvedValue({ id: "draft-1", local: true });
    render(
      <WorldResponse
        message={{
          intent: "create",
          scope,
          result: {
            ...result,
            workflow: "canon_proposal",
            operations: [
              {
                action: "put",
                record: {
                  id: "hero",
                  kind: "entity",
                  label: "Hero",
                  attributes: { loyalty: "House Veyr" },
                },
              },
            ],
          },
        }}
        canEdit
        onPropose={onPropose}
      />,
    );
    fireEvent.change(screen.getByLabelText("Name", { exact: true }), {
      target: { value: "Older diplomat" },
    });
    fireEvent.change(screen.getByLabelText("Description"), {
      target: { value: "Secretly loyal to Ashfall" },
    });
    fireEvent.change(screen.getByLabelText("Proposal reason"), {
      target: { value: "Introduce a diplomat" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Save AI candidate as draft" }),
    );
    await screen.findByText("Response lost");
    expect(screen.getByLabelText("Name", { exact: true })).toHaveValue(
      "Older diplomat",
    );
    fireEvent.change(screen.getByLabelText("Name", { exact: true }), {
      target: { value: "Mara" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Save AI candidate as draft" }),
    );
    await screen.findByText(/Saved on this device · check device sync status/);
    expect(onPropose.mock.calls[0][2]).not.toBe(onPropose.mock.calls[1][2]);
    expect(onPropose.mock.calls[1][0]).toMatchObject({
      baseRevision: 7,
      operations: [
        {
          record: {
            label: "Mara",
            attributes: {
              description: "Secretly loyal to Ashfall",
              loyalty: "House Veyr",
            },
          },
        },
      ],
    });
    expect(screen.queryByText(/Draft saved/)).toBeNull();
  });
  it("refines the edited artefact as unapproved context without changing canon or the original", async () => {
    const props = setup({
      api: vi.fn(async (path) =>
        path.endsWith("/reason")
          ? {
              ...result,
              operations: [
                {
                  action: "put",
                  record: { id: "hero", kind: "entity", label: "Hero" },
                },
              ],
            }
          : { items: [] },
      ),
    });
    await send("Create a diplomat");
    fireEvent.change(screen.getByLabelText("Name", { exact: true }), {
      target: { value: "Mara" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Refine this draft with AI" }),
    );
    await send("Make her older and loyal to Ashfall");
    const input = props.api.mock.calls.filter(([path]) =>
      path.endsWith("/reason"),
    )[1][1];
    expect(input).toMatchObject({
      workflow: "canon_proposal",
      revision: 7,
      timeline: "main",
    });
    expect(input.question).toContain('"label":"Mara"');
    expect(input.question).toContain("NOT published evidence");
    expect(screen.getAllByLabelText("Name", { exact: true })[0]).toHaveValue(
      "Mara",
    );
    expect(
      screen.getByText("Compare with the draft you refined"),
    ).toBeVisible();
    expect(props.onPropose).not.toHaveBeenCalled();
    expect(() =>
      conversationQuestion("Refine", [], [], {
        operations: [{ text: "世".repeat(1400) }],
      }),
    ).toThrow("4,000-byte");
  });
  it("keeps exact time and a shared result's local edits through refresh without repeating inference", async () => {
    const draft = {
      ...result,
      operations: [
        {
          action: "put",
          record: { id: "hero", kind: "entity", label: "Hero" },
        },
      ],
    };
    let request, messageId;
    const api = vi.fn(async (path, body) => {
      if (path.endsWith("/conversations")) return body || [];
      if (path.includes("/messages?"))
        return [
          {
            id: messageId,
            actor: "author@test",
            request,
            status: "complete",
            result: draft,
          },
        ];
      if (path.endsWith("/messages")) {
        request = body.request;
        messageId = body.id;
        return { id: body.id };
      }
      if (path.includes("/records/"))
        return { record: { id: "aldric", kind: "entity", label: "Aldric" } };
      return { items: [] };
    });
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(`event: result\ndata: ${JSON.stringify(draft)}\n\n`),
      );
    try {
      setup({
        api,
        email: "author@test",
        scope: { ...scope, at: "9223372036854775807" },
        canon: { ...canon, world: { capabilities: { conversations: true } } },
      });
      await send("Create a diplomat");
      fireEvent.change(await screen.findByLabelText("Name", { exact: true }), {
        target: { value: "Mara" },
      });
      fireEvent.click(
        screen.getByRole("button", { name: "Refresh saved results" }),
      );
      await waitFor(() =>
        expect(
          api.mock.calls.some(([path]) => path.includes("/messages?")),
        ).toBe(true),
      );
      expect(screen.getByLabelText("Name", { exact: true })).toHaveValue(
        "Mara",
      );
      expect(
        screen.getByRole("article", { name: "create response" }),
      ).toHaveTextContent("time 9223372036854775807");
      expect(fetchMock).toHaveBeenCalledTimes(1);
    } finally {
      fetchMock.mockRestore();
    }
  });
  it("retains draft edits and saved status when revisiting a conversation", async () => {
    const props = setup({
      api: vi.fn(async (path) =>
        path.endsWith("/reason")
          ? {
              ...result,
              operations: [
                {
                  action: "put",
                  id: "hero",
                  record: { id: "hero", kind: "entity", label: "Hero" },
                },
              ],
            }
          : { items: [] },
      ),
    });
    await send("Create a hero");
    fireEvent.change(screen.getByLabelText("Proposal reason"), {
      target: { value: "Introduce our hero" },
    });
    fireEvent.click(screen.getByRole("button", { name: "New conversation" }));
    const select = screen.getByLabelText("Recent conversations");
    const original = within(select).getByRole("option", {
      name: "Create a hero",
    }).value;
    fireEvent.change(select, { target: { value: original } });
    expect(screen.getByLabelText("Proposal reason")).toHaveValue(
      "Introduce our hero",
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Save AI candidate as draft" }),
    );
    await screen.findByText(/Draft saved. Open Drafts/);
    fireEvent.click(screen.getByRole("button", { name: "New conversation" }));
    fireEvent.change(screen.getByLabelText("Recent conversations"), {
      target: { value: original },
    });
    expect(
      screen.getByRole("button", { name: "Save AI candidate as draft" }),
    ).toBeDisabled();
    expect(props.onPropose).toHaveBeenCalledTimes(1);
  });
  it("saves edits as a draft at the generated revision and reuses the request ID after uncertainty", async () => {
    const onPropose = vi
      .fn()
      .mockRejectedValueOnce(Error("Response lost"))
      .mockResolvedValueOnce({});
    const proposal = {
      ...result,
      workflow: "canon_proposal",
      operations: [
        {
          action: "put",
          id: "hero",
          record: { id: "hero", kind: "entity", label: "Hero" },
        },
      ],
    };
    render(
      <WorldResponse
        message={{ intent: "create", scope, result: proposal }}
        canEdit
        onPropose={onPropose}
      />,
    );
    fireEvent.change(screen.getByLabelText("Proposal reason"), {
      target: { value: "Add hero" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Save AI candidate as draft" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("Response lost");
    fireEvent.click(
      screen.getByRole("button", { name: "Save AI candidate as draft" }),
    );
    expect(await screen.findByRole("status")).toHaveTextContent("Draft saved");
    expect(onPropose.mock.calls[0][2]).toBe(onPropose.mock.calls[1][2]);
    expect(onPropose.mock.calls[0][0].baseRevision).toBe(7);
    expect(screen.queryByRole("button", { name: /publish/i })).toBeNull();
  });
  it("refuses to save a stale or different story candidate", () => {
    render(
      <WorldResponse
        message={{
          intent: "play",
          scope,
          result: {
            ...result,
            workflow: "story_turn",
            sessionId: "s1",
            sessionVersion: 2,
          },
        }}
        session={{ id: "s2", version: 2 }}
      />,
    );
    expect(
      screen.getByRole("button", {
        name: "Save candidate as provisional turn",
      }),
    ).toBeDisabled();
    expect(screen.getByText(/earlier or different story turn/)).toBeVisible();
  });
  it("lets a player edit and explicitly persist narration with the original version", async () => {
    const onSaveTurn = vi.fn(async () => {});
    render(
      <WorldResponse
        message={{
          intent: "play",
          scope,
          result: {
            ...result,
            workflow: "story_turn",
            sessionId: "s1",
            sessionVersion: 2,
          },
        }}
        session={{ id: "s1", version: 2 }}
        onSaveTurn={onSaveTurn}
      />,
    );
    fireEvent.change(screen.getByLabelText("Edit draft narration"), {
      target: { value: "The door opens." },
    });
    fireEvent.click(
      screen.getByRole("button", {
        name: "Save candidate as provisional turn",
      }),
    );
    await screen.findByText(/Turn saved as private session state/);
    expect(onSaveTurn).toHaveBeenCalledWith(
      expect.objectContaining({
        answer: "The door opens.",
        sessionVersion: 2,
        sessionId: "s1",
      }),
      expect.any(String),
    );
  });
  it("rejects foreign-world citations and preserves route and byte boundaries", () => {
    expect(citationScope("world-b:7:main:secret", "world-a")).toBeNull();
    const route = readWorldRoute(
      worldHref("a/b", "explore", {
        record: "hero",
        revision: 7,
        timeline: "main",
        at: "-9223372036854775808",
      }),
    );
    expect(route.worldId).toBe("a/b");
    expect(route.params.get("at")).toBe("-9223372036854775808");
    expect(() => conversationQuestion("世".repeat(1400), [], [])).toThrow(
      "4,000-byte",
    );
  });
});
