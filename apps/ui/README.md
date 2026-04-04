# ui

This is a small Vite and React frontend used to verify routing to the public API from one place.

## What It Does

The UI exposes a button that calls:

- `/api/users/count`

The result is shown in the page so it is easy to confirm browser-to-gateway-to-service routing.

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
