#!/usr/bin/env bash
set -euo pipefail

flux install --namespace=flux-system

flux create source oci flux-bootstrap \
  --url=oci://ghcr.io/labiraus/homelab/helm/flux-bootstrap \
  --tag=uat \
  --interval=10m \
  --export | kubectl apply -f -

flux create helmrelease flux-bootstrap \
  --source=OCIRepository/flux-bootstrap \
  --path="./" \
  --prune=true \
  --wait=true \
  --interval=10m \
  --export | kubectl apply -f -
