import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, describe, it, expect, vi } from "vitest";
import Worldbuilder from "./Worldbuilder.jsx";
const world = { id: "w", name: "Novel", revision: 1, role: "editor" };
function mockApi(
  change = {
    status: "draft",
    author: "editor@test",
    id: "c",
    reason: "Add hero",
    baseRevision: 1,
    operations: [],
  },
) {
  const calls = [];
  global.fetch = vi.fn(async (url, options = {}) => {
    calls.push([url, options]);
    let data = [];
    if (url === "/api/worlds") data = [world];
    else if (url.includes("/canon?") || url.includes("/search?"))
      data = {
        world,
        revision: 1,
        total: 1,
        records: [{ id: "main", kind: "timeline", label: "Main" }],
      };
    else if (url.endsWith("/changes")) data = [change];
    else if (url.endsWith("/reason"))
      data = {
        answer: "A partial answer",
        unknown: false,
        citations: [],
        operations: [],
        baseRevision: 1,
        provenance: {
          requestId: "generation-1",
          latencyMs: 25,
          allocatedMicros: 100,
          currency: "TST",
        },
        evidenceScope: {
          coverage: "selected_records",
          evidenceCount: 25,
          canonRecordCount: 200,
        },
      };
    else if (url.endsWith("/submit")) {
      change.status = "ready_for_review";
      data = change;
    }
    return { ok: true, text: async () => JSON.stringify(data) };
  });
  return calls;
}
afterEach(() => {
  vi.restoreAllMocks();
});
describe("Worldbuilder", () => {
  it("allows an editor with superuser status to review their own proposal", async () => {
    world.superuser = true;
    try {
      const calls = mockApi({ id: "c", reason: "Own proposal", status: "ready_for_review", author: "editor@test", operations: [], reviews: [] });
      render(<Worldbuilder email="editor@test" />);
      fireEvent.change(await screen.findByLabelText("Open world"), { target: { value: "w" } });
      fireEvent.change(await screen.findByLabelText("Review reason"), { target: { value: "Explicit self review" } });
      fireEvent.click(await screen.findByRole("button", { name: "Approve" }));
      await waitFor(() => expect(calls.some(([url]) => url.endsWith("/changes/c/review"))).toBe(true));
    } finally { delete world.superuser; }
  });

  it("displays proposal quorum, prevents duplicate approvals and blocks publication after revocation", async () => {
    world.role = "reviewer";
    const change = {
      id: "c",
      status: "approved",
      author: "other@test",
      reason: "Quorum proposal",
      baseRevision: 1,
      operations: [],
      requiredApprovals: 2,
      approvals: ["editor@test"],
      reviews: [
        {
          reviewer: "editor@test",
          decision: "approved",
          reason: "Checked",
          eligible: true,
          createdAt: "2026-01-01T00:00:00Z",
        },
        {
          reviewer: "revoked@test",
          decision: "approved",
          reason: "Old grant",
          eligible: false,
          createdAt: "2026-01-01T00:00:00Z",
        },
      ],
    };
    try {
      mockApi(change);
      render(<Worldbuilder email="editor@test" />);
      await screen.findByRole("option", { name: "Novel" });
      fireEvent.change(screen.getByLabelText("Open world"), {
        target: { value: "w" },
      });
      await screen.findByText("Quorum proposal");
      expect(
        screen.getByText(/Currently eligible approvals: 1 \/ 2/),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Publish approved change" }),
      ).toBeDisabled();
      expect(
        screen.queryByRole("button", { name: "Approve", exact: true }),
      ).toBeNull();
      expect(
        screen.getByRole("list", { name: "Reviews for Quorum proposal" }),
      ).toHaveTextContent("Membership authority changed");
      change.reviews[0].eligible = false;
      change.approvals = [];
      fireEvent.click(screen.getByRole("button", { name: "Load canon" }));
      await screen.findByText(/Currently eligible approvals: 0 \/ 2/);
      expect(
        screen.getByRole("button", { name: "Approve", exact: true }),
      ).toBeInTheDocument();
    } finally {
      world.role = "editor";
    }
  });
  it("keeps unloaded record navigation at the submitted revision, timeline and exact time", async () => {
    const currentWorld = { ...world, revision: 3 };
    const calls = [];
    global.fetch = vi.fn(async (url) => {
      calls.push(new URL(url, "https://test.local"));
      const parsed = calls.at(-1);
      let data = [];
      if (parsed.pathname === "/api/worlds") data = [currentWorld];
      if (parsed.pathname.endsWith("/canon"))
        data = {
          world: currentWorld,
          revision: Number(parsed.searchParams.get("revision") || 3),
          timeline: "main",
          total: 1,
          records: parsed.searchParams.has("kind")
            ? []
            : [
                {
                  id: "edge",
                  kind: "relationship",
                  subject: "hidden",
                  target: "other",
                  predicate: "friend",
                  timeline: "main",
                },
              ],
        };
      if (parsed.pathname.endsWith("/records/hidden"))
        data = {
          revision: 2,
          record: { id: "hidden", kind: "entity", label: "Hidden" },
        };
      return { ok: true, text: async () => JSON.stringify(data) };
    });
    render(<Worldbuilder email="editor@test" />);
    await screen.findByRole("option", { name: "Novel" });
    fireEvent.change(screen.getByLabelText("Open world"), {
      target: { value: "w" },
    });
    await screen.findByRole("heading", { name: "edge relationship" });
    fireEvent.change(screen.getByLabelText("Historical revision"), {
      target: { value: "2" },
    });
    fireEvent.change(screen.getByLabelText("Fictional time"), {
      target: { value: "-9223372036854775808" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Load canon" }));
    await screen.findByText(/Showing 1 of 1 records at revision 2/);
    fireEvent.change(screen.getByLabelText("Historical revision"), {
      target: { value: "3" },
    });
    fireEvent.change(screen.getByLabelText("Timeline", { exact: true }), {
      target: { value: "unsubmitted" },
    });
    fireEvent.change(screen.getByLabelText("Fictional time"), {
      target: { value: "0" },
    });
    expect(
      screen.getByRole("region", { name: "Canon visualisations" }),
    ).toHaveTextContent(
      "Revision 2, timeline main, fictional time -9223372036854775808",
    );
    const article = screen
      .getByRole("heading", { name: "edge relationship" })
      .closest("article");
    fireEvent.click(within(article).getByRole("link", { name: "hidden" }));
    await screen.findByRole("complementary", { name: "Linked canon record" });
    const linked = calls.find((url) =>
      url.pathname.endsWith("/records/hidden"),
    );
    expect(Object.fromEntries(linked.searchParams)).toEqual({
      revision: "2",
      timeline: "main",
      at: "-9223372036854775808",
    });
  });
  it("uses scoped search and identifies limited AI coverage", async () => {
    const calls = mockApi();
    render(<Worldbuilder email="editor@test" />);
    await screen.findByRole("option", { name: "Novel" });
    fireEvent.change(screen.getByLabelText("Open world"), {
      target: { value: "w" },
    });
    await screen.findByText("Novel · canon revision 1");
    fireEvent.change(screen.getByLabelText("Search canon"), {
      target: { value: "Hero" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Load canon" }));
    await waitFor(() =>
      expect(
        calls.some(
          ([url]) =>
            url.includes("/search?") &&
            url.includes("q=Hero") &&
            url.includes("timeline=main"),
        ),
      ).toBe(true),
    );
    fireEvent.change(screen.getByLabelText("Question or creative direction"), {
      target: { value: "Who is Hero?" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Generate candidate" }));
    expect(
      await screen.findByText(/25 selected canon records out of 200/),
    ).toBeVisible();
    fireEvent.click(screen.getByText("Generation details"));
    expect(screen.getByText(/Request: generation-1/)).toBeVisible();
    expect(screen.getByText(/0.000100 TST/)).toBeVisible();
  });
  it("shows unavailable activation without fabricating worlds", async () => {
    global.fetch = vi.fn(async () => ({
      ok: false,
      status: 404,
      text: async () => "{}",
    }));
    render(<Worldbuilder email="editor@test" />);
    expect(await screen.findByRole("alert")).toHaveTextContent("not enabled");
  });
  it("loads canon and submits draft without publishing", async () => {
    const calls = mockApi();
    render(<Worldbuilder email="editor@test" />);
    await screen.findByRole("option", { name: "Novel" });
    fireEvent.change(screen.getByLabelText("Open world"), {
      target: { value: "w" },
    });
    await screen.findByText("Novel · canon revision 1");
    fireEvent.click(screen.getByRole("button", { name: "Submit for review" }));
    await waitFor(() =>
      expect(calls.some(([u]) => u.endsWith("/submit"))).toBe(true),
    );
    expect(calls.some(([u]) => u.endsWith("/publish"))).toBe(false);
    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull();
  });
  it("does not expose self-approval for an owner", async () => {
    world.role = "owner";
    mockApi({
      status: "ready_for_review",
      author: "editor@test",
      id: "c",
      reason: "Own proposal",
      baseRevision: 1,
      operations: [],
    });
    render(<Worldbuilder email="editor@test" />);
    await screen.findByRole("option", { name: "Novel" });
    fireEvent.change(screen.getByLabelText("Open world"), {
      target: { value: "w" },
    });
    await screen.findByText("Own proposal");
    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull();
    world.role = "editor";
  });
  it("keeps the original base revision when canon advances after staging", async () => {
    const calls = mockApi();
    render(<Worldbuilder email="editor@test" />);
    await screen.findByRole("option", { name: "Novel" });
    fireEvent.change(screen.getByLabelText("Open world"), {
      target: { value: "w" },
    });
    fireEvent.click(
      await screen.findByRole("button", { name: "Propose removal" }),
    );
    world.revision = 2;
    try {
      fireEvent.click(screen.getByRole("button", { name: "Load canon" }));
      await screen.findByText("Novel · canon revision 2");
      fireEvent.change(screen.getByLabelText("Reason for change"), {
        target: { value: "Remove timeline" },
      });
      fireEvent.click(
        screen.getByRole("button", { name: "Save draft proposal" }),
      );
      await waitFor(() =>
        expect(
          calls.some(
            ([url, options]) =>
              url.endsWith("/changes") && options.method === "POST",
          ),
        ).toBe(true),
      );
      const [, options] = calls.find(
        ([url, options]) =>
          url.endsWith("/changes") && options.method === "POST",
      );
      expect(JSON.parse(options.body).baseRevision).toBe(1);
    } finally {
      world.revision = 1;
    }
  });
  it("pages canon at the inspected revision and loads editor references independently", async () => {
    const novel = { ...world, revision: 4 };
    const references = [
      {
        id: "person",
        kind: "type",
        label: "Person",
        fields: { age: "number" },
      },
      { id: "source", kind: "source", label: "Notes" },
    ];
    const calls = [];
    global.fetch = vi.fn(async (url) => {
      calls.push(url);
      const parsed = new URL(url, "https://test.local");
      let data = [];
      if (parsed.pathname === "/api/worlds") data = [novel];
      if (parsed.pathname.endsWith("/canon")) {
        const catalog = parsed.searchParams.has("kind");
        const offset = Number(parsed.searchParams.get("offset") || 0);
        data = {
          world: novel,
          revision: 4,
          timeline: "main",
          total: 2,
          records: catalog
            ? references
            : offset
              ? [
                  {
                    id: "friend",
                    kind: "entity",
                    type: "person",
                    label: "Friend",
                  },
                ]
              : [{ id: "hero", kind: "entity", type: "person", label: "Hero" }],
        };
      }
      return { ok: true, text: async () => JSON.stringify(data) };
    });
    render(<Worldbuilder email="editor@test" />);
    await screen.findByRole("option", { name: "Novel" });
    fireEvent.change(screen.getByLabelText("Open world"), {
      target: { value: "w" },
    });
    await screen.findByRole("option", { name: "Person" });
    fireEvent.change(screen.getByLabelText("Search canon"), {
      target: { value: "Unsubmitted search" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Load more canon records" }),
    );
    await screen.findByRole("button", { name: "Edit Friend" });
    const page = calls
      .map((url) => new URL(url, "https://test.local"))
      .find((url) => url.searchParams.get("offset") === "1");
    expect(page.searchParams.get("revision")).toBe("4");
    expect(page.searchParams.has("q")).toBe(false);
    expect(
      screen.queryByRole("button", { name: "Load more canon records" }),
    ).toBeNull();
  });
});
