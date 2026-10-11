import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import WorldUsers from "./WorldUsers.jsx";

describe("superuser user management", () => {
  it("adds a normal user using their Google email and shows registration errors", async () => {
    const api = vi.fn(async (path, body) => {
      if (path === "/account") return { superuser: true };
      if (body) throw new Error("Registration unavailable");
      return [{ email: "owner@example.test", displayName: "Owner", superuser: true }];
    });
    render(<WorldUsers api={api} />);
    fireEvent.click(await screen.findByText("Superuser · Manage users"));
    await screen.findByText("Owner · owner@example.test · Superuser");
    fireEvent.change(screen.getByLabelText("Google account email"), { target: { value: "new@example.test" } });
    fireEvent.change(screen.getByLabelText("Display name"), { target: { value: "New person" } });
    fireEvent.click(screen.getByRole("button", { name: "Add user" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Registration unavailable");
    expect(api).toHaveBeenCalledWith("/users", { email: "new@example.test", displayName: "New person" });
    expect(screen.getByLabelText("Google account email")).toHaveValue("new@example.test");
  });
  it("does not load the registry for an ordinary account", async () => {
    const api = vi.fn(async () => ({ superuser: false }));
    render(<WorldUsers api={api} />);
    await waitFor(() => expect(api).toHaveBeenCalledWith("/account"));
    expect(api).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("Superuser · Manage users")).not.toBeInTheDocument();
  });
});
