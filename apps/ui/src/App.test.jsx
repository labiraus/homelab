import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";

import App from "./App";

describe("App", () => {
	test("renders service controls", () => {
		render(<App />);

		expect(screen.getByRole("heading", { name: /verify backend routes/i })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /user count via external api/i })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /use dark mode/i })).toBeInTheDocument();
	});

	test("shows API response after a successful request", async () => {
		const user = userEvent.setup();
		global.fetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ data: "Hello from Go" }),
		});

		render(<App />);
		await user.click(screen.getByRole("button", { name: /user count via external api/i }));

		expect(await screen.findByText("Hello from Go")).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/users/count",
			expect.objectContaining({ method: "GET" }),
		);
	});

	test("toggles dark mode", async () => {
		const user = userEvent.setup();
		render(<App />);

		const toggle = screen.getByRole("button", { name: /use dark mode/i });
		await user.click(toggle);

		expect(document.documentElement.dataset.theme).toBe("dark");
		expect(screen.getByRole("button", { name: /use light mode/i })).toBeInTheDocument();
	});
});
