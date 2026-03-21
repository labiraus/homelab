#!/usr/bin/env bash

set -euo pipefail

flux resume kustomization --all -A
flux resume helmrelease --all -A
flux resume source oci --all -n flux-system
flux resume source git --all -n flux-system
flux resume source helm --all -n flux-system
