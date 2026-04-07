# ui

This is a small Vite and React frontend used to verify routing to the public API from one place.

## What It Does

The UI exposes:

- an authentication panel backed by `/api/auth/status`
- a federated login button driven by `/api/auth/providers`
- `/api/users/count`

The results are shown in the page so it is easy to confirm browser-to-gateway-to-service routing and inspect the current auth state seen by the API.

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

## Container Build

The production image is built from `apps/ui/dockerfile`. It builds the Vite app with Node and serves the output with Nginx.
