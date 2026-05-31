import { act, render, screen, waitFor, within } from "@testing-library/react";
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
		expect(screen.queryByRole("button", { name: /^inventory$/i })).not.toBeInTheDocument();

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
		expect(await screen.findByRole("button", { name: /^assistant$/i })).toBeInTheDocument();
		expect(await screen.findByRole("button", { name: /^documents$/i })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /^search$/i })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /^inventory$/i })).toBeInTheDocument();

		await user.click(screen.getByRole("button", { name: /authentication menu/i }));
		expect(screen.getAllByText("oliver@labiraus.com").length).toBeGreaterThan(0);
		expect(screen.getByText("certificate")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /sign out/i })).toBeInTheDocument();
	});

	test("sends an assistant chat message through the browser API", async () => {
		const user = userEvent.setup();
		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "oidc", email: "oliver@labiraus.com", valid: true }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ providers: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ conversations: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ memories: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ audits: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					conversation: {
						conversationId: "conv-1",
						title: "Runbook",
						updatedAt: "2026-05-13T12:00:00Z",
					},
					userMessage: {
						messageId: "msg-user",
						conversationId: "conv-1",
						role: "user",
						content: "Which runbook?",
						citations: [],
						metadata: {},
					},
					reply: {
						messageId: "msg-assistant",
						conversationId: "conv-1",
						role: "assistant",
						content: "Use the kubeconfig runbook.",
						citations: [{ reference: "[1]", citation: { label: "runbooks/process.md chunk 0" } }],
						metadata: {},
					},
					toolCalls: [],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					conversations: [
						{
							conversationId: "conv-1",
							title: "Runbook",
							updatedAt: "2026-05-13T12:00:00Z",
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ proposals: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ audits: [] }),
			});

		render(<App />);
		await user.click(await screen.findByRole("button", { name: /^assistant$/i }));
		await screen.findByRole("heading", { name: /talk to labiraus/i });
		await user.type(screen.getByLabelText(/^message$/i), "Which runbook?");
		await user.click(screen.getByRole("button", { name: /^send$/i }));

		expect(await screen.findByText("Use the kubeconfig runbook.")).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/assistant/chat",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					conversationId: "",
					message: "Which runbook?",
					title: "Which runbook?",
				}),
			}),
		);
	});

	test("approves an assistant file proposal", async () => {
		const user = userEvent.setup();
		const pendingProposal = {
			proposalId: "prop-1",
			conversationId: "conv-1",
			action: "edit",
			documentId: "doc-1",
			objectKey: "runbooks/process.md",
			contentType: "text/markdown",
			proposedText: "edited",
			rationale: "Tighten wording",
			status: "pending",
			createdAt: "2026-05-13T12:00:00Z",
		};
		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "oidc", email: "oliver@labiraus.com", valid: true }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ providers: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					conversations: [
						{
							conversationId: "conv-1",
							title: "Runbook",
							updatedAt: "2026-05-13T12:00:00Z",
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ memories: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ audits: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					conversation: {
						conversationId: "conv-1",
						title: "Runbook",
						updatedAt: "2026-05-13T12:00:00Z",
					},
					messages: [],
					proposals: [pendingProposal],
					audits: [],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					proposal: {
						...pendingProposal,
						status: "approved",
						orchestratorResponse: { status: "queued", processingVersion: 4 },
					},
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					proposals: [
						{
							...pendingProposal,
							status: "approved",
							orchestratorResponse: { status: "queued", processingVersion: 4 },
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ audits: [] }),
			});

		render(<App />);
		await user.click(await screen.findByRole("button", { name: /^assistant$/i }));
		await screen.findByText("Tighten wording");
		await user.click(screen.getByRole("button", { name: /^approve$/i }));

		await waitFor(() => {
			expect(global.fetch).toHaveBeenCalledWith(
				"/api/assistant/proposals/prop-1/approve",
				expect.objectContaining({ method: "POST" }),
			);
		});
		expect(await screen.findByText(/^approved$/i)).toBeInTheDocument();
		expect(await screen.findByText(/orchestrator queued .* processing v4/i)).toBeInTheDocument();
	});

	test("rejects an assistant file proposal with an audit reason", async () => {
		const user = userEvent.setup();
		const pendingProposal = {
			proposalId: "prop-1",
			conversationId: "conv-1",
			action: "edit",
			documentId: "doc-1",
			objectKey: "runbooks/process.md",
			contentType: "text/markdown",
			proposedText: "edited",
			rationale: "Tighten wording",
			status: "pending",
			createdAt: "2026-05-13T12:00:00Z",
		};
		global.fetch = vi
			.fn()
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ mode: "oidc", email: "oliver@labiraus.com", valid: true }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ providers: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					conversations: [
						{
							conversationId: "conv-1",
							title: "Runbook",
							updatedAt: "2026-05-13T12:00:00Z",
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ memories: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ audits: [] }),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					conversation: {
						conversationId: "conv-1",
						title: "Runbook",
						updatedAt: "2026-05-13T12:00:00Z",
					},
					messages: [],
					proposals: [pendingProposal],
					audits: [],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					proposal: {
						...pendingProposal,
						status: "rejected",
						orchestratorResponse: { reason: "needs source context" },
					},
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					proposals: [
						{
							...pendingProposal,
							status: "rejected",
							orchestratorResponse: { reason: "needs source context" },
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({ audits: [] }),
			});

		render(<App />);
		await user.click(await screen.findByRole("button", { name: /^assistant$/i }));
		await screen.findByText("Tighten wording");
		await user.type(
			screen.getByRole("textbox", { name: /rejection reason for runbooks\/process\.md/i }),
			"needs source context",
		);
		await user.click(screen.getByRole("button", { name: /^reject$/i }));

		await waitFor(() => {
			expect(global.fetch).toHaveBeenCalledWith(
				"/api/assistant/proposals/prop-1/reject",
				expect.objectContaining({
					method: "POST",
					body: JSON.stringify({ reason: "needs source context" }),
				}),
			);
		});
		expect(await screen.findByText(/^rejected$/i)).toBeInTheDocument();
		expect(await screen.findByText(/reason: needs source context/i)).toBeInTheDocument();
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

	test("loads document inventory with filters", async () => {
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
					count: 1,
					documents: [
						{
							documentId: "doc-1",
							bucket: "documents",
							objectKey: "runbooks/process.md",
							sourceUri: "s3://documents/runbooks/process.md",
							contentType: "text/markdown",
							status: "processed",
							metadata: { tag: "runbook" },
							desiredProcessingVersion: 3,
							currentProcessingVersion: 2,
							lastReconciledAt: "2026-05-10T17:00:00Z",
							lastProcessedAt: "2026-05-10T17:01:00Z",
							lastEventSubject: "documents.events.processor.completed",
							lastEventAt: "2026-05-10T17:01:00Z",
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					status: "scanned",
					bucket: "documents",
					prefix: "runbooks/",
					scanned: 2,
					created: 0,
					updated: 1,
					queued: 1,
					skipped: 1,
					unsupported: 0,
					failed: 0,
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					count: 1,
					documents: [
						{
							documentId: "doc-1",
							bucket: "documents",
							objectKey: "runbooks/process.md",
							sourceUri: "s3://documents/runbooks/process.md",
							contentType: "text/markdown",
							status: "processed",
							metadata: { tag: "runbook" },
							desiredProcessingVersion: 3,
							currentProcessingVersion: 2,
							lastReconciledAt: "2026-05-10T17:02:00Z",
							lastProcessedAt: "2026-05-10T17:01:00Z",
							lastEventSubject: "documents.events.processor.completed",
							lastEventAt: "2026-05-10T17:01:00Z",
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					status: "updated",
					documentId: "doc-1",
					metadata: { tag: "runbook", owner: "ops" },
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					documentId: "doc-1",
					count: 1,
					events: [
						{
							id: 21,
							documentId: "doc-1",
							subject: "documents.events.processor.started",
							processingVersion: 2,
							occurredAt: "2026-05-10T17:00:30Z",
							createdAt: "2026-05-10T17:00:31Z",
							payload: {
								documentId: "doc-1",
								objectKey: "runbooks/process.md",
							},
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					status: "queued",
					documentId: "doc-1",
					processingVersion: 4,
					sourceUri: "s3://documents/runbooks/process.md",
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					documentId: "doc-1",
					count: 1,
					events: [
						{
							id: 22,
							documentId: "doc-1",
							subject: "documents.events.processor.queued",
							processingVersion: 4,
							occurredAt: "2026-05-10T17:03:00Z",
							createdAt: "2026-05-10T17:03:01Z",
							payload: {
								documentId: "doc-1",
								objectKey: "runbooks/process.md",
							},
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				text: async () => "old inventory process notes",
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					status: "queued",
					documentId: "doc-1",
					processingVersion: 5,
					sourceUri: "s3://documents/runbooks/process.md",
					objectKey: "runbooks/process.md",
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					documentId: "doc-1",
					count: 1,
					events: [
						{
							id: 23,
							documentId: "doc-1",
							subject: "documents.events.processor.queued",
							processingVersion: 5,
							occurredAt: "2026-05-10T17:04:00Z",
							createdAt: "2026-05-10T17:04:01Z",
							payload: {
								documentId: "doc-1",
								objectKey: "runbooks/process.md",
							},
						},
					],
				}),
			});

		render(<App />);
		await user.click(await screen.findByRole("button", { name: /^inventory$/i }));
		await user.selectOptions(
			screen.getByRole("combobox", { name: /inventory status filter/i }),
			"processed",
		);
		await user.type(
			screen.getByRole("textbox", { name: /inventory document id filter/i }),
			"doc-1",
		);
		await user.type(
			screen.getByRole("textbox", { name: /inventory prefix filter/i }),
			"runbooks/",
		);
		await user.type(screen.getByRole("textbox", { name: /inventory metadata key/i }), "tag");
		await user.type(
			screen.getByRole("textbox", { name: /inventory metadata value/i }),
			"runbook",
		);
		await user.click(screen.getByRole("button", { name: /load inventory/i }));

		expect(await screen.findByText("runbooks/process.md")).toBeInTheDocument();
		expect(screen.getAllByText("processed").length).toBeGreaterThan(1);
		expect(screen.getByText(/current v2 \/ desired v3/i)).toBeInTheDocument();
		expect(screen.getByText(/Processing completed/)).toBeInTheDocument();
		expect(screen.getByText("runbook")).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/inventory",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					status: "processed",
					documentId: "doc-1",
					prefix: "runbooks/",
					metadata: { tag: "runbook" },
					limit: 50,
				}),
			}),
		);

		await user.click(screen.getByRole("button", { name: /scan bucket/i }));

		expect(
			await screen.findByText(/scanned 2 objects; queued 1, skipped 1, unsupported 0, failed 0/i),
		).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/scan-bucket",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					prefix: "runbooks/",
					maxKeys: 50,
				}),
			}),
		);
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/inventory",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					status: "processed",
					documentId: "doc-1",
					prefix: "runbooks/",
					metadata: { tag: "runbook" },
					limit: 50,
				}),
			}),
		);

		await user.click(
			screen.getByRole("button", {
				name: /curate inventory metadata for runbooks\/process\.md/i,
			}),
		);

		const inventoryMetadataPanel = screen.getByRole("region", {
			name: /inventory metadata curation/i,
		});
		await user.type(
			within(inventoryMetadataPanel).getByRole("textbox", {
				name: /inventory curation metadata key/i,
			}),
			"owner",
		);
		await user.type(
			within(inventoryMetadataPanel).getByRole("textbox", {
				name: /inventory curation metadata value/i,
			}),
			"ops",
		);
		await user.click(
			within(inventoryMetadataPanel).getByRole("button", {
				name: /update inventory metadata/i,
			}),
		);

		expect(
			await within(inventoryMetadataPanel).findByText(
				/updated metadata for runbooks\/process\.md/i,
			),
		).toBeInTheDocument();
		expect(within(inventoryMetadataPanel).getByText("ops")).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/curation",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					documentId: "doc-1",
					metadata: { owner: "ops" },
					replace: false,
				}),
			}),
		);

		await user.click(
			screen.getByRole("button", { name: /inventory history for runbooks\/process\.md/i }),
		);

		expect(await screen.findByText("Processing started")).toBeInTheDocument();
		expect(screen.getByText(/runbooks\/process\.md recorded in postgres/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/history",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					documentId: "doc-1",
					limit: 20,
				}),
			}),
		);

		await user.click(
			screen.getByRole("button", {
				name: /queue reprocess for inventory runbooks\/process\.md/i,
			}),
		);

		expect(
			await screen.findByText(/queued processing version 4 for runbooks\/process\.md/i),
		).toBeInTheDocument();
		expect(screen.getByText(/current v2 \/ desired v4/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/reprocess",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({ documentId: "doc-1" }),
			}),
		);
		expect(await screen.findByText("Processing queued")).toBeInTheDocument();
		expect(
			screen.getByText(/runbooks\/process\.md version 4 recorded in postgres/i),
		).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/history",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					documentId: "doc-1",
					processingVersion: 4,
					limit: 20,
				}),
			}),
		);

		await user.click(
			screen.getByRole("button", {
				name: /edit inventory text for runbooks\/process\.md/i,
			}),
		);

		const inventoryEditPanel = screen.getByRole("region", {
			name: /inventory text edit/i,
		});
		await user.click(
			within(inventoryEditPanel).getByRole("button", { name: /load inventory text/i }),
		);

		const inventoryEditTextArea = await within(inventoryEditPanel).findByRole("textbox", {
			name: /inventory editable document text/i,
		});
		expect(inventoryEditTextArea).toHaveValue("old inventory process notes");

		await user.clear(inventoryEditTextArea);
		await user.type(inventoryEditTextArea, "updated inventory process notes");
		await user.click(
			within(inventoryEditPanel).getByRole("checkbox", {
				name: /confirm inventory raw text overwrite/i,
			}),
		);
		await user.click(
			within(inventoryEditPanel).getByRole("button", {
				name: /save inventory text edit/i,
			}),
		);

		expect(
			await within(inventoryEditPanel).findByText(/queued text edit as processing version 5/i),
		).toBeInTheDocument();
		expect(screen.getByText(/current v2 \/ desired v5/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/object?objectKey=runbooks%2Fprocess.md",
		);
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/edit-text",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					documentId: "doc-1",
					text: "updated inventory process notes",
					contentType: "text/markdown",
				}),
			}),
		);
		expect(
			await screen.findByText(/runbooks\/process\.md version 5 recorded in postgres/i),
		).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/history",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					documentId: "doc-1",
					processingVersion: 5,
					limit: 20,
				}),
			}),
		);
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
		await user.type(screen.getByRole("textbox", { name: /metadata key/i }), "tag");
		await user.type(screen.getByRole("textbox", { name: /metadata value/i }), "runbook");
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
					metadata: { tag: "runbook" },
					limit: 8,
				}),
			}),
		);
	});

	test("assembles cited context from the search tab filters", async () => {
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
					query: "ancient tower",
					context: "[1] campaign/tower.md chunk 0\nStone stairs climb toward the beacon.",
					citations: [
						{
							reference: "[1]",
							citation: {
								id: "s3://documents/campaign/tower.md#chunk-0",
								label: "campaign/tower.md chunk 0",
								sourceUri: "s3://documents/campaign/tower.md",
								objectKey: "campaign/tower.md",
								chunkId: 12,
								chunkIndex: 0,
								processingVersion: 4,
							},
						},
					],
					hits: [],
					maxChars: 6000,
					truncated: true,
				}),
			});

		render(<App />);
		await user.click(await screen.findByRole("button", { name: /^search$/i }));
		await user.type(screen.getByRole("textbox", { name: /search query/i }), "ancient tower");
		await user.type(
			screen.getByRole("textbox", { name: /optional prefix filter/i }),
			"campaign/",
		);
		await user.type(screen.getByRole("textbox", { name: /metadata key/i }), "tag");
		await user.type(screen.getByRole("textbox", { name: /metadata value/i }), "lore");
		await user.click(screen.getByRole("button", { name: /assemble context/i }));

		expect(await screen.findByText(/Stone stairs climb toward the beacon/i)).toBeInTheDocument();
		expect(screen.getByText("campaign/tower.md chunk 0")).toBeInTheDocument();
		expect(screen.getByText(/Truncated at 6000 characters/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/context",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					query: "ancient tower",
					prefix: "campaign/",
					metadata: { tag: "lore" },
					limit: 6,
					maxChars: 6000,
				}),
			}),
		);
	});

	test("loads durable lifecycle history for a search result", async () => {
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
					query: "processing status",
					hits: [
						{
							documentId: "doc-1",
							objectKey: "runbooks/process.md",
							contentType: "text/markdown",
							chunkId: 12,
							chunkIndex: 0,
							chunkText: "Processor completion is recorded in Postgres.",
							similarity: 0.88,
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					documentId: "doc-1",
					count: 2,
					events: [
						{
							id: 9,
							documentId: "doc-1",
							subject: "documents.events.processor.completed",
							processingVersion: 3,
							occurredAt: "2026-05-09T20:00:00Z",
							createdAt: "2026-05-09T20:00:01Z",
							payload: {
								documentId: "doc-1",
								objectKey: "runbooks/process.md",
								chunkCount: 3,
							},
						},
						{
							id: 8,
							documentId: "doc-1",
							subject: "documents.events.processor.started",
							processingVersion: 3,
							occurredAt: "2026-05-09T19:59:00Z",
							createdAt: "2026-05-09T19:59:01Z",
							payload: {
								documentId: "doc-1",
								objectKey: "runbooks/process.md",
							},
						},
					],
				}),
			});

		render(<App />);
		await user.click(await screen.findByRole("button", { name: /^search$/i }));
		await user.type(
			screen.getByRole("textbox", { name: /search query/i }),
			"processing status",
		);
		await user.click(screen.getByRole("button", { name: /search chunks/i }));

		expect(await screen.findByText("runbooks/process.md")).toBeInTheDocument();
		await user.click(
			screen.getByRole("button", { name: /history for runbooks\/process\.md/i }),
		);

		expect(await screen.findByText("Processing completed")).toBeInTheDocument();
		expect(screen.getByText("Processing started")).toBeInTheDocument();
		expect(screen.getAllByText("v3")).toHaveLength(2);
		expect(screen.getByText(/runbooks\/process\.md.*3 chunks/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/history",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({ documentId: "doc-1", limit: 20 }),
			}),
		);
	});

	test("updates metadata and queues reprocessing for a search result", async () => {
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
					query: "processing status",
					hits: [
						{
							documentId: "doc-1",
							objectKey: "runbooks/process.md",
							contentType: "text/markdown",
							metadata: { tag: "old" },
							chunkId: 12,
							chunkIndex: 0,
							chunkText: "Processor completion is recorded in Postgres.",
							similarity: 0.88,
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					status: "updated",
					documentId: "doc-1",
					metadata: { tag: "runbook" },
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					status: "queued",
					documentId: "doc-1",
					processingVersion: 4,
					sourceUri: "s3://documents/runbooks/process.md",
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					documentId: "doc-1",
					count: 1,
					events: [
						{
							id: 11,
							documentId: "doc-1",
							subject: "documents.events.processor.queued",
							processingVersion: 4,
							occurredAt: "2026-05-10T16:50:00Z",
							createdAt: "2026-05-10T16:50:00Z",
							payload: {
								documentId: "doc-1",
								objectKey: "runbooks/process.md",
							},
						},
					],
				}),
			});

		render(<App />);
		await user.click(await screen.findByRole("button", { name: /^search$/i }));
		await user.type(
			screen.getByRole("textbox", { name: /search query/i }),
			"processing status",
		);
		await user.click(screen.getByRole("button", { name: /search chunks/i }));

		expect(await screen.findByText("runbooks/process.md")).toBeInTheDocument();
		await user.click(
			screen.getByRole("button", { name: /manage runbooks\/process\.md/i }),
		);

		const actionsPanel = screen.getByRole("region", { name: /document actions/i });
		expect(within(actionsPanel).getByText("old")).toBeInTheDocument();
		await user.type(
			within(actionsPanel).getByRole("textbox", { name: /document metadata key/i }),
			"tag",
		);
		await user.type(
			within(actionsPanel).getByRole("textbox", { name: /document metadata value/i }),
			"runbook",
		);
		await user.click(within(actionsPanel).getByRole("button", { name: /update metadata/i }));

		expect(await within(actionsPanel).findByText(/updated metadata for runbooks\/process\.md/i)).toBeInTheDocument();
		expect(within(actionsPanel).getByText("runbook")).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/curation",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					documentId: "doc-1",
					metadata: { tag: "runbook" },
					replace: false,
				}),
			}),
		);

		await user.click(within(actionsPanel).getByRole("button", { name: /queue reprocess/i }));

		expect(await within(actionsPanel).findByText(/queued processing version 4/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/reprocess",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({ documentId: "doc-1" }),
			}),
		);
		expect(await screen.findByText("Processing queued")).toBeInTheDocument();
		expect(screen.getByText(/runbooks\/process\.md version 4 recorded in postgres/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/history",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					documentId: "doc-1",
					processingVersion: 4,
					limit: 20,
				}),
			}),
		);
	});

	test("loads and saves a guarded text edit for a search result", async () => {
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
					query: "processing status",
					hits: [
						{
							documentId: "doc-1",
							objectKey: "runbooks/process.md",
							contentType: "text/markdown",
							metadata: { tag: "runbook" },
							chunkId: 12,
							chunkIndex: 0,
							chunkText: "Processor completion is recorded in Postgres.",
							similarity: 0.88,
						},
					],
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				text: async () => "old process notes",
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					status: "queued",
					documentId: "doc-1",
					processingVersion: 6,
					sourceUri: "s3://documents/runbooks/process.md",
					objectKey: "runbooks/process.md",
				}),
			})
			.mockResolvedValueOnce({
				ok: true,
				json: async () => ({
					documentId: "doc-1",
					count: 1,
					events: [
						{
							id: 12,
							documentId: "doc-1",
							subject: "documents.events.processor.queued",
							processingVersion: 6,
							occurredAt: "2026-05-10T16:51:00Z",
							createdAt: "2026-05-10T16:51:00Z",
							payload: {
								documentId: "doc-1",
								objectKey: "runbooks/process.md",
							},
						},
					],
				}),
			});

		render(<App />);
		await user.click(await screen.findByRole("button", { name: /^search$/i }));
		await user.type(
			screen.getByRole("textbox", { name: /search query/i }),
			"processing status",
		);
		await user.click(screen.getByRole("button", { name: /search chunks/i }));

		expect(await screen.findByText("runbooks/process.md")).toBeInTheDocument();
		await user.click(
			screen.getByRole("button", { name: /manage runbooks\/process\.md/i }),
		);

		const actionsPanel = screen.getByRole("region", { name: /document actions/i });
		await user.click(within(actionsPanel).getByRole("button", { name: /load current text/i }));

		const editTextArea = await within(actionsPanel).findByRole("textbox", {
			name: /editable document text/i,
		});
		expect(editTextArea).toHaveValue("old process notes");

		await user.clear(editTextArea);
		await user.type(editTextArea, "updated process notes");
		await user.click(
			within(actionsPanel).getByRole("checkbox", { name: /confirm raw text overwrite/i }),
		);
		await user.click(within(actionsPanel).getByRole("button", { name: /save text edit/i }));

		expect(await within(actionsPanel).findByText(/queued text edit as processing version 6/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/object?objectKey=runbooks%2Fprocess.md",
		);
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/edit-text",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					documentId: "doc-1",
					text: "updated process notes",
					contentType: "text/markdown",
				}),
			}),
		);
		expect(await screen.findByText("Processing queued")).toBeInTheDocument();
		expect(screen.getByText(/runbooks\/process\.md version 6 recorded in postgres/i)).toBeInTheDocument();
		expect(global.fetch).toHaveBeenCalledWith(
			"/api/documents/history",
			expect.objectContaining({
				method: "POST",
				body: JSON.stringify({
					documentId: "doc-1",
					processingVersion: 6,
					limit: 20,
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
		expect(screen.queryByRole("button", { name: /^inventory$/i })).not.toBeInTheDocument();
		expect(global.fetch).not.toHaveBeenCalledWith(
			"/api/documents/tree",
			expect.objectContaining({ method: "GET" }),
		);
	});

	test("subscribes to document events for authenticated users and shows toast notifications", async () => {
		const originalEventSource = global.EventSource;
		global.EventSource = MockEventSource;

		try {
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

			await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
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
		} finally {
			global.EventSource = originalEventSource;
		}
	});

	test("does not subscribe to document events when the user is not authenticated", async () => {
		const originalEventSource = global.EventSource;
		global.EventSource = MockEventSource;

		try {
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
		} finally {
			global.EventSource = originalEventSource;
		}
	});
});
