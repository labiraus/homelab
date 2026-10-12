import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import WorldTimelineBranch from "./WorldTimelineBranch.jsx";

function fields() {
  fireEvent.click(screen.getByRole("button", { name: "Prepare timeline branch" }));
  fireEvent.change(screen.getByLabelText("New timeline ID"), { target: { value: "alternate" } });
  fireEvent.change(screen.getByLabelText("New timeline name"), { target: { value: "Alternate history" } });
  fireEvent.change(screen.getByLabelText("Branch reason"), { target: { value: "Explore a different outcome" } });
}

describe("timeline branches", () => {
  it("pins the inspected parent and preserves an exact request ID after a lost response", async () => {
    const api = vi.fn().mockRejectedValueOnce(new Error("Lost response")).mockResolvedValue({ id: "draft" });
    const onSaved = vi.fn();
    const props = { api, path: "/w", timeline: "main", revision: 2, currentRevision: 4, canEdit: true, onSaved };
    const { rerender } = render(<WorldTimelineBranch {...props} />);
    fields();
    rerender(<WorldTimelineBranch {...props} timeline="different" revision={3} />);
    fireEvent.click(screen.getByRole("button", { name: "Create branch draft" }));
    await screen.findByText("Lost response");
    const first = api.mock.calls[0];
    expect(first[0]).toBe("/w/timeline-branches");
    expect(first[1]).toMatchObject({ parentTimeline: "main", parentRevision: 2, baseRevision: 4, timeline: "alternate" });
    rerender(<WorldTimelineBranch {...props} timeline="different" revision={3} currentRevision={5} />);
    fireEvent.click(screen.getByRole("button", { name: "Create branch draft" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
    expect(api.mock.calls[1]).toEqual(first);
    expect(api.mock.calls.every(([url]) => !url.includes("publish"))).toBe(true);
    expect(screen.getByRole("status")).toHaveTextContent("Review and publish");
  });

  it("refuses silent draft rebasing after current canon advances", () => {
    const props = { api: vi.fn(), path: "/w", timeline: "main", revision: 2, currentRevision: 4, canEdit: true, onSaved: vi.fn() };
    const { rerender } = render(<WorldTimelineBranch {...props} />);
    fields();
    rerender(<WorldTimelineBranch {...props} currentRevision={5} />);
    expect(screen.getByRole("button", { name: "Create branch draft" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("Canon changed");
    expect(props.api).not.toHaveBeenCalled();
  });

  it("allows a reader to inspect origin only at the displayed historical revision", async () => {
    const api = vi.fn().mockResolvedValue({ request: { parentTimeline: "main", parentRevision: 1 }, publishedRevision: 2, records: { fact: "copy" } });
    render(<WorldTimelineBranch api={api} path="/w" timeline="alternate" revision={3} currentRevision={8} canEdit={false} />);
    expect(screen.queryByRole("button", { name: "Prepare timeline branch" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "View timeline origin" }));
    await screen.findByText(/created from main at revision 1/);
    expect(api).toHaveBeenCalledExactlyOnceWith("/w/timelines/alternate/origin?revision=3");
  });
});
