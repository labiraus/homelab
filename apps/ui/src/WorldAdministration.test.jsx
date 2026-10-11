import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import WorldAdministration from "./WorldAdministration.jsx";

describe("world administration", () => {
  it("submits an audited owner action with the displayed administration version", async () => {
    const api = vi.fn(async (_url, body) =>
      body ? {} : [{ principal: "owner@test", role: "owner" }],
    );
    const onSaved = vi.fn();
    render(
      <WorldAdministration
        api={api}
        path="/w"
        world={{ adminVersion: 4, archived: false }}
        onSaved={onSaved}
      />,
    );
    await screen.findByText("owner@test · owner");
    fireEvent.change(screen.getByLabelText("Administration action"), {
      target: { value: "archive" },
    });
    fireEvent.change(screen.getByLabelText("Administration reason"), {
      target: { value: "Pause writing" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Apply administration change" }),
    );
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
    const [url, body] = api.mock.calls.find(([, body]) => body);
    expect(url).toBe("/w/administration");
    expect(body).toMatchObject({
      action: "archive",
      expectedVersion: 4,
      reason: "Pause writing",
    });
    expect(body.principal).toBeUndefined();
    expect(body.role).toBeUndefined();
    expect(body.requestId).toBeTruthy();
  });
});
