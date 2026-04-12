import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";

import App from "./App";

describe("App", () => {
	afterEach(() => {
		vi.restoreAllMocks();
		window.location.hash = "";
	});

	test("renders the github-style header shell and auth menu", async () => {
		const user = userEvent.setup();
		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "none", valid: false, invalidReason: "none" }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					providers: [
						{
							id: "google",
							name: "Google",
							issuer: "https://accounts.google.com",
							authorizationUrl: "https://accounts.google.com/o/oauth2/v2/auth",
							configured: true,
						},
					],
				}),
			});

		render(<App />);

		expect(
			await screen.findByRole("heading", { name: /inspect the deployed labiraus surface/i }),
		).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /overview/i })).toBeInTheDocument();
		expect(screen.queryByRole("button", { name: /^documents$/i })).not.toBeInTheDocument();

		await user.click(screen.getByRole("button", { name: /authentication menu/i }));

		expect(screen.getAllByText("No authenticated identity is present.").length).toBeGreaterThan(0);
		expect(screen.getByRole("button", { name: /sign in with google/i })).toBeInTheDocument();
		expect(screen.getByText(/client-certificate authentication/i)).toBeInTheDocument();
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
				json: async () => ({
					providers: [
						{
							id: "google",
							name: "Google",
							issuer: "https://accounts.google.com",
							authorizationUrl: "https://accounts.google.com/o/oauth2/v2/auth",
							configured: true,
						},
					],
				}),
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
		expect(global.fetch).toHaveBeenNthCalledWith(
			2,
			"/api/auth/providers",
			expect.objectContaining({ method: "GET" }),
		);
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/users/count",
			expect.objectContaining({ method: "GET" }),
		);
	});

	test("toggles dark mode from the auth menu", async () => {
		const user = userEvent.setup();
		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "none", valid: false, invalidReason: "none" }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					providers: [
						{
							id: "google",
							name: "Google",
							issuer: "https://accounts.google.com",
							authorizationUrl: "https://accounts.google.com/o/oauth2/v2/auth",
							configured: true,
						},
					],
				}),
			});

		render(<App />);
		await screen.findByRole("heading", { name: /inspect the deployed labiraus surface/i });
		await user.click(screen.getByRole("button", { name: /authentication menu/i }));
		await user.click(screen.getByRole("button", { name: /use dark mode/i }));

		expect(document.documentElement.dataset.theme).toBe("dark");
		expect(screen.getByRole("button", { name: /use light mode/i })).toBeInTheDocument();
	});

	test("shows recognized auth details and exposes the documents tab", async () => {
		const user = userEvent.setup();
		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "certificate", email: "oliver@labiraus.com", valid: true }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					providers: [
						{
							id: "google",
							name: "Google",
							issuer: "https://accounts.google.com",
							authorizationUrl: "https://accounts.google.com/o/oauth2/v2/auth",
							configured: true,
						},
					],
				}),
			});

		render(<App />);
		expect(await screen.findByRole("button", { name: /^documents$/i })).toBeInTheDocument();

		await user.click(screen.getByRole("button", { name: /authentication menu/i }));
		expect(screen.getAllByText("oliver@labiraus.com").length).toBeGreaterThan(0);
		expect(screen.getByText("certificate")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /sign out/i })).toBeInTheDocument();
	});

	test("uses the configured federated provider URL for sign in", async () => {
		const user = userEvent.setup();
		const assign = vi.fn();
		const originalLocation = window.location;
		delete window.location;
		window.location = { assign, hash: "" };

		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "none", valid: false, invalidReason: "none" }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					providers: [
						{
							id: "google",
							name: "Google",
							issuer: "https://accounts.google.com",
							authorizationUrl: "https://accounts.google.com/o/oauth2/v2/auth",
							configured: true,
						},
					],
				}),
			});

		render(<App />);
		await screen.findByRole("heading", { name: /inspect the deployed labiraus surface/i });
		await user.click(screen.getByRole("button", { name: /authentication menu/i }));
		await user.click(screen.getByRole("button", { name: /sign in with google/i }));

		expect(assign).toHaveBeenCalledWith("https://accounts.google.com/o/oauth2/v2/auth");
		window.location = originalLocation;
	});

	test("uses the oauth2 sign out path when authenticated", async () => {
		const user = userEvent.setup();
		const assign = vi.fn();
		const originalLocation = window.location;
		delete window.location;
		window.location = { assign, hash: "" };

		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "oidc", email: "oliver@labiraus.com", valid: true }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					providers: [
						{
							id: "google",
							name: "Google",
							issuer: "https://accounts.google.com",
							authorizationUrl: "https://accounts.google.com/o/oauth2/v2/auth",
							configured: true,
						},
					],
				}),
			});

		render(<App />);
		await screen.findByRole("button", { name: /^documents$/i });
		await user.click(screen.getByRole("button", { name: /authentication menu/i }));
		await user.click(screen.getByRole("button", { name: /sign out/i }));

		expect(assign).toHaveBeenCalledWith("/oauth2/sign_out");
		window.location = originalLocation;
	});

	test("navigates to the documents page and shows folder entries", async () => {
		const user = userEvent.setup();
		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "oidc", email: "oliver@labiraus.com", valid: true }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					providers: [
						{
							id: "google",
							name: "Google",
							issuer: "https://accounts.google.com",
							authorizationUrl: "https://accounts.google.com/o/oauth2/v2/auth",
							configured: true,
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					bucket: "documents",
					prefix: "",
					breadcrumbs: [{ name: "documents", prefix: "" }],
					entries: [
						{ name: "reports", type: "folder", prefix: "reports/" },
						{
							name: "notes.txt",
							type: "file",
							objectKey: "notes.txt",
							sizeBytes: 12,
							contentType: "text/plain",
							lastModified: "2026-04-12T12:00:00Z",
						},
					],
				}),
			});

		render(<App />);
		await screen.findByRole("button", { name: /^documents$/i });
		await user.click(screen.getByRole("button", { name: /^documents$/i }));

		expect(await screen.findByText(/folder: reports/i)).toBeInTheDocument();
		expect(screen.getByText(/file: notes.txt/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/tree",
			expect.objectContaining({ method: "GET" }),
		);
	});

	test("does not show the documents tab before authentication", async () => {
		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "none", valid: false, invalidReason: "none" }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					providers: [
						{
							id: "google",
							name: "Google",
							issuer: "https://accounts.google.com",
							authorizationUrl: "https://accounts.google.com/o/oauth2/v2/auth",
							configured: true,
						},
					],
				}),
			});

		render(<App />);
		await screen.findByRole("heading", { name: /inspect the deployed labiraus surface/i });

		expect(screen.queryByRole("button", { name: /^documents$/i })).not.toBeInTheDocument();
		expect(global.fetch).not.toHaveBeenCalledWith(
			"/api/documents/tree",
			expect.objectContaining({ method: "GET" }),
		);
	});
});
