FROM mcr.microsoft.com/devcontainers/go:1-bookworm

USER root

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    openssh-client \
    sudo \
    unzip \
    && rm -rf /var/lib/apt/lists/*

COPY zscaler-ca.crt /usr/local/share/ca-certificates/zscaler-ca.crt
RUN update-ca-certificates

# Install terraform-docs
RUN curl -sSLo /tmp/terraform-docs.tar.gz https://terraform-docs.io/dl/v0.16.0/terraform-docs-v0.16.0-$(uname)-amd64.tar.gz \
    && tar -xzf /tmp/terraform-docs.tar.gz -C /tmp \
    && mv /tmp/terraform-docs /usr/local/bin/terraform-docs \
    && chmod +x /usr/local/bin/terraform-docs

# Install tflint
RUN curl -s https://raw.githubusercontent.com/terraform-linters/tflint/master/install_linux.sh | bash

# Install istioctl
RUN set -eux; \
    cd /tmp && \
    curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.29.0 sh - && \
    cd istio-* && \
    install -m 0755 bin/istioctl /usr/local/bin/istioctl && \
    cd / && \
    rm -rf /tmp/istio-*

RUN set -eux; \
    plat="$(uname -m | grep -qiE 'aarch64\|arm64' && echo linux_arm64 || echo linux_amd64)"; \
    mkdir -p /home/vscode/.terraform.d/plugins/$plat /home/vscode/terraform-plugin-cache; \
    chown -R vscode:vscode /home/vscode/.terraform.d /home/vscode/terraform-plugin-cache; \
    # write a .terraformrc that points to the cache
    printf 'plugin_cache_dir = "/home/vscode/terraform-plugin-cache"\n' > /home/vscode/.terraformrc; \
    chown vscode:vscode /home/vscode/.terraformrc

# Ensure the mount target exists and source mounted bash customizations for interactive shells.
RUN printf '\n[ -f /home/vscode/.bashrc.devcontainer ] && . /home/vscode/.bashrc.devcontainer\n' >> /etc/bash.bashrc
