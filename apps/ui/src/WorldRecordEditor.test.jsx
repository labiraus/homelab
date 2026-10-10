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
});
