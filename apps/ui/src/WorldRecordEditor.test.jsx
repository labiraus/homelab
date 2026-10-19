import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import WorldRecordEditor from "./WorldRecordEditor.jsx";

const type = {
  id: "person",
  kind: "type",
  label: "Person",
  fields: { age: "number", alive: "boolean" },
  relations: { friend: "person" },
  attributes: { description: "People", notes: "" },
};
const records = [
  type,
  { id: "hero", kind: "entity", type: "person", label: "Hero" },
  { id: "source", kind: "source", label: "Notes" },
];

describe("structured canon editing", () => {
  it("keeps original editor scope when current canon advances", () => {
    const onStage = vi.fn();
    const props = {
      records,
      record: records[2],
      timeline: "main",
      revision: 4,
      onStage,
      onNew: () => {},
    };
    const { rerender } = render(<WorldRecordEditor {...props} />);
    fireEvent.change(screen.getByLabelText("Label", { exact: true }), {
      target: { value: "Saved original writing" },
    });
    rerender(<WorldRecordEditor {...props} revision={5} />);
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "revision 4, timeline main",
    );
    rerender(<WorldRecordEditor {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenCalledWith(
      expect.objectContaining({ label: "Saved original writing" }),
    );
  });
  it("retains legacy policy fields without permitting canonical policy edits", () => {
    const onStage = vi.fn();
    const record = {
      id: "world_settings",
      kind: "world_settings",
      label: "World",
      timeline: "main",
      attributes: { tone: "calm" },
      reviewPolicy: { requiredApprovals: 2 },
    };
    render(
      <WorldRecordEditor
        record={record}
        records={[{ id: "main", kind: "timeline", label: "Main" }]}
        timeline="main"
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    expect(
      screen.queryByLabelText("Required independent approvals"),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenCalledWith(expect.objectContaining(record));
  });
  it("stages reviewed calendar settings while retaining other complete-record fields", () => {
    const onStage = vi.fn();
    render(
      <WorldRecordEditor
        records={[{ id: "main", kind: "timeline", label: "Main" }]}
        timeline="main"
        record={{
          id: "world_settings",
          kind: "world_settings",
          label: "World",
          timeline: "main",
          attributes: { tone: "gentle" },
        }}
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    fireEvent.click(screen.getByLabelText("Use a fictional calendar"));
    fireEvent.change(screen.getByLabelText("Epoch tick"), {
      target: { value: "-9223372036854775808" },
    });
    fireEvent.change(screen.getByLabelText("Calendar name"), {
      target: { value: "Tides" },
    });
    fireEvent.change(screen.getByLabelText("Years per cycle"), {
      target: { value: "4" },
    });
    fireEvent.change(
      screen.getByLabelText(
        "Leap year positions (comma separated, starting at 1)",
      ),
      { target: { value: "4" } },
    );
    fireEvent.change(screen.getByLabelText("Extra days in leap years 1"), {
      target: { value: "1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenLastCalledWith(
      expect.objectContaining({
        attributes: { tone: "gentle" },
        timeline: "main",
        calendar: expect.objectContaining({
          name: "Tides",
          epochTick: "-9223372036854775808",
          cycleYears: 4,
          leapYears: [4],
        }),
      }),
    );
    fireEvent.click(screen.getByLabelText("Use a fictional calendar"));
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage.mock.lastCall[0].calendar).toBeUndefined();
  });
  it("keeps typed date facts as canonical strings across the entire signed range", () => {
    const onStage = vi.fn();
    render(
      <WorldRecordEditor
        records={[...records, { ...type, fields: { birth: "tick" } }]}
        timeline="main"
        record={{
          id: "born",
          kind: "fact",
          subject: "hero",
          predicate: "birth",
          valueType: "tick",
          value: "-9223372036854775808",
          source: "source",
          timeline: "main",
        }}
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage.mock.lastCall[0].value).toBe("-9223372036854775808");
    fireEvent.change(screen.getByLabelText("Value"), {
      target: { value: "9223372036854775807" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage.mock.lastCall[0].value).toBe("9223372036854775807");
    fireEvent.change(screen.getByLabelText("Value"), {
      target: { value: "9223372036854775808" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(screen.getByRole("alert")).toHaveTextContent("64-bit");
    expect(onStage).toHaveBeenCalledTimes(2);
  });
  it("stages and re-edits exact signed boundaries without Number conversion", () => {
    const onStage = vi.fn();
    const record = {
      id: "epoch",
      kind: "event",
      label: "Epoch",
      timeline: "main",
      source: "source",
      from: "-9223372036854775808",
      until: "9223372036854775807",
    };
    render(
      <WorldRecordEditor
        records={records}
        timeline="main"
        record={record}
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    expect(screen.getByLabelText("Valid from")).toHaveValue(record.from);
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenLastCalledWith(expect.objectContaining(record));
    fireEvent.change(screen.getByLabelText("Valid from"), {
      target: { value: "9007199254740993" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenLastCalledWith(
      expect.objectContaining({ from: "9007199254740993" }),
    );
    fireEvent.change(screen.getByLabelText("Valid until"), {
      target: { value: "9223372036854775808" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(screen.getByRole("alert")).toHaveTextContent("signed 64-bit range");
    expect(onStage).toHaveBeenCalledTimes(2);
  });
  it("preserves the complete schema, relationships and attributes when editing one field", () => {
    const onStage = vi.fn();
    render(
      <WorldRecordEditor
        records={records}
        timeline="main"
        record={type}
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    fireEvent.change(screen.getByLabelText("Fields name 1"), {
      target: { value: "years" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenCalledWith({
      ...type,
      fields: { years: "number", alive: "boolean" },
    });
    expect(screen.getByLabelText("Stable ID")).toHaveAttribute("readonly");
    expect(screen.getByLabelText("Record kind")).toBeDisabled();
  });
  it("supports multiple schema fields and self-referencing relationship declarations", () => {
    const onStage = vi.fn();
    render(
      <WorldRecordEditor
        records={[]}
        timeline="main"
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    fireEvent.change(screen.getByLabelText("Record kind"), {
      target: { value: "type" },
    });
    fireEvent.change(screen.getByLabelText("Stable ID"), {
      target: { value: "person" },
    });
    for (const [index, name, value] of [
      [1, "age", "number"],
      [2, "alive", "boolean"],
    ]) {
      fireEvent.click(screen.getByRole("button", { name: "Add fields" }));
      fireEvent.change(screen.getByLabelText(`Fields name ${index}`), {
        target: { value: name },
      });
      fireEvent.change(screen.getByLabelText(`Fields value ${index}`), {
        target: { value },
      });
    }
    fireEvent.click(screen.getByRole("button", { name: "Add relationships" }));
    fireEvent.change(screen.getByLabelText("Relationships name 1"), {
      target: { value: "friend" },
    });
    fireEvent.change(screen.getByLabelText("Relationships value 1"), {
      target: { value: "person" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenCalledWith(
      expect.objectContaining({
        fields: type.fields,
        relations: type.relations,
      }),
    );
  });
  it("rejects duplicate schema fields before they could overwrite one another", () => {
    const onStage = vi.fn();
    render(
      <WorldRecordEditor
        records={records}
        timeline="main"
        record={type}
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    fireEvent.change(screen.getByLabelText("Fields name 2"), {
      target: { value: "age" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(screen.getByRole("alert")).toHaveTextContent("duplicate name age");
    expect(onStage).not.toHaveBeenCalled();
  });
  it("preserves false fact values and their original timeline and validity", () => {
    const onStage = vi.fn();
    const fact = {
      id: "life",
      kind: "fact",
      subject: "hero",
      predicate: "alive",
      valueType: "boolean",
      value: false,
      timeline: "alternate",
      source: "source",
      from: -10,
      until: 20,
    };
    render(
      <WorldRecordEditor
        records={records}
        timeline="main"
        record={fact}
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenCalledWith({ ...fact, attributes: {} });
  });
  it("edits deterministic life rules while retaining other attributes", () => {
    const onStage = vi.fn();
    const rule = {
      id: "life",
      kind: "rule",
      label: "Chronology",
      attributes: {
        check: "life_bounds",
        birth: "born",
        death: "died",
        explanation: "Fictional ticks",
      },
    };
    render(
      <WorldRecordEditor
        records={records}
        timeline="main"
        record={rule}
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    fireEvent.change(screen.getByLabelText("Death predicate"), {
      target: { value: "death" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenCalledWith({
      ...rule,
      attributes: { ...rule.attributes, death: "death" },
    });
  });
  it("preserves and edits a scoped relationship cardinality rule", () => {
    const onStage = vi.fn();
    const rule = {
      id: "friends",
      kind: "rule",
      label: "One friend",
      type: "person",
      timeline: "main",
      predicate: "friend",
      from: 0,
      until: 20,
      attributes: { check: "relationship_cardinality", max: "1" },
    };
    render(
      <WorldRecordEditor
        records={[...records, { id: "main", kind: "timeline", label: "Main" }]}
        timeline="main"
        record={rule}
        onStage={onStage}
        onNew={() => {}}
      />,
    );
    fireEvent.change(
      screen.getByLabelText("Maximum simultaneous relationships"),
      { target: { value: "2" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Stage record" }));
    expect(onStage).toHaveBeenCalledWith({
      ...rule,
      attributes: { check: "relationship_cardinality", max: "2" },
    });
  });
});
