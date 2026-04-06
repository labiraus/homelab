import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";

import App from "./App";

describe("App", () => {
	test("renders service controls", async () => {
		global.fetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ mode: "none", valid: false, invalidReason: "none" }),
		});

		render(<App />);
		expect(await screen.findByText("none")).toBeInTheDocument();

		expect(screen.getByRole("heading", { name: /verify backend routes/i })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /user count via external api/i })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /login with google/i })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /use dark mode/i })).toBeInTheDocument();
	});

	test("shows API response after a successful request", async () => {
		const user = userEvent.setup();
		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "oidc", email: "oliver@labiraus.com", valid: true }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ data: "Hello from Go" }),
			});

		render(<App />);
		await user.click(screen.getByRole("button", { name: /user count via external api/i }));

		expect(await screen.findByText("Hello from Go")).toBeInTheDocument();
		expect(global.fetch).toHaveBeenNthCalledWith(
			1,
			"/api/auth/status",
			expect.objectContaining({ method: "GET" }),
		);
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/users/count",
			expect.objectContaining({ method: "GET" }),
		);
	});

	test("toggles dark mode", async () => {
		const user = userEvent.setup();
		global.fetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ mode: "none", valid: false, invalidReason: "none" }),
		});
		render(<App />);
		expect(await screen.findByText("none")).toBeInTheDocument();

		const toggle = screen.getByRole("button", { name: /use dark mode/i });
		await user.click(toggle);

		expect(document.documentElement.dataset.theme).toBe("dark");
		expect(screen.getByRole("button", { name: /use light mode/i })).toBeInTheDocument();
	});

	test("shows recognized auth details after loading", async () => {
		global.fetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ mode: "certificate", email: "oliver@labiraus.com", valid: true }),
		});

		render(<App />);

		expect(await screen.findByText("certificate")).toBeInTheDocument();
		expect(screen.getByText("oliver@labiraus.com")).toBeInTheDocument();
		expect(screen.getByText("true")).toBeInTheDocument();
	});
});
