import { useEffect, useState } from "react";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";
const THEME_STORAGE_KEY = "homelab-theme";
const AUTH_STATUS_PATH = "/api/auth/status";
const AUTH_PROVIDERS_PATH = "/api/auth/providers";
const DOCUMENTS_TREE_PATH = "/api/documents/tree";
const DOCUMENT_OBJECT_PATH = "/api/documents/object";
const DOCUMENT_UPLOAD_PATH = "/api/documents/upload";

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
	return window.location.hash === "#/documents" ? "documents" : "overview";
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

function App() {
	const [page, setPage] = useState(pageFromHash);
	const [activeRequest, setActiveRequest] = useState(null);
	const [message, setMessage] = useState("Choose a service call to verify routing.");
	const [error, setError] = useState("");
	const [authStatus, setAuthStatus] = useState(null);
	const [authLoading, setAuthLoading] = useState(true);
	const [authError, setAuthError] = useState("");
	const [authProvider, setAuthProvider] = useState(null);
	const [providerLoading, setProviderLoading] = useState(true);
	const [providerError, setProviderError] = useState("");
	const [documentsTree, setDocumentsTree] = useState({ bucket: "documents", prefix: "", breadcrumbs: [], entries: [] });
	const [documentsLoading, setDocumentsLoading] = useState(false);
	const [documentsError, setDocumentsError] = useState("");
	const [currentPrefix, setCurrentPrefix] = useState("");
	const [selectedDocument, setSelectedDocument] = useState(null);
	const [preview, setPreview] = useState({ status: "idle", kind: "empty", text: "", url: "", contentType: "" });
	const [uploadFile, setUploadFile] = useState(null);
	const [uploadName, setUploadName] = useState("");
	const [uploading, setUploading] = useState(false);
	const [uploadMessage, setUploadMessage] = useState("");
	const [uploadError, setUploadError] = useState("");
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
	const documentsEnabled = authStatus?.valid === true;

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
			const googleProvider = response?.providers?.find((provider) => provider.id === "google") ?? null;
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
		if (page !== "documents" || !documentsEnabled) {
			return;
		}
		void loadDocuments(currentPrefix);
	}, [page, currentPrefix, documentsEnabled]);

	const handleAction = async (action) => {
		setActiveRequest(action.id);
		setError("");

		try {
			const response = await callApi(action);
			setMessage(response?.data ?? "Request completed with no response body.");
		} catch (requestError) {
			setError(requestError instanceof Error ? requestError.message : "Unknown request failure");
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

	const navigateToPage = (nextPage) => {
		if (nextPage === "documents" && !documentsEnabled) {
			setPage("documents");
			if (typeof window !== "undefined") {
				window.location.hash = "#/documents";
			}
			return;
		}

		setPage(nextPage);
		if (typeof window === "undefined") {
			return;
		}
		window.location.hash = nextPage === "documents" ? "#/documents" : "#/";
	};

	const handleOpenFolder = (prefix) => {
		setSelectedDocument(null);
		setPreview({ status: "idle", kind: "empty", text: "", url: "", contentType: "" });
		setCurrentPrefix(prefix);
	};

	const handleSelectDocument = async (entry) => {
		setSelectedDocument(entry);
		setPreview({ status: "loading", kind: "loading", text: "", url: "", contentType: entry.contentType ?? "" });

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
				text: requestError instanceof Error ? requestError.message : "Could not load preview",
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

	return (
		<main className="app-shell">
			<section className="hero">
				<p className="eyebrow">Labiraus</p>
				<div className="hero-heading">
					<h1>Verify Labiraus routes from one place.</h1>
					<button
						type="button"
						className="theme-toggle"
						onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
						aria-pressed={theme === "dark"}
					>
						{theme === "dark" ? "Use Light Mode" : "Use Dark Mode"}
					</button>
				</div>
				<p className="intro">
					This frontend doubles as a quick auth console and document browser: it shows what
					the gateway and API believe about your identity before you browse the MinIO-backed
					documents bucket, while the Labiraus MCP server exposes the same storage surface to
					agents.
				</p>
				<div className="page-nav" role="tablist" aria-label="Labiraus pages">
					<button
						type="button"
						className={`page-tab ${page === "overview" ? "active" : ""}`}
						onClick={() => navigateToPage("overview")}
					>
						Overview
					</button>
					<button
						type="button"
						className={`page-tab ${page === "documents" ? "active" : ""}`}
						onClick={() => navigateToPage("documents")}
						aria-disabled={!documentsEnabled}
					>
						Documents
					</button>
				</div>
			</section>

			<section className="panel">
				<div className="panel-header">
					<div>
						<h2>Authentication</h2>
						<p>Inspect the trusted auth state seen by the public Go API.</p>
					</div>
					<span className={`status-badge ${authStatus?.valid ? "success" : "warning"}`}>
						{authLoading ? "Checking auth" : authStatus?.valid ? "Recognized user" : "Not recognized"}
					</span>
				</div>

				<div className="auth-grid">
					<div className="auth-card">
						<p className="response-label">Auth mode</p>
						<p className="auth-value">{authStatus?.mode ?? "loading"}</p>
					</div>
					<div className="auth-card">
						<p className="response-label">Valid user</p>
						<p className="auth-value">{authStatus ? String(authStatus.valid) : "loading"}</p>
					</div>
					<div className="auth-card auth-card-wide">
						<p className="response-label">Authenticated email</p>
						<p className="auth-value">{authStatus?.email || "No email present"}</p>
					</div>
				</div>

				<div className="response-card auth-response" aria-live="polite">
					<p className="response-label">Current auth state</p>
					<p className="response-value">{authSummary(authStatus)}</p>
					{authStatus?.invalidReason ? (
						<p className="error-text">Reason: {authStatus.invalidReason}</p>
					) : null}
					{authError ? <p className="error-text">Status request failed: {authError}</p> : null}
					<p className="response-label">Federated login</p>
					<p className="response-value">{providerSummary(authProvider)}</p>
					{authProvider?.issuer ? (
						<p className="response-value">Issuer: {authProvider.issuer}</p>
					) : null}
					<p className="response-label">MCP access</p>
					<p className="response-value">
						Labiraus also accepts trusted client-certificate access for MCP clients.
					</p>
					{providerError ? (
						<p className="error-text">Provider request failed: {providerError}</p>
					) : null}
				</div>

				<div className="auth-actions">
					<button
						type="button"
						className="action-card primary-action"
						onClick={handleGoogleLogin}
						disabled={providerLoading || !authProvider?.configured}
					>
						<span>{authProvider ? `Log In With ${authProvider.name}` : "Log In With Google"}</span>
						<small>Start the configured federated OIDC flow directly from provider metadata.</small>
					</button>
					<button
						type="button"
						className="action-card"
						onClick={() => {
							void loadAuthStatus();
							void loadAuthProviders();
						}}
						disabled={authLoading || providerLoading}
					>
						<span>Refresh Auth State</span>
						<small>Re-read the current identity and available federated providers.</small>
					</button>
				</div>
			</section>

			{page === "overview" ? (
				<section className="panel">
					<div className="panel-header">
						<div>
							<h2>Service Checks</h2>
							<p>Run a request and inspect the latest application response.</p>
						</div>
						<span className="status-badge">
							{activeRequest ? "Request in progress" : "Idle"}
						</span>
					</div>

					<div className="actions">
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

					<div className="response-card" aria-live="polite">
						<p className="response-label">Latest response</p>
						<p className="response-value">{message}</p>
						{error ? <p className="error-text">Request failed: {error}</p> : null}
					</div>
				</section>
			) : (
				<section className="panel">
					<div className="panel-header">
						<div>
							<h2>Documents</h2>
							<p>Browse the MinIO documents bucket, upload files, preview content, and download originals.</p>
						</div>
						<span className={`status-badge ${documentsEnabled ? "" : "warning"}`}>
							{!documentsEnabled ? "Sign-in required" : documentsLoading ? "Loading" : "Ready"}
						</span>
					</div>
					{!documentsEnabled ? (
						<div className="response-card auth-response" aria-live="polite">
							<p className="response-label">Documents access</p>
							<p className="response-value">
								Sign in with a recognized Labiraus identity before browsing, uploading, or downloading documents.
							</p>
							<p className="response-value">
								The document browser and the matching MCP MinIO surface are both intended for authenticated access.
							</p>
							<button
								type="button"
								className="action-card primary-action"
								onClick={handleGoogleLogin}
								disabled={providerLoading || !authProvider?.configured}
							>
								<span>{authProvider ? `Log In With ${authProvider.name}` : "Log In With Google"}</span>
								<small>Authenticate first, then return here to browse the documents bucket.</small>
							</button>
						</div>
					) : (
						<>
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
										<button type="submit" className="action-card" disabled={uploading}>
											<span>{uploading ? "Uploading..." : "Upload Document"}</span>
											<small>Files are stored in the external MinIO documents bucket.</small>
										</button>
										{uploadMessage ? <p className="success-text">{uploadMessage}</p> : null}
										{uploadError ? <p className="error-text">{uploadError}</p> : null}
									</form>

									<div className="browser-card">
										<div className="browser-header">
											<h3>Folder Contents</h3>
											<button type="button" className="inline-button" onClick={() => void loadDocuments(currentPrefix)}>
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
														<span className="document-time">{formatTimestamp(entry.lastModified)}</span>
													) : (
														<span className="document-time">{entry.type === "folder" ? "Open" : "Preview"}</span>
													)}
												</button>
											))}
										</div>
									</div>
								</div>

								<div className="preview-card">
									<div className="browser-header">
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
										<img className="preview-image" src={preview.url} alt={selectedDocument?.name ?? "Selected document"} />
									) : null}
									{preview.kind === "pdf" ? (
										<iframe className="preview-frame" src={preview.url} title={selectedDocument?.name ?? "Document preview"} />
									) : null}
									{preview.kind === "binary" ? (
										<div className="binary-preview">
											<p>This file type does not render inline yet.</p>
											<p>Use the download link to inspect the original document locally.</p>
										</div>
									) : null}
								</div>
							</div>
						</>
					)}
				</section>
			)}

			<footer className="legal-footer">
				<a href="/privacy-policy.html">Privacy Policy</a>
				<a href="/terms-of-service.html">Terms of Service</a>
			</footer>
		</main>
	);
}

export default App;
