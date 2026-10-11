import { render, screen, fireEvent } from "@testing-library/react";
import { it, expect, vi } from "vitest";
import WorldConnections from "./WorldConnections.jsx";
it("traverses explicit connections without losing revision, timeline or time", async () => {
  const api = vi.fn(async () => ({
      records: [{ id: "edge", kind: "relationship", label: "Friendship" }],
      total: 1,
    })),
    explore = vi.fn();
  render(
    <WorldConnections
      api={api}
      path="/world"
      scope={{ id: "hero", revision: 2, timeline: "branch", at: "30" }}
      onExplore={explore}
      onClose={vi.fn()}
    />,
  );
  fireEvent.click(
    await screen.findByRole("button", { name: "Friendship · relationship" }),
  );
  expect(api.mock.calls[0][0]).toContain("revision=2&timeline=branch");
  expect(api.mock.calls[0][0]).toContain("at=30");
  expect(explore).toHaveBeenCalledWith({
    id: "edge",
    revision: 2,
    timeline: "branch",
    at: "30",
  });
});
