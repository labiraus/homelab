import { render, screen, fireEvent, within } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import WorldViews from "./WorldViews.jsx";

const scope = { revision: 4, timeline: "main", at: "", query: "" };
const entity = (id) => ({ id, kind: "entity", label: id.toUpperCase() });
const edge = (id, subject, target, extra = {}) => ({
  id,
  kind: "relationship",
  subject,
  target,
  predicate: "friend",
  source: "notes",
  timeline: "main",
  ...extra,
});
const event = (id, from, extra = {}) => ({
  id,
  kind: "event",
  label: id,
  from,
  source: "notes",
  timeline: "main",
  ...extra,
});
function open(name) {
  fireEvent.click(screen.getByText(name, { selector: "summary" }));
}

describe("scoped canon views", () => {
  it("shows directed/self/reverse edges, sources and full-scope links with entity focus", () => {
    const records = [
      entity("a"),
      entity("b"),
      entity("c"),
      edge("ab", "a", "b"),
      edge("ba", "b", "a"),
      edge("bb", "b", "b"),
      edge("ac", "a", "c"),
    ];
    render(
      <WorldViews records={records} total={records.length} scope={scope} />,
    );
    open("Relationship graph");
    const graph = screen.getByRole("region", { name: "Relationship graph" });
    expect(
      within(graph).getByRole("link", { name: "Inspect friend: A to B" }),
    ).toHaveAttribute("href", "#canon-ab");
    expect(
      within(graph).getByRole("link", { name: "Inspect friend: B to A" }),
    ).toHaveAttribute("href", "#canon-ba");
    expect(
      within(graph).getByRole("link", { name: "Inspect friend: B to B" }),
    ).toHaveAttribute("href", "#canon-bb");
    fireEvent.change(screen.getByLabelText("Focus on entity"), {
      target: { value: "b" },
    });
    expect(within(graph).getAllByRole("row")).toHaveLength(4);
    expect(
      within(graph).queryByRole("link", { name: "Inspect friend: A to C" }),
    ).toBeNull();
    expect(
      within(graph).getAllByRole("link", { name: "notes" })[0],
    ).toHaveAttribute("href", "#canon-notes");
  });
  it("orders negative and adjacent unsafe ticks exactly with deterministic ties", () => {
    const records = [
      event("last", "9223372036854775807"),
      event("z-tie", "9007199254740993"),
      event("later", "9007199254740994"),
      event("a-tie", "9007199254740993"),
      event("B-tie", "9007199254740993"),
      event("first", "-9223372036854775808"),
      event("zero", 0),
    ];
    render(
      <WorldViews
        records={records}
        total={8}
        scope={{ ...scope, query: "epoch" }}
      />,
    );
    open("Chronology");
    const chronology = screen.getByRole("region", { name: "Chronology" });
    expect(
      within(chronology)
        .getAllByRole("listitem")
        .map((row) => within(row).getAllByRole("link")[0].textContent),
    ).toEqual(["first", "zero", "B-tie", "a-tie", "z-tie", "later", "last"]);
    expect(
      screen.getByRole("region", { name: "Canon visualisations" }),
    ).toHaveTextContent(
      "Views cover 7 of 8 matching canon records. Load more canon records",
    );
    expect(
      screen.getByRole("region", { name: "Canon visualisations" }),
    ).toHaveTextContent("Search: epoch");
  });
  it("honours half-open scope at tick zero and signed extremes without timeline leakage", () => {
    const records = [
      event("expired", -2, { until: 0 }),
      event("current", 0, { until: 1 }),
      event("future", 1),
      event("other", 0, { timeline: "alternate" }),
      edge("hidden-edge", "a", "b", { timeline: "alternate" }),
    ];
    const { rerender } = render(
      <WorldViews records={records} total={5} scope={{ ...scope, at: "000" }} />,
    );
    open("Chronology");
    open("Relationship graph");
    const chronology = screen.getByRole("region", { name: "Chronology" });
    expect(within(chronology).getAllByRole("listitem")).toHaveLength(1);
    expect(
      within(chronology).getByRole("link", { name: "current" }),
    ).toBeVisible();
    expect(
      screen.getByText("No relationships in the loaded scope."),
    ).toBeVisible();
    const extreme = [
      event("boundary", "9223372036854775806", {
        until: "9223372036854775807",
      }),
    ];
    rerender(
      <WorldViews
        records={extreme}
        total={1}
        scope={{ ...scope, at: "9223372036854775807" }}
      />,
    );
    expect(
      within(chronology).getByText("No events in the loaded scope."),
    ).toBeVisible();
  });
  it("bounds the diagram while retaining all rows and unloaded reference navigation", () => {
    const records = Array.from({ length: 60 }, (_, i) =>
      edge(`edge-${i}`, "hub", `node-${i}`),
    );
    render(<WorldViews records={records} total={100} scope={scope} />);
    open("Relationship graph");
    const graph = screen.getByRole("region", { name: "Relationship graph" });
    expect(graph.querySelectorAll("svg circle")).toHaveLength(24);
    expect(within(graph).getAllByRole("row")).toHaveLength(61);
    expect(
      within(graph).getByText(
        "Diagram shows 23 of 60 relationships. The table lists all 60 in this focus.",
      ),
    ).toBeVisible();
    fireEvent.change(screen.getByLabelText("Focus on entity"), {
      target: { value: "node-59" },
    });
    expect(
      within(graph).getByRole("link", {
        name: "Inspect friend: hub to node-59",
      }),
    ).toHaveAttribute("href", "#canon-edge-59");
    expect(
      within(graph).getByRole("link", { name: "Open node-59" }),
    ).toHaveAttribute("href", "#canon-node-59");
  });
});
