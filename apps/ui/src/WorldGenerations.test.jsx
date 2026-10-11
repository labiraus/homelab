import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import WorldGenerations from "./WorldGenerations.jsx";
const request = {
  workflow: "narrative_generate",
  question: "Continue",
  revision: 2,
  timeline: "past",
};
describe("Durable generations", () => {
  it("retries the same queue ID and retains original scope and part count", async () => {
    let first = true,
      job;
    const api = vi.fn(async (path, body) => {
      if (body) {
        if (first) {
          first = false;
          throw new Error("response lost");
        }
        job = {
          id: body.id,
          request: body,
          status: "queued",
          step: 0,
          parts: body.parts,
          results: [],
        };
        return job;
      }
      return path.endsWith("/generations") ? { enabled: true, items: [] } : job;
    });
    render(
      <WorldGenerations
        api={api}
        path="/world"
        request={request}
        onCandidate={vi.fn()}
      />,
    );
    const queue = await screen.findByRole("button", {
      name: "Queue creative generation",
    });
    fireEvent.change(screen.getByLabelText("Narrative parts"), {
      target: { value: "3" },
    });
    fireEvent.click(queue);
    await screen.findByText("response lost");
    fireEvent.click(queue);
    await waitFor(() =>
      expect(api.mock.calls.filter(([, body]) => body)).toHaveLength(2),
    );
    const calls = api.mock.calls.filter(([, body]) => body);
    expect(calls[0][1]).toEqual(calls[1][1]);
    expect(calls[1][1]).toMatchObject({ ...request, parts: 3 });
  });
  it("keeps completed candidates inspectable after uncertain work and refreshes manually", async () => {
    const original = {
      ...request,
      workflow: "story_turn",
      sessionId: "session",
      expectedVersion: 4,
    };
    const result = {
      answer: "Preserved scene",
      sessionVersion: 4,
      revision: 2,
    };
    const job = {
      id: "job",
      request: original,
      status: "uncertain",
      step: 1,
      parts: 2,
      errorCode: "lease_expired",
      results: [result],
    };
    const api = vi.fn(async (path) =>
      path.endsWith("/generations")
        ? {
            enabled: true,
            items: [{ ...job, question: "Continue", createdAt: "2026-01-01" }],
          }
        : job,
    );
    const inspect = vi.fn();
    render(
      <WorldGenerations
        api={api}
        path="/world"
        request={request}
        onCandidate={inspect}
      />,
    );
    fireEvent.click(
      await screen.findByRole("button", { name: /Continue · uncertain/ }),
    );
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Inspect candidate from part 1",
      }),
    );
    expect(inspect).toHaveBeenCalledWith(result, original);
    expect(
      screen.queryByRole("button", { name: "Cancel remaining parts" }),
    ).toBeNull();
    const count = api.mock.calls.length;
    fireEvent.click(
      screen.getByRole("button", { name: "Refresh generations" }),
    );
    await waitFor(() => expect(api.mock.calls.length).toBeGreaterThan(count));
  });
});
