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
  render(<WorldWorkspace {...props} />);
  return props;
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
