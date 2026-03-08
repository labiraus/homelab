import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";

import App from "./App";

describe("App", () => {
	test("renders service controls", () => {
		render(<App />);

		expect(screen.getByRole("heading", { name: /verify backend routes/i })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /call go api/i })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /call python api/i })).toBeInTheDocument();
	});

	test("shows API response after a successful request", async () => {
		const user = userEvent.setup();
		global.fetch = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ data: "Hello from Go" }),
		});

		render(<App />);
		await user.click(screen.getByRole("button", { name: /call go api/i }));

		expect(await screen.findByText("Hello from Go")).toBeInTheDocument();
	});
});
