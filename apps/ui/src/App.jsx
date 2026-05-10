import { useEffect, useRef, useState } from "react";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";
const SIGN_OUT_PATH = "/oauth2/sign_out";
const THEME_STORAGE_KEY = "homelab-theme";
const AUTH_STATUS_PATH = "/api/auth/status";
const AUTH_PROVIDERS_PATH = "/api/auth/providers";
const DOCUMENTS_TREE_PATH = "/api/documents/tree";
const DOCUMENT_OBJECT_PATH = "/api/documents/object";
const DOCUMENT_UPLOAD_PATH = "/api/documents/upload";
const DOCUMENT_EVENTS_PATH = "/api/documents/events";
const DOCUMENT_SEARCH_PATH = "/api/documents/search";
const DOCUMENT_CONTEXT_PATH = "/api/documents/context";
const DOCUMENT_HISTORY_PATH = "/api/documents/history";
const TOAST_DISMISS_MS = 6000;

const actions = [
	{
		id: "external",
		label: "User Count via External API",
		method: "GET",
		path: "/api/users/count",
		description: "Fetches the total number of users from Postgres through the public Go service.",
	},
];

function pageFromHash() {
	if (typeof window === "undefined") {
		return "overview";
	}
	switch (window.location.hash) {
		case "#/documents":
			return "documents";
		case "#/search":
			return "search";
		default:
			return "overview";
	}
}

function apiUrl(path) {
	return `${API_BASE_URL}${path}`;
}

async function callApi({ method, path, body }) {
	const response = await fetch(apiUrl(path), {
		method,
		headers: body ? { "Content-Type": "application/json" } : undefined,
		body: body ? JSON.stringify(body) : undefined,
	});

	let payload = null;
	try {
		payload = await response.json();
	} catch {
		payload = null;
	}

	if (!response.ok) {
		const message =
			payload?.error ??
			payload?.message ??
			`${response.status} ${response.statusText}`.trim();
		throw new Error(message || "Request failed");
	}

	return payload;
}

async function fetchAuthStatus() {
	return callApi({ method: "GET", path: AUTH_STATUS_PATH });
}

async function fetchAuthProviders() {
	return callApi({ method: "GET", path: AUTH_PROVIDERS_PATH });
}

async function fetchDocumentsTree(prefix) {
	const params = new URLSearchParams();
	if (prefix) {
		params.set("prefix", prefix);
	}

	return callApi({
		method: "GET",
		path: `${DOCUMENTS_TREE_PATH}${params.size > 0 ? `?${params.toString()}` : ""}`,
	});
}

async function fetchDocumentObject(objectKey) {
	const params = new URLSearchParams({ objectKey });
	const response = await fetch(apiUrl(`${DOCUMENT_OBJECT_PATH}?${params.toString()}`));

	if (!response.ok) {
		let message = `${response.status} ${response.statusText}`.trim();
		try {
			const payload = await response.json();
			message = payload?.error ?? message;
		} catch {
			// Ignore JSON parsing errors for binary responses.
		}
		throw new Error(message || "Request failed");
	}

	return response;
}

async function uploadDocument(file, prefix, objectName) {
	const formData = new FormData();
	formData.append("file", file);

	if (objectName) {
		formData.append("objectKey", joinObjectKey(prefix, objectName));
	} else if (prefix) {
		formData.append("prefix", prefix);
	}

	const response = await fetch(apiUrl(DOCUMENT_UPLOAD_PATH), {
		method: "POST",
		body: formData,
	});

	let payload = null;
	try {
		payload = await response.json();
	} catch {
		payload = null;
	}

	if (!response.ok) {
		throw new Error(payload?.error ?? `${response.status} ${response.statusText}`.trim());
	}

	return payload;
}

async function searchDocuments(request) {
	return callApi({
		method: "POST",
		path: DOCUMENT_SEARCH_PATH,
		body: request,
	});
}

async function fetchDocumentContext(request) {
	return callApi({
		method: "POST",
		path: DOCUMENT_CONTEXT_PATH,
		body: request,
	});
}

async function fetchDocumentHistory(request) {
	return callApi({
		method: "POST",
		path: DOCUMENT_HISTORY_PATH,
		body: request,
	});
}

function authSummary(status) {
	if (!status) {
		return "Authentication status has not been loaded yet.";
	}

	if (status.valid) {
		return `Signed in as ${status.email}.`;
	}

	if (status.mode === "none") {
		return "No authenticated identity is present.";
	}

	return status.invalidReason || "The authenticated identity is not recognized.";
}

function providerSummary(provider) {
	if (!provider) {
		return "Federated login options have not been loaded yet.";
	}

	if (provider.configured) {
		return `Federated sign-in is available through ${provider.name}.`;
	}

	return `${provider.name} sign-in is not configured in the public API yet.`;
}

function authTone(status, loading) {
	if (loading) {
		return "checking";
	}
	return status?.valid ? "success" : "warning";
}

function authLabel(status, loading) {
	if (loading) {
		return "Checking authentication";
	}
	return status?.valid ? "Authenticated" : "Signed out";
}

function authInitial(status) {
	if (status?.email) {
		return status.email[0].toUpperCase();
	}
	return "L";
}

function isTextContentType(contentType) {
	if (!contentType) {
		return false;
	}
	const normalized = contentType.split(";")[0].trim().toLowerCase();
	return normalized.startsWith("text/") || normalized === "application/json";
}

function fileDescription(entry) {
	if (!entry) {
		return "Select a file to preview it here.";
	}

	if (entry.type === "folder") {
		return "Open folders to keep browsing deeper into the documents bucket.";
	}

	return entry.contentType
		? `${entry.contentType} • ${formatBytes(entry.sizeBytes)}`
		: formatBytes(entry.sizeBytes);
}

function formatBytes(value) {
	if (!value) {
		return "0 B";
	}
	const units = ["B", "KB", "MB", "GB"];
	let size = value;
	let unitIndex = 0;
	while (size >= 1024 && unitIndex < units.length - 1) {
		size /= 1024;
		unitIndex += 1;
	}
	return `${size.toFixed(size >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatTimestamp(value) {
	if (!value) {
		return "Unknown time";
	}

	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) {
		return value;
	}

	return parsed.toLocaleString();
}

function joinObjectKey(prefix, name) {
	const cleanName = name.replaceAll("\\", "/").replace(/^\/+/, "");
	return `${prefix ?? ""}${cleanName}`;
}

function documentDownloadUrl(objectKey) {
	const params = new URLSearchParams({ objectKey, download: "1" });
	return apiUrl(`${DOCUMENT_OBJECT_PATH}?${params.toString()}`);
}

function documentOpenUrl(objectKey) {
	const params = new URLSearchParams({ objectKey });
	return apiUrl(`${DOCUMENT_OBJECT_PATH}?${params.toString()}`);
}

function citationLabel(result) {
	return (
		result?.citation?.label ??
		`${result?.objectKey || result?.documentId || "document"} chunk ${result?.chunkIndex ?? 0}`
	);
}

function formatSimilarity(value) {
	if (typeof value !== "number" || Number.isNaN(value)) {
		return "0%";
	}
	return `${Math.round(value * 100)}%`;
}

function searchSubtitle(resultCount, loading) {
	if (loading) {
		return "Searching processed chunks";
	}
	if (resultCount === 1) {
		return "1 matching chunk";
	}
	return `${resultCount} matching chunks`;
}

function lifecycleTone(subject) {
	switch (subject) {
		case "documents.events.processor.completed":
			return "success";
		case "documents.events.processor.failed":
			return "error";
		default:
			return "info";
	}
}

function lifecycleTitle(subject) {
	switch (subject) {
		case "documents.events.minio.stored":
			return "Document stored";
		case "documents.events.processor.queued":
			return "Processing queued";
		case "documents.events.processor.started":
			return "Processing started";
		case "documents.events.processor.completed":
			return "Processing completed";
		case "documents.events.processor.failed":
			return "Processing failed";
		default:
			return "Document updated";
	}
}

function lifecycleDescription(event) {
	const target = event.objectKey || event.documentId || "document";
	switch (event.subject) {
		case "documents.events.processor.failed":
			return `${target}: ${event.error || "processing failed"}`;
		case "documents.events.processor.completed":
			return `${target} is ready for retrieval.`;
		case "documents.events.processor.started":
			return `${target} is being processed now.`;
		case "documents.events.processor.queued":
			return `${target} is waiting for the processor.`;
		case "documents.events.minio.stored":
			return `${target} was written to storage.`;
		default:
			return target;
	}
}

function lifecycleHistorySummary(event) {
	const payload = event?.payload ?? {};
	const target = payload.objectKey || payload.sourceUri || payload.documentId || event?.documentId;
	const details = [];

	if (target) {
		details.push(target);
	}
	if (payload.status) {
		details.push(`status ${payload.status}`);
	}
	if (typeof payload.chunkCount === "number") {
		details.push(`${payload.chunkCount} chunks`);
	}
	if (payload.error) {
		details.push(payload.error);
	}

	return details.join(" • ");
}

function stringifyHistoryPayload(payload) {
	try {
		return JSON.stringify(payload ?? {}, null, 2);
	} catch {
		return "{}";
	}
}

function contextCitationLabel(entry) {
	return entry?.citation?.label ?? entry?.reference ?? "citation";
}

function contextCitationMeta(entry) {
	const citation = entry?.citation ?? {};
	const details = [];

	if (citation.sourceUri || citation.objectKey) {
		details.push(citation.sourceUri || citation.objectKey);
	}
	if (typeof citation.processingVersion === "number") {
		details.push(`v${citation.processingVersion}`);
	}
	if (typeof citation.chunkIndex === "number") {
		details.push(`chunk ${citation.chunkIndex}`);
	}

	return details.join(" • ");
}

function createToastFromLifecycleEvent(event) {
	return {
		id: `${event.subject}-${event.documentId}-${event.occurredAt}`,
		title: lifecycleTitle(event.subject),
		description: lifecycleDescription(event),
		tone: lifecycleTone(event.subject),
	};
}

function App() {
	const [page, setPage] = useState(pageFromHash);
	const [authMenuOpen, setAuthMenuOpen] = useState(false);
	const [activeRequest, setActiveRequest] = useState(null);
	const [message, setMessage] = useState("Choose a service call to verify routing.");
	const [error, setError] = useState("");
	const [authStatus, setAuthStatus] = useState(null);
	const [authLoading, setAuthLoading] = useState(true);
	const [authError, setAuthError] = useState("");
	const [authProvider, setAuthProvider] = useState(null);
	const [providerLoading, setProviderLoading] = useState(true);
	const [providerError, setProviderError] = useState("");
	const [documentsTree, setDocumentsTree] = useState({
		bucket: "documents",
		prefix: "",
		breadcrumbs: [],
		entries: [],
	});
	const [documentsLoading, setDocumentsLoading] = useState(false);
	const [documentsError, setDocumentsError] = useState("");
	const [currentPrefix, setCurrentPrefix] = useState("");
	const [selectedDocument, setSelectedDocument] = useState(null);
	const [preview, setPreview] = useState({
		status: "idle",
		kind: "empty",
		text: "",
		url: "",
		contentType: "",
	});
	const [uploadFile, setUploadFile] = useState(null);
	const [uploadName, setUploadName] = useState("");
	const [uploading, setUploading] = useState(false);
	const [uploadMessage, setUploadMessage] = useState("");
	const [uploadError, setUploadError] = useState("");
	const [searchQuery, setSearchQuery] = useState("");
	const [searchPrefix, setSearchPrefix] = useState("");
	const [searchMetadataKey, setSearchMetadataKey] = useState("");
	const [searchMetadataValue, setSearchMetadataValue] = useState("");
	const [searchLoading, setSearchLoading] = useState(false);
	const [searchError, setSearchError] = useState("");
	const [searchResults, setSearchResults] = useState([]);
	const [submittedQuery, setSubmittedQuery] = useState("");
	const [contextState, setContextState] = useState({
		status: "idle",
		query: "",
		context: "",
		citations: [],
		hits: [],
		maxChars: 0,
		truncated: false,
		error: "",
	});
	const [historyState, setHistoryState] = useState({
		status: "idle",
		documentId: "",
		sourceLabel: "",
		events: [],
		error: "",
	});
	const [toasts, setToasts] = useState([]);
	const [theme, setTheme] = useState(() => {
		if (typeof window === "undefined") {
			return "light";
		}

		const storedTheme = window.localStorage.getItem(THEME_STORAGE_KEY);
		if (storedTheme === "light" || storedTheme === "dark") {
			return storedTheme;
		}

		return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
	});
	const authMenuRef = useRef(null);
	const toastTimersRef = useRef(new Map());
	const documentsEnabled = authStatus?.valid === true;

	const dismissToast = (toastId) => {
		const timer = toastTimersRef.current.get(toastId);
		if (timer) {
			window.clearTimeout(timer);
			toastTimersRef.current.delete(toastId);
		}
		setToasts((current) => current.filter((toast) => toast.id !== toastId));
	};

	const pushToast = (toast) => {
		setToasts((current) => {
			const next = [...current.filter((item) => item.id !== toast.id), toast];
			return next.slice(-4);
		});
		const existingTimer = toastTimersRef.current.get(toast.id);
		if (existingTimer) {
			window.clearTimeout(existingTimer);
		}
		const timer = window.setTimeout(() => {
			toastTimersRef.current.delete(toast.id);
			setToasts((current) => current.filter((item) => item.id !== toast.id));
		}, TOAST_DISMISS_MS);
		toastTimersRef.current.set(toast.id, timer);
	};

	useEffect(() => {
		document.documentElement.dataset.theme = theme;
		window.localStorage.setItem(THEME_STORAGE_KEY, theme);
	}, [theme]);

	useEffect(() => {
		if (typeof window === "undefined") {
			return undefined;
		}

		const onHashChange = () => {
			setPage(pageFromHash());
		};

		window.addEventListener("hashchange", onHashChange);
		return () => window.removeEventListener("hashchange", onHashChange);
	}, []);

	useEffect(() => {
		if (!preview.url) {
			return undefined;
		}

		return () => {
			URL.revokeObjectURL(preview.url);
		};
	}, [preview.url]);

	useEffect(() => {
		return () => {
			for (const timer of toastTimersRef.current.values()) {
				window.clearTimeout(timer);
			}
			toastTimersRef.current.clear();
		};
	}, []);

	useEffect(() => {
		if (!authMenuOpen) {
			return undefined;
		}

		const handlePointerDown = (event) => {
			if (authMenuRef.current && !authMenuRef.current.contains(event.target)) {
				setAuthMenuOpen(false);
			}
		};

		const handleKeyDown = (event) => {
			if (event.key === "Escape") {
				setAuthMenuOpen(false);
			}
		};

		document.addEventListener("mousedown", handlePointerDown);
		document.addEventListener("keydown", handleKeyDown);
		return () => {
			document.removeEventListener("mousedown", handlePointerDown);
			document.removeEventListener("keydown", handleKeyDown);
		};
	}, [authMenuOpen]);

	const loadAuthStatus = async () => {
		setAuthLoading(true);
		setAuthError("");

		try {
			const status = await fetchAuthStatus();
			setAuthStatus(status);
		} catch (requestError) {
			setAuthError(
				requestError instanceof Error ? requestError.message : "Could not load auth status",
			);
		} finally {
			setAuthLoading(false);
		}
	};

	const loadAuthProviders = async () => {
		setProviderLoading(true);
		setProviderError("");

		try {
			const response = await fetchAuthProviders();
			const googleProvider =
				response?.providers?.find((provider) => provider.id === "google") ?? null;
			setAuthProvider(googleProvider);
		} catch (requestError) {
			setProviderError(
				requestError instanceof Error ? requestError.message : "Could not load auth providers",
			);
		} finally {
			setProviderLoading(false);
		}
	};

	const loadDocuments = async (prefix) => {
		setDocumentsLoading(true);
		setDocumentsError("");

		try {
			const response = await fetchDocumentsTree(prefix);
			setDocumentsTree(response);
		} catch (requestError) {
			setDocumentsError(
				requestError instanceof Error ? requestError.message : "Could not load documents",
			);
		} finally {
			setDocumentsLoading(false);
		}
	};

	useEffect(() => {
		void loadAuthStatus();
		void loadAuthProviders();
	}, []);

	useEffect(() => {
		if (!documentsEnabled && (page === "documents" || page === "search")) {
			setPage("overview");
			if (typeof window !== "undefined") {
				window.location.hash = "#/";
			}
		}
	}, [documentsEnabled, page]);

	useEffect(() => {
		if (page !== "documents" || !documentsEnabled) {
			return;
		}
		void loadDocuments(currentPrefix);
	}, [page, currentPrefix, documentsEnabled]);

	useEffect(() => {
		if (!documentsEnabled || typeof window === "undefined" || typeof EventSource === "undefined") {
			return undefined;
		}

		const stream = new EventSource(apiUrl(DOCUMENT_EVENTS_PATH));
		const handleDocumentEvent = (message) => {
			try {
				const event = JSON.parse(message.data);
				pushToast(createToastFromLifecycleEvent(event));
			} catch {
				// Ignore malformed events so the stream stays alive.
			}
		};

		stream.addEventListener("document", handleDocumentEvent);
		return () => {
			stream.removeEventListener("document", handleDocumentEvent);
			stream.close();
		};
	}, [documentsEnabled]);

	const handleAction = async (action) => {
		setActiveRequest(action.id);
		setError("");

		try {
			const response = await callApi(action);
			setMessage(response?.data ?? "Request completed with no response body.");
		} catch (requestError) {
			setError(
				requestError instanceof Error ? requestError.message : "Unknown request failure",
			);
		} finally {
			setActiveRequest(null);
		}
	};

	const handleGoogleLogin = () => {
		if (!authProvider?.authorizationUrl) {
			return;
		}

		window.location.assign(authProvider.authorizationUrl);
	};

	const handleSignOut = () => {
		window.location.assign(SIGN_OUT_PATH);
	};

	const navigateToPage = (nextPage) => {
		setPage(nextPage);
		setAuthMenuOpen(false);
		if (typeof window === "undefined") {
			return;
		}
		window.location.hash =
			nextPage === "documents" ? "#/documents" : nextPage === "search" ? "#/search" : "#/";
	};

	const handleOpenFolder = (prefix) => {
		setSelectedDocument(null);
		setPreview({ status: "idle", kind: "empty", text: "", url: "", contentType: "" });
		setCurrentPrefix(prefix);
	};

	const handleSelectDocument = async (entry) => {
		setSelectedDocument(entry);
		setPreview({
			status: "loading",
			kind: "loading",
			text: "",
			url: "",
			contentType: entry.contentType ?? "",
		});

		try {
			const response = await fetchDocumentObject(entry.objectKey);
			const contentType = response.headers.get("Content-Type") ?? entry.contentType ?? "";
			const blob = await response.blob();

			if (isTextContentType(contentType)) {
				const text = await blob.text();
				setPreview({ status: "ready", kind: "text", text, url: "", contentType });
				return;
			}

			const url = URL.createObjectURL(blob);
			if (contentType.startsWith("image/")) {
				setPreview({ status: "ready", kind: "image", text: "", url, contentType });
				return;
			}
			if (contentType === "application/pdf") {
				setPreview({ status: "ready", kind: "pdf", text: "", url, contentType });
				return;
			}

			setPreview({ status: "ready", kind: "binary", text: "", url, contentType });
		} catch (requestError) {
			setPreview({
				status: "error",
				kind: "error",
				text:
					requestError instanceof Error ? requestError.message : "Could not load preview",
				url: "",
				contentType: entry.contentType ?? "",
			});
		}
	};

	const handleUploadSubmit = async (event) => {
		event.preventDefault();

		if (!uploadFile) {
			setUploadError("Choose a file before uploading.");
			return;
		}

		setUploading(true);
		setUploadError("");
		setUploadMessage("");

		try {
			const response = await uploadDocument(uploadFile, currentPrefix, uploadName.trim());
			setUploadMessage(`Uploaded ${response.objectKey}.`);
			setUploadFile(null);
			setUploadName("");
			const fileInput = event.currentTarget.querySelector('input[type="file"]');
			if (fileInput) {
				fileInput.value = "";
			}
			await loadDocuments(currentPrefix);
		} catch (requestError) {
			setUploadError(requestError instanceof Error ? requestError.message : "Upload failed");
		} finally {
			setUploading(false);
		}
	};

	const handleSearchSubmit = async (event) => {
		event.preventDefault();
		const query = searchQuery.trim();
		if (!query) {
			setSearchError("Enter a search phrase first.");
			return;
		}

		setSearchLoading(true);
		setSearchError("");
		setHistoryState({
			status: "idle",
			documentId: "",
			sourceLabel: "",
			events: [],
			error: "",
		});
		setContextState({
			status: "idle",
			query: "",
			context: "",
			citations: [],
			hits: [],
			maxChars: 0,
			truncated: false,
			error: "",
		});

		try {
			const metadataKey = searchMetadataKey.trim();
			const metadataValue = searchMetadataValue.trim();
			const metadata =
				metadataKey && metadataValue ? { [metadataKey]: metadataValue } : undefined;
			const response = await searchDocuments({
				query,
				prefix: searchPrefix.trim(),
				...(metadata ? { metadata } : {}),
				limit: 8,
			});
			setSubmittedQuery(query);
			setSearchResults(response?.hits ?? []);
		} catch (requestError) {
			setSearchError(requestError instanceof Error ? requestError.message : "Search failed");
			setSearchResults([]);
		} finally {
			setSearchLoading(false);
		}
	};

	const handleContextSubmit = async () => {
		const query = searchQuery.trim();
		if (!query) {
			setContextState({
				status: "error",
				query: "",
				context: "",
				citations: [],
				hits: [],
				maxChars: 0,
				truncated: false,
				error: "Enter a search phrase first.",
			});
			return;
		}

		setContextState({
			status: "loading",
			query,
			context: "",
			citations: [],
			hits: [],
			maxChars: 0,
			truncated: false,
			error: "",
		});

		try {
			const metadataKey = searchMetadataKey.trim();
			const metadataValue = searchMetadataValue.trim();
			const metadata =
				metadataKey && metadataValue ? { [metadataKey]: metadataValue } : undefined;
			const response = await fetchDocumentContext({
				query,
				prefix: searchPrefix.trim(),
				...(metadata ? { metadata } : {}),
				limit: 6,
				maxChars: 6000,
			});

			setContextState({
				status: "ready",
				query,
				context: response?.context ?? "",
				citations: response?.citations ?? [],
				hits: response?.hits ?? [],
				maxChars: response?.maxChars ?? 0,
				truncated: response?.truncated === true,
				error: "",
			});
		} catch (requestError) {
			setContextState({
				status: "error",
				query,
				context: "",
				citations: [],
				hits: [],
				maxChars: 0,
				truncated: false,
				error:
					requestError instanceof Error ? requestError.message : "Context request failed",
			});
		}
	};

	const handleLoadHistory = async (result) => {
		const documentId = result?.documentId?.trim();
		if (!documentId) {
			return;
		}

		const sourceLabel = result.objectKey || documentId;
		setHistoryState({
			status: "loading",
			documentId,
			sourceLabel,
			events: [],
			error: "",
		});

		try {
			const response = await fetchDocumentHistory({ documentId, limit: 20 });
			setHistoryState({
				status: "ready",
				documentId,
				sourceLabel,
				events: response?.events ?? [],
				error: "",
			});
		} catch (requestError) {
			setHistoryState({
				status: "error",
				documentId,
				sourceLabel,
				events: [],
				error: requestError instanceof Error ? requestError.message : "History request failed",
			});
		}
	};

	return (
		<main className="app-shell">
			<div className="toast-stack" aria-live="polite" aria-label="Document notifications">
				{toasts.map((toast) => (
					<article key={toast.id} className={`toast-card ${toast.tone}`}>
						<div>
							<p className="toast-title">{toast.title}</p>
							<p className="toast-copy">{toast.description}</p>
						</div>
						<button
							type="button"
							className="toast-dismiss"
							aria-label={`Dismiss ${toast.title}`}
							onClick={() => dismissToast(toast.id)}
						>
							Close
						</button>
					</article>
				))}
			</div>

			<header className="topbar">
				<div className="topbar-brand">
					<div className="brand-mark" aria-hidden="true">
						L
					</div>
					<div className="brand-copy">
						<p className="brand-label">Labiraus</p>
						<p className="brand-subtitle">Authenticated storage and control plane</p>
					</div>
				</div>

				<nav className="topbar-nav" aria-label="Primary">
					<button
						type="button"
						className={`topbar-tab ${page === "overview" ? "active" : ""}`}
						onClick={() => navigateToPage("overview")}
					>
						Overview
					</button>
					{documentsEnabled ? (
						<button
							type="button"
							className={`topbar-tab ${page === "search" ? "active" : ""}`}
							onClick={() => navigateToPage("search")}
						>
							Search
						</button>
					) : null}
					{documentsEnabled ? (
						<button
							type="button"
							className={`topbar-tab ${page === "documents" ? "active" : ""}`}
							onClick={() => navigateToPage("documents")}
						>
							Documents
						</button>
					) : null}
				</nav>

				<div className="topbar-actions">
					<div className="auth-menu" ref={authMenuRef}>
						<button
							type="button"
							className="auth-toggle"
							aria-label="Authentication menu"
							aria-expanded={authMenuOpen}
							onClick={() => setAuthMenuOpen((value) => !value)}
						>
							<span
								className={`status-dot ${authTone(authStatus, authLoading)}`}
								aria-hidden="true"
							/>
							<span className="auth-toggle-label">{authLabel(authStatus, authLoading)}</span>
							<span className="auth-avatar" aria-hidden="true">
								{authInitial(authStatus)}
							</span>
						</button>

						{authMenuOpen ? (
							<div className="auth-popover">
								<div className="auth-popover-header">
									<div>
										<p className="popover-title">Authentication</p>
										<p className="popover-subtitle">{authSummary(authStatus)}</p>
									</div>
									<button
										type="button"
										className="theme-toggle"
										onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
										aria-pressed={theme === "dark"}
									>
										{theme === "dark" ? "Use Light Mode" : "Use Dark Mode"}
									</button>
								</div>

								<div className="auth-meta-grid">
									<div className="auth-meta-card">
										<p className="meta-label">Mode</p>
										<p className="meta-value">{authStatus?.mode ?? "loading"}</p>
									</div>
									<div className="auth-meta-card">
										<p className="meta-label">Valid user</p>
										<p className="meta-value">
											{authStatus ? String(authStatus.valid) : "loading"}
										</p>
									</div>
									<div className="auth-meta-card auth-meta-wide">
										<p className="meta-label">Authenticated email</p>
										<p className="meta-value">
											{authStatus?.email || "No email present"}
										</p>
									</div>
								</div>

								<p className="auth-supporting-copy">{providerSummary(authProvider)}</p>
								<p className="auth-supporting-copy">
									Labiraus MCP access also accepts trusted client-certificate
									authentication.
								</p>

								{authStatus?.invalidReason ? (
									<p className="error-text">Reason: {authStatus.invalidReason}</p>
								) : null}
								{authError ? (
									<p className="error-text">Status request failed: {authError}</p>
								) : null}
								{providerError ? (
									<p className="error-text">Provider request failed: {providerError}</p>
								) : null}

								<div className="auth-menu-actions">
									{documentsEnabled ? (
										<button
											type="button"
											className="menu-action danger"
											onClick={handleSignOut}
										>
											Sign Out
										</button>
									) : (
										<button
											type="button"
											className="menu-action primary"
											onClick={handleGoogleLogin}
											disabled={providerLoading || !authProvider?.configured}
										>
											{authProvider ? `Sign In With ${authProvider.name}` : "Sign In"}
										</button>
									)}
									<button
										type="button"
										className="menu-action"
										onClick={() => {
											void loadAuthStatus();
											void loadAuthProviders();
										}}
										disabled={authLoading || providerLoading}
									>
										Refresh Auth
									</button>
								</div>
							</div>
						) : null}
					</div>
				</div>
			</header>

			<div className="workspace">
				{page === "overview" ? (
					<section className="workspace-shell">
						<header className="workspace-header">
							<div>
								<p className="workspace-eyebrow">Overview</p>
								<h1>Inspect the deployed Labiraus surface.</h1>
								<p className="workspace-intro">
									Use the same browser entrypoint your operators see to confirm
									identity, route wiring, and the public API behavior that sits
									alongside the MCP server.
								</p>
							</div>
							<div className="header-badge-card">
								<p className="meta-label">Current auth state</p>
								<p className="header-badge-value">{authLabel(authStatus, authLoading)}</p>
								<p className="header-badge-copy">{authSummary(authStatus)}</p>
							</div>
						</header>

						<div className="summary-grid">
							<div className="summary-card">
								<p className="meta-label">Browser identity</p>
								<p className="summary-value">
									{authStatus?.email || "Authenticate to unlock document browsing"}
								</p>
								<p className="summary-copy">
									The `ui` and `external` surfaces sit behind the shared browser
									login path on `mcp.labiraus.com`.
								</p>
							</div>
							<div className="summary-card">
								<p className="meta-label">Documents tab</p>
								<p className="summary-value">
									{documentsEnabled ? "Available" : "Hidden until sign-in"}
								</p>
								<p className="summary-copy">
									The MinIO-backed browser stays in the header only for recognized
									users.
								</p>
							</div>
							<div className="summary-card">
								<p className="meta-label">MCP access</p>
								<p className="summary-value">Google bearer or client certificate</p>
								<p className="summary-copy">
									The same deployment also supports authenticated MCP clients with
									direct storage access paths.
								</p>
							</div>
						</div>

						<div className="content-grid">
							<section className="content-panel">
								<div className="panel-heading">
									<div>
										<h2>Service checks</h2>
										<p>Run a routed request and inspect the latest result.</p>
									</div>
									<span className={`status-pill ${activeRequest ? "busy" : ""}`}>
										{activeRequest ? "Request in progress" : "Idle"}
									</span>
								</div>

								<div className="actions-grid">
									{actions.map((action) => (
										<button
											key={action.id}
											type="button"
											className="action-card"
											onClick={() => handleAction(action)}
											disabled={Boolean(activeRequest)}
										>
											<span>{action.label}</span>
											<small>{action.description}</small>
										</button>
									))}
								</div>
							</section>

							<section className="response-panel" aria-live="polite">
								<p className="meta-label">Latest response</p>
								<p className="response-value">{message}</p>
								{error ? <p className="error-text">Request failed: {error}</p> : null}
							</section>
						</div>
					</section>
				) : page === "search" ? (
					<section className="workspace-shell">
						<header className="workspace-header">
							<div>
								<p className="workspace-eyebrow">Search</p>
								<h1>Search processed document chunks.</h1>
								<p className="workspace-intro">
									Enter a natural-language query, optionally narrow it to a folder
									prefix, and inspect the nearest matching chunks stored in Postgres.
								</p>
							</div>
							<div className="header-badge-card">
								<p className="meta-label">Retrieval status</p>
								<p className="header-badge-value">
									{searchSubtitle(searchResults.length, searchLoading)}
								</p>
								<p className="header-badge-copy">
									Results come from pgvector similarity search over processed
									document chunks.
								</p>
							</div>
						</header>

						<div className="search-layout">
							<form className="search-card" onSubmit={handleSearchSubmit}>
								<div className="panel-heading compact">
									<div>
										<h3>Semantic query</h3>
										<p>Use the same embedding model family as ingestion.</p>
									</div>
								</div>

								<label className="field-group">
									<span>Search query</span>
									<textarea
										value={searchQuery}
										onChange={(event) => setSearchQuery(event.target.value)}
										placeholder="How do we refresh kubeconfig for the cluster?"
										rows={4}
									/>
								</label>

								<label className="field-group">
									<span>Optional prefix filter</span>
									<input
										type="text"
										value={searchPrefix}
										onChange={(event) => setSearchPrefix(event.target.value)}
										placeholder="scripts/ or docs/"
									/>
								</label>

								<div className="metadata-filter-grid">
									<label className="field-group">
										<span>Metadata key</span>
										<input
											type="text"
											value={searchMetadataKey}
											onChange={(event) => setSearchMetadataKey(event.target.value)}
											placeholder="tag"
										/>
									</label>
									<label className="field-group">
										<span>Metadata value</span>
										<input
											type="text"
											value={searchMetadataValue}
											onChange={(event) => setSearchMetadataValue(event.target.value)}
											placeholder="runbook"
										/>
									</label>
								</div>

								<div className="search-actions">
									<button
										type="submit"
										className="menu-action primary"
										disabled={searchLoading || contextState.status === "loading"}
									>
										{searchLoading ? "Searching..." : "Search Chunks"}
									</button>
									<button
										type="button"
										className="menu-action"
										onClick={() => void handleContextSubmit()}
										disabled={searchLoading || contextState.status === "loading"}
									>
										{contextState.status === "loading" ? "Assembling..." : "Assemble Context"}
									</button>
									<button
										type="button"
										className="menu-action"
										onClick={() => {
											setSearchQuery("");
											setSearchPrefix("");
											setSearchMetadataKey("");
											setSearchMetadataValue("");
											setSearchResults([]);
											setSearchError("");
											setSubmittedQuery("");
											setContextState({
												status: "idle",
												query: "",
												context: "",
												citations: [],
												hits: [],
												maxChars: 0,
												truncated: false,
												error: "",
											});
											setHistoryState({
												status: "idle",
												documentId: "",
												sourceLabel: "",
												events: [],
												error: "",
											});
										}}
										disabled={searchLoading || contextState.status === "loading"}
									>
										Clear
									</button>
								</div>

								{searchError ? <p className="error-text">{searchError}</p> : null}
							</form>

							<div className="search-results-column">
								<div className="search-results-card">
									<div className="panel-heading compact">
										<div>
											<h3>Results</h3>
											<p>
												{submittedQuery
													? `Ranked matches for "${submittedQuery}".`
													: "Run a query to inspect matching chunks."}
											</p>
										</div>
									</div>

									{!searchLoading && searchResults.length === 0 && !searchError ? (
										<p className="empty-state">
											{submittedQuery
												? "No processed chunks matched this query yet."
												: "Search results will appear here."}
										</p>
									) : null}

									<div className="search-result-list">
										{searchResults.map((result) => (
											<article
												key={`${result.documentId}-${result.chunkId}`}
												className="search-result-card"
											>
												<div className="search-result-header">
													<div>
														<p className="search-result-title">{result.objectKey || result.documentId}</p>
														<p className="search-result-meta">
															{result.contentType || "Unknown type"} • chunk {result.chunkIndex}
														</p>
													</div>
													<div className="search-score-pill">
														{formatSimilarity(result.similarity)}
													</div>
												</div>

												<p className="search-result-text">{result.chunkText}</p>

												<div className="search-result-footer">
													<div className="citation-block">
														<p className="search-result-meta">
															Citation: <span className="citation-code">{citationLabel(result)}</span>
														</p>
														<p className="search-result-meta">
															Document ID: {result.documentId}
															{result.lastProcessedAt
																? ` • Indexed ${formatTimestamp(result.lastProcessedAt)}`
																: ""}
														</p>
													</div>
													<div className="search-result-actions">
														<button
															type="button"
															className="inline-button"
															onClick={() => void handleLoadHistory(result)}
															disabled={
																historyState.status === "loading" &&
																historyState.documentId === result.documentId
															}
															aria-label={`History for ${result.objectKey || result.documentId}`}
														>
															History
														</button>
														{result.objectKey ? (
															<a className="inline-button" href={documentOpenUrl(result.objectKey)}>
																Open Source
															</a>
														) : null}
														{result.objectKey ? (
															<a className="inline-button" href={documentDownloadUrl(result.objectKey)}>
																Download
															</a>
														) : null}
													</div>
												</div>
											</article>
										))}
									</div>
								</div>

								<section className="context-card" aria-live="polite">
									<div className="panel-heading compact">
										<div>
											<h3>Context block</h3>
											<p>
												{contextState.query
													? `Cited context for "${contextState.query}".`
													: "Assembled context will appear here."}
											</p>
										</div>
										{contextState.status === "loading" ? (
											<span className="status-pill busy">Assembling</span>
										) : contextState.status === "ready" ? (
											<span className="status-pill">Ready</span>
										) : null}
									</div>

									{contextState.error ? <p className="error-text">{contextState.error}</p> : null}
									{contextState.status === "idle" ? (
										<p className="empty-state">Context output will appear here.</p>
									) : null}
									{contextState.status === "loading" ? (
										<p className="empty-state">Assembling cited context...</p>
									) : null}
									{contextState.status === "ready" && !contextState.context ? (
										<p className="empty-state">No context could be assembled from the current filters.</p>
									) : null}
									{contextState.context ? (
										<pre className="context-output">{contextState.context}</pre>
									) : null}
									{contextState.truncated ? (
										<p className="search-result-meta">
											Truncated at {contextState.maxChars} characters.
										</p>
									) : null}
									{contextState.citations.length > 0 ? (
										<div className="context-citations">
											<p className="meta-label">Citations</p>
											<ol>
												{contextState.citations.map((entry) => (
													<li key={`${entry.reference}-${contextCitationLabel(entry)}`}>
														<span className="citation-code">{entry.reference}</span>
														<span>{contextCitationLabel(entry)}</span>
														{contextCitationMeta(entry) ? (
															<small>{contextCitationMeta(entry)}</small>
														) : null}
													</li>
												))}
											</ol>
										</div>
									) : null}
								</section>

								<section className="history-card" aria-live="polite">
									<div className="panel-heading compact">
										<div>
											<h3>Lifecycle history</h3>
											<p>
												{historyState.documentId
													? `${historyState.sourceLabel} recorded in Postgres.`
													: "Select a search result to inspect durable processing events."}
											</p>
										</div>
										{historyState.status === "loading" ? (
											<span className="status-pill busy">Loading</span>
										) : historyState.status === "ready" ? (
											<span className="status-pill">Loaded</span>
										) : null}
									</div>

									{historyState.error ? <p className="error-text">{historyState.error}</p> : null}
									{historyState.status === "idle" ? (
										<p className="empty-state">Lifecycle events will appear here.</p>
									) : null}
									{historyState.status === "loading" ? (
										<p className="empty-state">Loading lifecycle events…</p>
									) : null}
									{historyState.status === "ready" && historyState.events.length === 0 ? (
										<p className="empty-state">No lifecycle events are recorded for this document yet.</p>
									) : null}

									{historyState.events.length > 0 ? (
										<ol className="history-event-list">
											{historyState.events.map((event) => (
												<li key={event.id} className="history-event">
													<span
														className={`history-event-marker ${lifecycleTone(event.subject)}`}
														aria-hidden="true"
													/>
													<div className="history-event-body">
														<div className="history-event-header">
															<p className="history-event-title">{lifecycleTitle(event.subject)}</p>
															<span className="history-version-pill">
																v{event.processingVersion}
															</span>
														</div>
														<p className="search-result-meta">
															{formatTimestamp(event.occurredAt)}
															{event.createdAt
																? ` • recorded ${formatTimestamp(event.createdAt)}`
																: ""}
														</p>
														{lifecycleHistorySummary(event) ? (
															<p className="history-event-summary">
																{lifecycleHistorySummary(event)}
															</p>
														) : null}
														<details className="history-payload">
															<summary>Payload</summary>
															<pre>{stringifyHistoryPayload(event.payload)}</pre>
														</details>
													</div>
												</li>
											))}
										</ol>
									) : null}
								</section>
							</div>
						</div>
					</section>
				) : (
					<section className="workspace-shell">
						<header className="workspace-header">
							<div>
								<p className="workspace-eyebrow">Documents</p>
								<h1>Browse the documents bucket.</h1>
								<p className="workspace-intro">
									Navigate folders, upload new documents, preview inline-supported
									types, and download originals from the same authenticated surface.
								</p>
							</div>
							<div className="header-badge-card">
								<p className="meta-label">Documents access</p>
								<p className="header-badge-value">
									{documentsLoading ? "Loading" : "Ready"}
								</p>
								<p className="header-badge-copy">
									Authenticated users can browse the private MinIO-backed storage
									surface.
								</p>
							</div>
						</header>

						<div className="breadcrumbs" aria-label="Document path breadcrumbs">
							{documentsTree.breadcrumbs.map((crumb) => (
								<button
									key={`${crumb.name}-${crumb.prefix}`}
									type="button"
									className={`breadcrumb ${crumb.prefix === currentPrefix ? "active" : ""}`}
									onClick={() => handleOpenFolder(crumb.prefix)}
								>
									{crumb.name}
								</button>
							))}
						</div>

						<div className="documents-layout">
							<div className="documents-column">
								<form className="upload-card" onSubmit={handleUploadSubmit}>
									<div>
										<h3>Upload</h3>
										<p>Upload into {currentPrefix || "the root documents folder"}.</p>
									</div>
									<input
										type="file"
										onChange={(event) => setUploadFile(event.target.files?.[0] ?? null)}
									/>
									<input
										type="text"
										value={uploadName}
										onChange={(event) => setUploadName(event.target.value)}
										placeholder="Optional file name override"
									/>
									<button type="submit" className="menu-action primary" disabled={uploading}>
										{uploading ? "Uploading..." : "Upload Document"}
									</button>
									{uploadMessage ? <p className="success-text">{uploadMessage}</p> : null}
									{uploadError ? <p className="error-text">{uploadError}</p> : null}
								</form>

								<div className="browser-card">
									<div className="panel-heading compact">
										<div>
											<h3>Folder contents</h3>
											<p>Immediate folders and files at the current prefix.</p>
										</div>
										<button
											type="button"
											className="inline-button"
											onClick={() => void loadDocuments(currentPrefix)}
										>
											Refresh
										</button>
									</div>
									{documentsError ? <p className="error-text">{documentsError}</p> : null}
									{documentsTree.entries.length === 0 && !documentsLoading ? (
										<p className="empty-state">No folders or files are present here yet.</p>
									) : null}
									<div className="document-list" role="list">
										{documentsTree.entries.map((entry) => (
											<button
												key={entry.prefix || entry.objectKey}
												type="button"
												role="listitem"
												className={`document-row ${selectedDocument?.objectKey === entry.objectKey ? "active" : ""}`}
												onClick={() =>
													entry.type === "folder"
														? handleOpenFolder(entry.prefix)
														: void handleSelectDocument(entry)
												}
											>
												<div>
													<p className="document-name">
														{entry.type === "folder" ? "Folder" : "File"}: {entry.name}
													</p>
													<p className="document-meta">
														{entry.type === "folder"
															? entry.prefix
															: `${entry.contentType || "Unknown type"} • ${formatBytes(entry.sizeBytes)}`}
													</p>
												</div>
												{entry.type === "file" && entry.lastModified ? (
													<span className="document-time">
														{formatTimestamp(entry.lastModified)}
													</span>
												) : (
													<span className="document-time">
														{entry.type === "folder" ? "Open" : "Preview"}
													</span>
												)}
											</button>
										))}
									</div>
								</div>
							</div>

							<div className="preview-card">
								<div className="panel-heading compact">
									<div>
										<h3>Preview</h3>
										<p>{fileDescription(selectedDocument)}</p>
									</div>
									{selectedDocument?.objectKey ? (
										<a className="inline-button" href={documentDownloadUrl(selectedDocument.objectKey)}>
											Download
										</a>
									) : null}
								</div>

								{preview.status === "loading" ? <p className="empty-state">Loading preview…</p> : null}
								{preview.status === "error" ? <p className="error-text">{preview.text}</p> : null}
								{preview.kind === "empty" ? (
									<p className="empty-state">Choose a file from the browser to inspect it here.</p>
								) : null}
								{preview.kind === "text" ? <pre className="preview-text">{preview.text}</pre> : null}
								{preview.kind === "image" ? (
									<img
										className="preview-image"
										src={preview.url}
										alt={selectedDocument?.name ?? "Selected document"}
									/>
								) : null}
								{preview.kind === "pdf" ? (
									<iframe
										className="preview-frame"
										src={preview.url}
										title={selectedDocument?.name ?? "Document preview"}
									/>
								) : null}
								{preview.kind === "binary" ? (
									<div className="binary-preview">
										<p>This file type does not render inline yet.</p>
										<p>Use the download link to inspect the original document locally.</p>
									</div>
								) : null}
							</div>
						</div>
					</section>
				)}
			</div>

			<footer className="legal-footer">
				<a href="/privacy-policy.html">Privacy Policy</a>
				<a href="/terms-of-service.html">Terms of Service</a>
			</footer>
		</main>
	);
}

export default App;
