# ui

This is a small Vite and React frontend used to verify routing to the public API from one place.

Externally, the UI should present itself as Labiraus even though the repo keeps the app under `apps/ui/`.

## What It Does

The UI exposes:

- a GitHub-style top bar with an Overview tab and a Documents tab for authenticated users
- an auth status menu backed by `/api/auth/status` and `/api/auth/providers`
- sign-in and sign-out actions from the header auth menu
- `/api/users/count`
- an authenticated Search tab backed by `/api/documents/search` for semantic chunk retrieval
- a Search-tab context panel backed by `/api/documents/context` for cited context blocks
- a Search-tab lifecycle panel backed by `/api/documents/history` for retrieved documents
- an authenticated hash-routed documents page that browses `/api/documents/tree`, `/api/documents/object`, and `/api/documents/upload`
- a global toast stack backed by `/api/documents/events` for document lifecycle notifications
- static public legal pages at `/privacy-policy.html` and `/terms-of-service.html`

The results are shown in the page so it is easy to confirm browser-to-gateway-to-service routing and inspect the current auth state seen by the API.

In the current repo choice, the shared-host browser path is fronted by `oauth2-proxy`: `/` is proxied to `ui`, `/api/...` is proxied to `external`, and the login URL published by `/api/auth/providers` points at the local `oauth2-proxy` start endpoint rather than directly at Google.

When `authStatus.valid === true`, the app opens an `EventSource` to `/api/documents/events`, shows toast notifications for lifecycle updates, and tears the stream down again on sign-out or unmount. Toast copy is derived from the document lifecycle subject and document identifiers, with success, info, and error tones.

The authenticated Search tab submits natural-language queries to the public API, optionally narrows by object-key prefix or one exact metadata key/value pair, and renders the top matching processed chunks with similarity scores and download links. The same filters can assemble a compact cited context block through `/api/documents/context`. Each search result can also load its durable lifecycle history from `/api/documents/history`, showing processing versions, timestamps, lifecycle subjects, and recorded payload details.

## Local Development

Install dependencies:

```bash
npm install
```

Start the dev server:

```bash
npm run dev
```

The Vite dev server listens on `0.0.0.0:3000`.

If you need to target an API running on a different origin, set:

```bash
VITE_API_BASE_URL=http://host-or-gateway
```

## Tests

Run the test suite with:

```bash
npm test
```

## Production Build

Create the static production build:

```bash
npm run build
```

The generated `dist/` output is local build artifact only. It should not be committed; CI and the container build generate production assets from source.

## Container Build

The production image is built from `apps/ui/dockerfile`. It builds the Vite app with Node and serves the output with Nginx.
