import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import WorldConfiguration from "./WorldConfiguration.jsx";
import WorldEntityPicker from "./WorldEntityPicker.jsx";
import WorldStoryScene from "./WorldStoryScene.jsx";
import { useState } from "react";

const settings = {
  id: "settings",
  kind: "world_settings",
  label: "World",
  timeline: "main",
  attributes: { description: "Old description", retained: "keep" },
  reviewPolicy: { requiredApprovals: 2 },
  calendar: { name: "Moon calendar" },
};
describe("Frontend corrections", () => {
  it("stages configuration at the edit's original revision and preserves unrelated canon fields", () => {
    const onStage = vi.fn();
    const props = {
      record: settings,
      revision: 7,
      timelines: [{ id: "main", label: "Main" }],
      canEdit: true,
      onStage,
    };
    const view = render(<WorldConfiguration {...props} />);
    fireEvent.click(
      screen.getByRole("button", { name: "Edit name and description" }),
    );
    fireEvent.change(screen.getByLabelText("World description"), {
      target: { value: "New description" },
    });
    view.rerender(<WorldConfiguration {...props} revision={8} />);
    fireEvent.click(
      screen.getByRole("button", { name: "Stage configuration draft" }),
    );
    expect(onStage).toHaveBeenCalledWith(
      {
        ...settings,
        attributes: { ...settings.attributes, description: "New description" },
      },
      7,
    );
    expect(
      screen.getByText(
        /Reviewed guidance shapes the world/,
      ),
    ).toBeVisible();
  });
  it("selects named entities from the exact story pin and clears selections when that pin changes", async () => {
    const api = vi.fn(async () => ({
      records: [
        {
          id: "raw-character-id",
          kind: "entity",
          label: "Mira",
          type: "person",
        },
      ],
      total: 1,
    }));
    function Picker({ revision }) {
      const [value, onChange] = useState([]);
      return (
        <WorldEntityPicker
          api={api}
          path="/w"
          revision={revision}
          timeline="old-branch"
          value={value}
          onChange={onChange}
        />
      );
    }
    const view = render(<Picker revision={2} />);
    fireEvent.click(await screen.findByRole("checkbox", { name: /Mira/ }));
    expect(screen.getByRole("button", { name: "Remove Mira" })).toBeVisible();
    expect(api).toHaveBeenCalledWith(
      expect.stringContaining("revision=2&timeline=old-branch"),
    );
    expect(screen.queryByText("raw-character-id")).toBeNull();
    view.rerender(<Picker revision={3} />);
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Remove Mira" })).toBeNull(),
    );
  });
  it("shows unavailable character lookup explicitly instead of inventing choices", async () => {
    render(
      <WorldEntityPicker
        api={vi.fn(async () => {
          throw Error("Canon unavailable");
        })}
        path="/w"
        revision={2}
        timeline="main"
        value={[]}
        onChange={vi.fn()}
      />,
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Canon unavailable",
    );
    expect(screen.queryByRole("checkbox")).toBeNull();
  });
  it("shows saved narration and sends a player action at the inspected session version without automatically saving", async () => {
    const api = vi.fn(async () => ({
      answer: "A new scene",
      baseRevision: 2,
      sessionVersion: 4,
      operations: [],
      citations: [],
    }));
    const onSaveTurn = vi.fn();
    render(
      <WorldStoryScene
        api={api}
        path="/w"
        worldId="w"
        session={{
          id: "session",
          title: "Journey",
          pinnedRevision: 2,
          timeline: "old",
          version: 4,
          state: {},
          memory: {},
        }}
        turns={[{ version: 4, narration: "The saved scene" }]}
        onSaveTurn={onSaveTurn}
      />,
    );
    expect(screen.getByText("The saved scene")).toBeVisible();
    fireEvent.change(screen.getByLabelText("Your action"), {
      target: { value: "Open the gate" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Continue scene" }));
    expect((await screen.findAllByText("A new scene"))[0]).toBeVisible();
    expect(api).toHaveBeenCalledWith(
      "/w/reason",
      {
        workflow: "story_turn",
        question: "Open the gate",
        revision: 2,
        timeline: "old",
        sessionId: "session",
        expectedVersion: 4,
      },
      expect.any(Object),
    );
    expect(onSaveTurn).not.toHaveBeenCalled();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Save candidate as provisional turn",
      }),
    );
    await waitFor(() =>
      expect(onSaveTurn).toHaveBeenCalledWith(
        expect.objectContaining({
          sessionVersion: 4,
          sessionId: "session",
          direction: "Open the gate",
        }),
        expect.any(String),
      ),
    );
  });
  it("retains archived scenes while preventing new turns", () => {
    render(
      <WorldStoryScene
        api={vi.fn()}
        archived
        path="/w"
        worldId="w"
        session={{
          id: "s",
          title: "Archived journey",
          pinnedRevision: 2,
          timeline: "main",
          version: 1,
          state: {},
          memory: {},
        }}
        turns={[{ version: 1, narration: "Preserved scene" }]}
      />,
    );
    expect(screen.getByText("Preserved scene")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Continue scene" }),
    ).toBeDisabled();
    expect(screen.getByLabelText("Your action")).toBeDisabled();
  });
});
