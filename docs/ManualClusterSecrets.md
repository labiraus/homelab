# Manual Cluster Secrets

This document tracks the Kubernetes secrets that are still intentionally installed out-of-band rather than committed into Helm values or generated entirely inside the cluster.

Use it as the operator checklist when bootstrapping a new cluster or recovering one that is missing credential material.

## Why These Stay Manual

These secrets fall into one of three buckets:

- external credentials that should not live in Git, such as GHCR auth or Google OAuth client credentials
- trust material that is generated locally, such as the client CA used for certificate validation
- MinIO access credentials that are managed outside Kubernetes and then mirrored into the cluster

The current boundary for MinIO is:

- MinIO on `svartalfheim` should not be directly exposed to the public internet
- Kubernetes workloads should reach MinIO on the cluster-visible private path
- if a browser-facing download or future pre-signed-style route is needed, it should be served through an authenticated `mcp.labiraus.com` endpoint rather than pointing the client at MinIO directly

## Required Secrets

### `flux-system/ghcr-creds`

- Type: `kubernetes.io/dockerconfigjson`
- Purpose: lets Flux pull private OCI charts from `ghcr.io/labiraus/homelab/charts/*`
- Used by:
  - `helm/bootstrap/flux-bootstrap`
  - `helm/bootstrap/flux-infra`
  - `helm/bootstrap/flux-apps`
  - `helm/bootstrap/flux-data`
  - `helm/bootstrap/flux-observability`
- Source of truth: operator GitHub package credentials or PAT with GHCR access

Create it with Flux during bootstrap:

```bash
flux create secret oci ghcr-creds \
  --namespace=flux-system \
  --url=ghcr.io \
  --username="$GITHUB_USERNAME" \
  --password="$GITHUB_TOKEN"
```

Repo references:

- [scripts/kube-init.sh](/workspaces/homelab/scripts/kube-init.sh:8)
- [helm/bootstrap/flux-apps/values.yaml](/workspaces/homelab/helm/bootstrap/flux-apps/values.yaml:14)
- [helm/bootstrap/flux-infra/values.yaml](/workspaces/homelab/helm/bootstrap/flux-infra/values.yaml:14)

Bootstrap note:

- the `flux-*` bootstrap charts now declare one top-level values block and one template per component rather than a shared `releases:` list

### `homelab/oauth2-proxy-google`

- Type: `Opaque`
- Purpose: supplies the browser-login credentials for `oauth2-proxy`
- Required keys:
  - `OAUTH2_PROXY_CLIENT_ID`
  - `OAUTH2_PROXY_CLIENT_SECRET`
  - `OAUTH2_PROXY_COOKIE_SECRET`
- Used by:
  - `helm/infra/oauth2-proxy`
- Source of truth: Google Cloud OAuth client and a locally generated cookie secret

Create it manually before browser login is expected to work:

```bash
kubectl -n homelab create secret generic oauth2-proxy-google \
  --from-literal=OAUTH2_PROXY_CLIENT_ID='...' \
  --from-literal=OAUTH2_PROXY_CLIENT_SECRET='...' \
  --from-literal=OAUTH2_PROXY_COOKIE_SECRET='...'
```

Repo references:

- [docs/OAuth2ProxyGoogle.md](/workspaces/homelab/docs/OAuth2ProxyGoogle.md:109)
- [helm/infra/oauth2-proxy/values.yaml](/workspaces/homelab/helm/infra/oauth2-proxy/values.yaml:57)

### `ingress/homelab-client-ca`

- Type: `Opaque`
- Purpose: gives the Istio gateway the CA bundle used to validate client certificates
- Required key:
  - `ca.crt`
- Used by:
  - `helm/bootstrap/gateway`
- Source of truth: the local client-auth CA under `.local/auth-certs/ca.crt`

Create it when certificate validation at the gateway is enabled:

```bash
kubectl create secret generic homelab-client-ca \
  -n ingress \
  --from-file=ca.crt=./.local/auth-certs/ca.crt
```

Repo references:

- [docs/Auth.md](/workspaces/homelab/docs/Auth.md:101)
- [helm/bootstrap/gateway/values.yaml](/workspaces/homelab/helm/bootstrap/gateway/values.yaml:11)

### `velero/velero-minio-credentials`

- Type: `Opaque`
- Purpose: lets Velero store backups in the external MinIO bucket
- Source of truth: external MinIO managed-user credentials
- Preferred generation path: Ansible MinIO state run, then manual `kubectl apply`

Repo references:

- [values/velero-values.yaml](/workspaces/homelab/values/velero-values.yaml:14)
- [ansible/inventory/group_vars/all.yml](/workspaces/homelab/ansible/inventory/group_vars/all.yml:110)
- [ansible/README.md](/workspaces/homelab/ansible/README.md:277)

### `data/cnpg-backup-s3`

- Type: `Opaque`
- Purpose: lets CloudNativePG write backups to the external MinIO bucket
- Source of truth: external MinIO managed-user credentials
- Preferred generation path: Ansible MinIO state run, then manual `kubectl apply`

Repo references:

- [helm/data/postgres/values.yaml](/workspaces/homelab/helm/data/postgres/values.yaml:14)
- [ansible/inventory/group_vars/all.yml](/workspaces/homelab/ansible/inventory/group_vars/all.yml:118)
- [ansible/README.md](/workspaces/homelab/ansible/README.md:277)

### `homelab/documents-minio-credentials`

- Type: `Opaque`
- Required keys:
  - `accessKey`
  - `secretKey`
- Purpose: lets `external` and `mcp` browse, upload, and download objects from the `documents` bucket
- Source of truth: external MinIO managed-user credentials
- Preferred generation path:
  - provision or rotate the `documents` MinIO user outside the cluster
  - mirror the resulting access key and secret key into this Kubernetes secret

Repo references:

- [helm/apps/external/values.yaml](/workspaces/homelab/helm/apps/external/values.yaml:88)
- [helm/apps/mcp/values.yaml](/workspaces/homelab/helm/apps/mcp/values.yaml:62)
- [ansible/inventory/group_vars/all.yml](/workspaces/homelab/ansible/inventory/group_vars/all.yml:126)

## MinIO Secret Workflow

For the MinIO-backed secrets above, the current repo model is:

1. keep the real MinIO user credentials local-only in `ansible/.env`
2. use Ansible or an equivalent operator flow to ensure the MinIO user and policy exist on `svartalfheim`
3. mirror the resulting secret into Kubernetes

When Ansible generates the Kubernetes manifests, they land under `ansible/out/k8s-secrets/` and can be applied with:

```bash
kubectl apply -f ansible/out/k8s-secrets/
```

## Rotation Notes

- Rotating `ghcr-creds` affects Flux chart pulls and bootstrap chart fanout.
- Rotating `oauth2-proxy-google` may invalidate browser-login behavior until the deployment reloads.
- Rotating `homelab-client-ca` changes which client certificates the gateway trusts.
- Rotating any MinIO access secret requires rotating both the external MinIO user credentials and the matching Kubernetes secret.

## Related Docs

- [Secrets](/workspaces/homelab/docs/Secrets.md)
- [oauth2-proxy + Google](/workspaces/homelab/docs/OAuth2ProxyGoogle.md)
- [Authentication](/workspaces/homelab/docs/Auth.md)
- [ansible README](/workspaces/homelab/ansible/README.md)
