#!/usr/bin/env bash
set -euo pipefail

flux check --pre

flux install --namespace=flux-system

flux create secret oci ghcr-creds \
  --namespace=flux-system \
  --url=ghcr.io \
  --username=$GITHUB_USERNAME \
  --password=$GITHUB_TOKEN

  # If recreating source:
flux create source oci flux-bootstrap \
  --namespace=flux-system \
  --url=oci://ghcr.io/labiraus/homelab/helm/flux-bootstrap \
  --tag=uat \
  --secret-ref=ghcr-creds \
  --interval=10m \
  --export | kubectl apply -f -

flux create helmrelease flux-bootstrap \
  --chart-ref=OCIRepository/flux-bootstrap.flux-system \
  --interval=10m \
  --export | kubectl apply -f -
