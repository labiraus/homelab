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
    bubblewrap \
    ripgrep \
    python3-pip \
    pipx \
    postgresql-client \
    make \
    && rm -rf /var/lib/apt/lists/*

COPY zscaler-ca.crt /usr/local/share/ca-certificates/zscaler-ca.crt
RUN update-ca-certificates
ENV AWS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/zscaler-ca.crt

RUN set -eux; \
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg -o /usr/share/keyrings/githubcli-archive-keyring.gpg; \
    chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg; \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list; \
    apt-get update; \
    apt-get install -y --no-install-recommends gh; \
    rm -rf /var/lib/apt/lists/*

RUN python3 -m pip install --upgrade --break-system-packages pipx
RUN pipx install --global 'ansible-core>=2.15,<2.16'

# Install terraform-docs
RUN curl -sSLo /tmp/terraform-docs.tar.gz https://terraform-docs.io/dl/v0.16.0/terraform-docs-v0.16.0-$(uname)-amd64.tar.gz \
    && tar -xzf /tmp/terraform-docs.tar.gz -C /tmp \
    && mv /tmp/terraform-docs /usr/local/bin/terraform-docs \
    && chmod +x /usr/local/bin/terraform-docs

# TFLint is installed by the Terraform feature in devcontainer.json.

# Install istioctl
RUN set -eux; \
    cd /tmp && \
    curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.29.0 sh - && \
    cd istio-* && \
    install -m 0755 bin/istioctl /usr/local/bin/istioctl && \
    cd / && \
    rm -rf /tmp/istio-*

# Install SeaweedFS tooling from a pinned, checksum-verified release.
RUN set -eux; \
    seaweed_version='4.48'; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
    amd64) seaweed_sha256='4a7d108384d044d95212d1342cdda9533fa55842c1c9b41f606ca3c8a9561124' ;; \
    arm64) seaweed_sha256='557e92c38c5c7d180748eb8e2b2b45e426f8e5282a4caca8e098a90d09f03a07' ;; \
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/seaweedfs/seaweedfs/releases/download/${seaweed_version}/linux_${arch}.tar.gz" -o /tmp/seaweedfs.tar.gz; \
    echo "${seaweed_sha256}  /tmp/seaweedfs.tar.gz" | sha256sum -c -; \
    tar -xzf /tmp/seaweedfs.tar.gz -C /usr/local/bin weed; \
    chmod 0755 /usr/local/bin/weed; \
    rm /tmp/seaweedfs.tar.gz

RUN set -eux; \
    plat="$(uname -m | grep -qiE 'aarch64\|arm64' && echo linux_arm64 || echo linux_amd64)"; \
    mkdir -p /home/vscode/.terraform.d/plugins/$plat /home/vscode/terraform-plugin-cache; \
    chown -R vscode:vscode /home/vscode/.terraform.d /home/vscode/terraform-plugin-cache; \
    # write a .terraformrc that points to the cache
    printf 'plugin_cache_dir = "/home/vscode/terraform-plugin-cache"\n' > /home/vscode/.terraformrc; \
    chown vscode:vscode /home/vscode/.terraformrc

# Ensure the mount target exists and source mounted bash customizations for interactive shells.
RUN printf '\n[ -f /home/vscode/.bashrc.devcontainer ] && . /home/vscode/.bashrc.devcontainer\n' >> /etc/bash.bashrc
