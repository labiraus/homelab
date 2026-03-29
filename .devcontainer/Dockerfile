FROM mcr.microsoft.com/devcontainers/go:1-bookworm

USER root

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    xz-utils \
    curl \
    tar \
    socat \
    iputils-ping \
    netcat-openbsd \
    protobuf-compiler \
    ca-certificates \
    git \
    openssh-client \
    sudo \
    unzip \
    bash-completion \
    ripgrep \
    python3-pip \
    pipx \
    postgresql-client \
    && rm -rf /var/lib/apt/lists/*

COPY zscaler-ca.crt /usr/local/share/ca-certificates/zscaler-ca.crt
RUN update-ca-certificates
ENV AWS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/zscaler-ca.crt

RUN python3 -m pip install --upgrade --break-system-packages pipx
RUN pipx install --global 'ansible-core>=2.15,<2.16'

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

# Install MinIO client
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
    amd64) mc_arch='linux-amd64' ;; \
    arm64) mc_arch='linux-arm64' ;; \
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://dl.min.io/client/mc/release/${mc_arch}/mc" -o /usr/local/bin/mc; \
    chmod 0755 /usr/local/bin/mc

RUN set -eux; \
    plat="$(uname -m | grep -qiE 'aarch64\|arm64' && echo linux_arm64 || echo linux_amd64)"; \
    mkdir -p /home/vscode/.terraform.d/plugins/$plat /home/vscode/terraform-plugin-cache; \
    chown -R vscode:vscode /home/vscode/.terraform.d /home/vscode/terraform-plugin-cache; \
    # write a .terraformrc that points to the cache
    printf 'plugin_cache_dir = "/home/vscode/terraform-plugin-cache"\n' > /home/vscode/.terraformrc; \
    chown vscode:vscode /home/vscode/.terraformrc

# Ensure the mount target exists and source mounted bash customizations for interactive shells.
RUN printf '\n[ -f /home/vscode/.bashrc.devcontainer ] && . /home/vscode/.bashrc.devcontainer\n' >> /etc/bash.bashrc
