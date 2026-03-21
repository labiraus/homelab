#!/usr/bin/env bash

set -euo pipefail

flux suspend kustomization --all -A
flux suspend helmrelease --all -A
flux suspend source oci --all -n flux-system
flux suspend source git --all -n flux-system
flux suspend source helm --all -n flux-system
