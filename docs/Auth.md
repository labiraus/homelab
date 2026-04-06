# Authentication

This repo now supports three request auth outcomes in the public `external` API:

- `certificate`: the Istio gateway terminates mutual TLS, sanitizes and forwards `X-Forwarded-Client-Cert`, and the API extracts the email identity from the client certificate.
- `oidc`: an upstream gateway or auth proxy completes the OIDC login flow and forwards a trusted email header to the API.
- `none`: no trusted identity reached the API.

In all three cases, the Go middleware exposes the same auth status object to handlers and the UI:

- `mode`: `certificate`, `oidc`, or `none`
- `email`: normalized email address when present
- `valid`: whether that email exists in `auth.users`
- `invalidReason`: why the identity is rejected when it is not valid

## Identity Model

The database-backed identity model lives in `sql/auth.pgsql`.

- Email address is the primary identity key.
- The only seeded valid user is `oliver@labiraus.com`.
- Any other email is reported as invalid and unrecognized by the middleware and `/api/auth/status`.

## Runtime Flow

1. The browser or API client reaches the Istio gateway.
2. The gateway either:
   - forwards a sanitized `X-Forwarded-Client-Cert` header after successful client-certificate validation, or
   - forwards a trusted OIDC email header after an upstream OIDC flow, or
   - forwards neither, which becomes `mode: none`.
3. `apps/pkg/api` resolves the auth mode and normalized email.
4. `apps/external` validates the email against Postgres `auth.users`.
5. The UI reads `/api/auth/status` and shows the current status.

## Google Login Button

The UI button targets `/api/auth/login/google`.

- `external` redirects that route to `OIDC_LOGIN_URL`.
- Set `OIDC_LOGIN_URL` to the Google or oauth2-proxy start URL you want the browser to begin with.
- After the upstream OIDC flow completes, the trusted gateway or auth proxy must inject the email header the API reads, currently `X-Auth-Request-Email`.

## Client Certificate Generation

Use the repo script:

```bash
chmod +x scripts/generate-client-cert.sh
scripts/generate-client-cert.sh init-ca ./.local/auth-certs "Homelab Client CA"
scripts/generate-client-cert.sh issue-client ./.local/auth-certs oliver@labiraus.com
scripts/generate-client-cert.sh export-p12 ./.local/auth-certs oliver@labiraus.com changeit
```

What the script creates:

- `ca.crt` / `ca.key`: self-signed client-auth CA
- `<email>.key`: client private key
- `<email>.csr`: CSR
- `<email>.crt`: signed client certificate
- `<email>.p12`: browser-friendly PKCS#12 bundle

The client certificate embeds the email in two places:

- subject `emailAddress=<email>`
- URI SAN `spiffe://homelab/users/<email>`

The Go middleware reads the email from the forwarded certificate identity, preferring the URI SAN shape because it is stable in the `X-Forwarded-Client-Cert` header.

## Install The CA And Client Cert Locally

Common local steps:

```bash
# Trust the client-auth CA locally when testing with curl or browsers
sudo cp ./.local/auth-certs/ca.crt /usr/local/share/ca-certificates/homelab-client-ca.crt
sudo update-ca-certificates
```

For browser import, load the generated `.p12` file into the browser certificate store. The example `export-p12` command above uses password `changeit`; choose a different password for real use.

Example curl call against a strict mTLS gateway:

```bash
curl --cert ./.local/auth-certs/oliver_labiraus.com.crt \
  --key ./.local/auth-certs/oliver_labiraus.com.key \
  --cacert ./.local/auth-certs/ca.crt \
  https://mcp.labiraus.com/api/auth/status
```

## Istio Trust Configuration

Istio can validate client certificates from the self-signed CA by referencing a secret with `ca.crt`.

Create the CA secret in the gateway namespace:

```bash
kubectl create secret generic homelab-client-ca \
  -n ingress \
  --from-file=ca.crt=./.local/auth-certs/ca.crt
```

The gateway chart now supports an optional validation block:

- `helm/bootstrap/gateway/values.yaml`
- `clientCertificateValidation.enabled`
- `clientCertificateValidation.secretName`

When enabled, the rendered Gateway adds `spec.tls.frontend.default.validation.caCertificateRefs`, which makes Istio verify client certificates against that CA secret.

The Istio bootstrap values also set:

```yaml
meshConfig:
  defaultConfig:
    gatewayTopology:
      forwardClientCertDetails: SANITIZE_SET
```

That causes Istio to sanitize any incoming client-supplied `X-Forwarded-Client-Cert` value and replace it with gateway-generated certificate details.

## Strict mTLS Versus Browser OIDC Fallback

Enabling client-certificate validation on the shared browser gateway makes a client certificate mandatory for that HTTPS listener. That is appropriate for a dedicated cert-only endpoint, but it will block the normal browser OIDC fallback flow.

Recommended pattern:

- Keep the shared `mcp.labiraus.com` listener on normal HTTPS for UI plus OIDC fallback.
- Use the optional CA-validation block only on a dedicated strict-mTLS gateway or dedicated hostname when you want certificate-only access.

If you enable strict validation on the shared gateway anyway, document that operational choice clearly because browser users without a client certificate will fail the TLS handshake before the OIDC flow can begin.
