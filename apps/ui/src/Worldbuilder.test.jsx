import { render, screen, fireEvent, waitFor } from "@testing-library/react";
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
    else if (url.includes("/canon?"))
      data = {
        world,
        revision: 1,
        total: 1,
        records: [{ id: "main", kind: "timeline", label: "Main" }],
      };
    else if (url.endsWith("/changes")) data = [change];
    else if (url.endsWith("/submit")) {
      change.status = "ready_for_review";
      data = change;
    }
    return { ok: true, json: async () => data };
  });
  return calls;
}
afterEach(() => {
  vi.restoreAllMocks();
});
describe("Worldbuilder", () => {
  it("shows unavailable activation without fabricating worlds", async () => {
    global.fetch = vi.fn(async () => ({
      ok: false,
      status: 404,
      json: async () => ({}),
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
      return { ok: true, json: async () => data };
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
