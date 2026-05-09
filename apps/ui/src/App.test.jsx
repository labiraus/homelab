import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";

import App from "./App";

class MockEventSource {
	static instances = [];

	constructor(url) {
		this.url = url;
		this.listeners = new Map();
		this.close = vi.fn();
		MockEventSource.instances.push(this);
	}

	addEventListener(type, listener) {
		const current = this.listeners.get(type) ?? [];
		current.push(listener);
		this.listeners.set(type, current);
	}

	removeEventListener(type, listener) {
		const current = this.listeners.get(type) ?? [];
		this.listeners.set(
			type,
			current.filter((entry) => entry !== listener),
		);
	}

	emit(type, payload) {
		for (const listener of this.listeners.get(type) ?? []) {
			listener({ data: JSON.stringify(payload) });
		}
	}
}

describe("App", () => {
	afterEach(() => {
		vi.restoreAllMocks();
		window.location.hash = "";
		MockEventSource.instances = [];
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
		expect(screen.getByRole("button", { name: /^search$/i })).toBeInTheDocument();

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

	test("searches processed chunks from the search tab", async () => {
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
					query: "refresh kubeconfig",
					hits: [
						{
							documentId: "doc-1",
							objectKey: "scripts/refresh-kubeconfig.sh",
							contentType: "text/x-shellscript",
							chunkId: 42,
							chunkIndex: 0,
							chunkText: "aws eks update-kubeconfig --name homelab",
							similarity: 0.92,
							lastProcessedAt: "2026-04-14T12:00:00Z",
							citation: {
								id: "s3://documents/scripts/refresh-kubeconfig.sh#chunk-0",
								label: "scripts/refresh-kubeconfig.sh chunk 0",
								sourceUri: "s3://documents/scripts/refresh-kubeconfig.sh",
								objectKey: "scripts/refresh-kubeconfig.sh",
								chunkId: 42,
								chunkIndex: 0,
								processingVersion: 1,
							},
						},
					],
				}),
			});

		render(<App />);
		await user.click(await screen.findByRole("button", { name: /^search$/i }));
		await user.type(
			screen.getByRole("textbox", { name: /search query/i }),
			"refresh kubeconfig",
		);
		await user.type(
			screen.getByRole("textbox", { name: /optional prefix filter/i }),
			"scripts/",
		);
		await user.click(screen.getByRole("button", { name: /search chunks/i }));

		expect(await screen.findByText("scripts/refresh-kubeconfig.sh")).toBeInTheDocument();
		expect(screen.getByText("scripts/refresh-kubeconfig.sh chunk 0")).toBeInTheDocument();
		expect(screen.getByText(/aws eks update-kubeconfig/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/search",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					query: "refresh kubeconfig",
					prefix: "scripts/",
					limit: 8,
				}),
			}),
		);
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

	test("subscribes to document events for authenticated users and shows toast notifications", async () => {
		const originalEventSource = global.EventSource;
		global.EventSource = MockEventSource;

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

		const { unmount } = render(<App />);
		await screen.findByRole("button", { name: /^documents$/i });

		expect(MockEventSource.instances).toHaveLength(1);
		expect(MockEventSource.instances[0].url).toBe("/api/documents/events");

		act(() => {
			MockEventSource.instances[0].emit("document", {
				subject: "documents.events.processor.completed",
				documentId: "doc-1",
				objectKey: "reports/doc-1.txt",
				occurredAt: "2026-04-14T10:00:00Z",
			});
		});

		expect(await screen.findByText("Processing completed")).toBeInTheDocument();
		expect(screen.getByText("reports/doc-1.txt is ready for retrieval.")).toBeInTheDocument();

		unmount();
		expect(MockEventSource.instances[0].close).toHaveBeenCalled();
		global.EventSource = originalEventSource;
	});

	test("does not subscribe to document events when the user is not authenticated", async () => {
		const originalEventSource = global.EventSource;
		global.EventSource = MockEventSource;

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

		expect(MockEventSource.instances).toHaveLength(0);
		global.EventSource = originalEventSource;
	});
});
