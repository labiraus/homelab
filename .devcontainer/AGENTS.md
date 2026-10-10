# Devcontainer Maintenance

- Use versioned tool releases and verify download checksums where published. Download installer scripts successfully with `curl -fsSL` before executing them; do not pipe HTTP response bodies directly into a shell. Both the TFLint installer URL and MinIO binary endpoint have broken builds after upstream removal.
- The Dockerfile owns TFLint installation. Keep the Terraform feature's `tflint` option set to `none` so it does not replace that installation.
- Go intentionally follows the floating `mcr.microsoft.com/devcontainers/go:1-bookworm` image. Pull the base when refreshing Go and verify the actual toolchain in the finished image; do not add an exact patch pin unless requested.
- Preserve the restricted `.dockerignore` build context. Local credentials belong in runtime mounts, not image layers.
- Kubeconfig is optional at startup: use `KUBECONFIG` pointing to the workspace file, not a separate file bind mount. A missing kubeconfig must not block opening the container to run `make refresh-kubeconfig`.
- An image build and tool smoke checks do not validate startup mounts. When fixing startup or mount configuration, also validate with Dev Containers CLI `up --skip-post-create` and distinguish startup from lifecycle-hook checks.
- Validate changes with a full Dev Containers CLI build, including features, and smoke-check installed tools. On the corporate network, the host CLI may need `NODE_EXTRA_CA_CERTS` set to the local CA path.
