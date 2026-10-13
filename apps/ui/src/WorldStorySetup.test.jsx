import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import WorldStorySetup from "./WorldStorySetup.jsx";

describe("Private story setup", () => {
  it("reuses the definition pin when current canon advances", async () => {
    const definition = {
      id: "definition",
      title: "Old journey",
      pinnedRevision: 1,
      timeline: "past",
      premise: "Explore",
    };
    const api = vi.fn(async (path, body) =>
      path.includes("/canon?") ? { records: [], total: 0 } : body ? { ...body, version: 0, state: {} } : [definition],
    );
    const onStart = vi.fn();
    render(
      <WorldStorySetup
        api={api}
        path="/world"
        revision={5}
        timeline="main"
        onStart={onStart}
      />,
    );
    await screen.findByRole("option", { name: "Old journey · revision 1" });
    fireEvent.change(screen.getByLabelText("Reusable definition"), {
      target: { value: "definition" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Start at revision 1" }),
    );
    await waitFor(() => expect(onStart).toHaveBeenCalled());
    const [, body] = api.mock.calls.find(([path]) =>
      path.endsWith("/sessions"),
    );
    expect(body).toMatchObject({
      definitionId: "definition",
      pinnedRevision: 1,
      timeline: "past",
    });
  });
  it("retains request IDs on an uncertain retry and preserves generated premise scope", async () => {
    let failed = true;
    const api = vi.fn(async (path, body) => {
      if (path.includes("/canon?")) return { records: [], total: 0 };
      if (!body) return [];
      if (path.endsWith("/sessions") && failed) {
        failed = false;
        throw new Error("connection interrupted");
      }
      return { ...body, version: 0, state: {} };
    });
    render(
      <WorldStorySetup
        api={api}
        path="/world"
        revision={5}
        timeline="main"
        onStart={vi.fn()}
        premiseCandidate={{
          text: "Generated scene",
          revision: 2,
          timeline: "branch",
        }}
      />,
    );
    fireEvent.change(screen.getByLabelText("Story title"), {
      target: { value: "New journey" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Start at revision 2" }),
    );
    await screen.findByText("connection interrupted");
    fireEvent.click(
      screen.getByRole("button", { name: "Start at revision 2" }),
    );
    await waitFor(() =>
      expect(
        api.mock.calls.filter(([path]) => path.endsWith("/sessions")),
      ).toHaveLength(2),
    );
    const definitions = api.mock.calls.filter(
      ([path, body]) => path.endsWith("/story-definitions") && body,
    );
    expect(definitions[0][1]).toEqual(definitions[1][1]);
    expect(definitions[0][1]).toMatchObject({
      premise: "Generated scene",
      pinnedRevision: 2,
      timeline: "branch",
    });
  });
});
