# reactapp

This is a small Vite and React frontend used to verify routing to the backend services from one place.

## What It Does

The UI exposes buttons that call:

- `/go/hello`
- `/python/hello`

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

Watch mode:

```bash
npm run test:watch
```

## Production Build

Create the static production build:

```bash
npm run build
```

Preview the production build locally:

```bash
npm run preview
```

## Container Build

The production image is built from `apps/reactapp/dockerfile`. It builds the Vite app with Node and serves the output with Nginx.
