import { useState } from "react";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";

const actions = [
	{
		id: "go",
		label: "Call Go API",
		method: "GET",
		path: "/go/hello",
		description: "Fetches the Go service greeting.",
	},
	{
		id: "python",
		label: "Call Python API",
		method: "POST",
		path: "/python/hello",
		body: { title: "Homelab UI request" },
		description: "Fetches the Python service greeting.",
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

function App() {
	const [activeRequest, setActiveRequest] = useState(null);
	const [message, setMessage] = useState("Choose a service call to verify routing.");
	const [error, setError] = useState("");

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

	return (
		<main className="app-shell">
			<section className="hero">
				<p className="eyebrow">Homelab UI</p>
				<h1>Verify backend routes from one place.</h1>
				<p className="intro">
					This frontend is intentionally small: it exposes the health of the integration
					points between the browser, the Go API, and the Python API.
				</p>
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
		</main>
	);
}

export default App;
