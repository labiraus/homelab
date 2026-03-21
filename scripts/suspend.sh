#!/usr/bin/env bash

set -euo pipefail

flux suspend kustomization --all
flux suspend helmrelease --all
flux suspend source oci --all
flux suspend source git --all
flux suspend source helm --all
