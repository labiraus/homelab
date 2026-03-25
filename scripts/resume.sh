#!/usr/bin/env bash

set -euo pipefail

flux resume kustomization --all 
flux resume helmrelease --all 
flux resume source oci --all
flux resume source git --all
flux resume source helm --all
