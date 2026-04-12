import { useEffect, useState } from "react";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";
const THEME_STORAGE_KEY = "homelab-theme";
const AUTH_STATUS_PATH = "/api/auth/status";
const AUTH_PROVIDERS_PATH = "/api/auth/providers";

const actions = [
	{
		id: "external",
		label: "User Count via External API",
		method: "GET",
		path: "/api/users/count",
		description: "Fetches the total number of users from Postgres through the public Go service.",
	},
];

async function callApi({ method, path, body }) {
	const response = await fetch(`${API_BASE_URL}${path}`, {
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

function App() {
	const [activeRequest, setActiveRequest] = useState(null);
	const [message, setMessage] = useState("Choose a service call to verify routing.");
	const [error, setError] = useState("");
	const [authStatus, setAuthStatus] = useState(null);
	const [authLoading, setAuthLoading] = useState(true);
	const [authError, setAuthError] = useState("");
	const [authProvider, setAuthProvider] = useState(null);
	const [providerLoading, setProviderLoading] = useState(true);
	const [providerError, setProviderError] = useState("");
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

	useEffect(() => {
		document.documentElement.dataset.theme = theme;
		window.localStorage.setItem(THEME_STORAGE_KEY, theme);
	}, [theme]);

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

	useEffect(() => {
		void loadAuthStatus();
		void loadAuthProviders();
	}, []);

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
					This frontend doubles as a quick auth console: it shows what the gateway and
					API believe about your identity before you hit the backend checks, while the
					Labiraus MCP server accepts either Google-backed login or certificate-authenticated
					agent access.
				</p>
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

			<footer className="legal-footer">
				<a href="/privacy-policy.html">Privacy Policy</a>
				<a href="/terms-of-service.html">Terms of Service</a>
			</footer>
		</main>
	);
}

export default App;
