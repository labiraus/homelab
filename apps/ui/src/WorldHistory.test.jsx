import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { HistoryPanel, ReconciliationPanel } from "./WorldHistory.jsx";

const preview = {
  baseRevision: 1,
  currentRevision: 3,
  expectedVersion: 7,
  records: [
    {
      id: "hero",
      base: { label: "Old hero" },
      current: { label: "Current hero" },
      proposed: { label: "Story hero" },
      conflict: true,
    },
  ],
};

describe("canon history and reconciliation", () => {
  it("requires conflict decisions and preserves preview revision and story version", async () => {
    const api = vi.fn(async (_url, body) =>
      body ? { status: "draft" } : preview,
    );
    const onSaved = vi.fn();
    render(
      <ReconciliationPanel
        api={api}
        path="/w"
        target={{ kind: "sessions", id: "s" }}
        canEdit
        onSaved={onSaved}
        onClose={() => {}}
      />,
    );
    await screen.findByText("Story hero — conflict");
    fireEvent.change(screen.getByLabelText("Reconciliation reason"), {
      target: { value: "Reviewed differences" },
    });
    expect(
      screen.getByRole("button", { name: "Save reconciled draft" }),
    ).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Resolution for hero"), {
      target: { value: "proposed" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Save reconciled draft" }),
    );
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
    const [url, body] = api.mock.calls.find(([, body]) => body);
    expect(url).toBe("/w/sessions/s/reconcile");
    expect(body).toMatchObject({
      baseRevision: 3,
      expectedVersion: 7,
      resolutions: { hero: "proposed" },
      reason: "Reviewed differences",
    });
    expect(api.mock.calls.some(([url]) => url.includes("publish"))).toBe(false);
  });

  it("reuses the same id and preview on a retry after a lost response", async () => {
    let writes = 0;
    const api = vi.fn(async (_url, body) => {
      if (!body)
        return {
          ...preview,
          records: [{ ...preview.records[0], conflict: false }],
        };
      if (++writes === 1) throw new Error("Response lost");
      return { status: "draft" };
    });
    render(
      <ReconciliationPanel
        api={api}
        path="/w"
        target={{ kind: "changes", id: "c" }}
        canEdit
        onSaved={async () => {}}
        onClose={() => {}}
      />,
    );
    await screen.findByLabelText("Reconciliation reason");
    fireEvent.change(screen.getByLabelText("Reconciliation reason"), {
      target: { value: "Rebase" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Save reconciled draft" }),
    );
    await screen.findByText("Response lost");
    fireEvent.click(
      screen.getByRole("button", { name: "Save reconciled draft" }),
    );
    await screen.findByText(
      "Reconciled draft saved. Submit it in the review queue.",
    );
    const bodies = api.mock.calls
      .filter(([, body]) => body)
      .map(([, body]) => body);
    expect(bodies).toHaveLength(2);
    expect(bodies[0]).toEqual(bodies[1]);
  });

  it("offers an independently reviewed restore from the inspected history", async () => {
    const api = vi.fn(async (url, body) => {
      if (body) return { status: "draft" };
      if (url.includes("/revisions"))
        return [
          {
            revision: 1,
            reason: "Original",
            actor: "editor",
            createdAt: "2026-10-10T00:00:00Z",
          },
        ];
      return {
        fromRevision: 1,
        toRevision: 3,
        total: 1,
        records: [
          {
            id: "hero",
            before: { label: "Original" },
            after: { label: "Changed" },
          },
        ],
      };
    });
    const onSaved = vi.fn();
    render(
      <HistoryPanel
        api={api}
        path="/w"
        currentRevision={3}
        canEdit
        onSaved={onSaved}
      />,
    );
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Compare revision 1 with current",
      }),
    );
    await screen.findByLabelText("Restore reason");
    fireEvent.change(screen.getByLabelText("Restore reason"), {
      target: { value: "Undo label change" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Propose restoring revision 1" }),
    );
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
    expect(api).toHaveBeenCalledWith(
      "/w/restore",
      expect.objectContaining({
        baseRevision: 3,
        targetRevision: 1,
        reason: "Undo label change",
      }),
    );
  });
});
