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
RUN curl -fsSLo /tmp/terraform-docs.tar.gz https://terraform-docs.io/dl/v0.16.0/terraform-docs-v0.16.0-$(uname)-$(dpkg --print-architecture).tar.gz \
    && tar -xzf /tmp/terraform-docs.tar.gz -C /tmp \
    && mv /tmp/terraform-docs /usr/local/bin/terraform-docs \
    && chmod +x /usr/local/bin/terraform-docs \
    && rm /tmp/terraform-docs.tar.gz

# Install TFLint from a versioned release; the old install_linux.sh URL is gone.
ARG TFLINT_VERSION=0.64.0
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in amd64|arm64) ;; *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; esac; \
    mkdir /tmp/tflint-install; \
    cd /tmp/tflint-install; \
    release="https://github.com/terraform-linters/tflint/releases/download/v${TFLINT_VERSION}"; \
    curl -fsSLO "${release}/tflint_linux_${arch}.zip"; \
    curl -fsSLO "${release}/checksums.txt"; \
    sha256sum --ignore-missing -c checksums.txt; \
    unzip "tflint_linux_${arch}.zip"; \
    install -m 0755 tflint /usr/local/bin/tflint; \
    cd /; \
    rm -rf /tmp/tflint-install; \
    tflint --version

# Install istioctl
RUN set -eux; \
    cd /tmp && \
    curl -fsSL https://istio.io/downloadIstio -o /tmp/install-istio.sh && \
    ISTIO_VERSION=1.29.0 sh /tmp/install-istio.sh && \
    cd istio-* && \
    install -m 0755 bin/istioctl /usr/local/bin/istioctl && \
    cd / && \
    rm -rf /tmp/istio-* /tmp/install-istio.sh

RUN set -eux; \
    plat="linux_$(dpkg --print-architecture)"; \
    mkdir -p /home/vscode/.terraform.d/plugins/$plat /home/vscode/terraform-plugin-cache; \
    chown -R vscode:vscode /home/vscode/.terraform.d /home/vscode/terraform-plugin-cache; \
    # write a .terraformrc that points to the cache
    printf 'plugin_cache_dir = "/home/vscode/terraform-plugin-cache"\n' > /home/vscode/.terraformrc; \
    chown vscode:vscode /home/vscode/.terraformrc

# Ensure the mount target exists and source mounted bash customizations for interactive shells.
RUN printf '\n[ -f /home/vscode/.bashrc.devcontainer ] && . /home/vscode/.bashrc.devcontainer\n' >> /etc/bash.bashrc

# The public MinIO binary endpoint returns HTTP 410; build the pinned client source.
ARG MC_VERSION=RELEASE.2025-08-13T08-35-41Z
RUN set -eux; \
    go version; \
    GOBIN=/usr/local/bin GOMODCACHE=/tmp/mc-mod GOCACHE=/tmp/mc-cache \
        go install "github.com/minio/mc@${MC_VERSION}"; \
    rm -rf /tmp/mc-mod /tmp/mc-cache; \
    mc --version
