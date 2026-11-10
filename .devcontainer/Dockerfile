FROM golang:1.27.1-trixie AS go-toolchain
FROM mcr.microsoft.com/devcontainers/go:1-bookworm

# Replace the base image's Go tree so files from different releases cannot mix.
USER root
RUN rm -rf /usr/local/go
COPY --from=go-toolchain /usr/local/go /usr/local/go
ENV GOTOOLCHAIN=local

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

# Use the distro-managed pipx; pip cannot uninstall Debian packages without RECORD files.
# The base image is Python 3.11 (bookworm), so ansible-core 2.21.x is unavailable here;
# install the newest version that supports the image's interpreter instead.
RUN pipx install 'ansible-core==2.19.15'

# Install pinned official tools with architecture-specific release checksums.
# TFLint is installed by the Terraform feature in devcontainer.json.
RUN set -eux; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
    amd64) docs_sha='9005daf969de0b50134493a2c00078b49f5f5b39d021cda7c89bf4d4f3d776d3'; istio_sha='b32bba09bc6138d738fd19a09ef6d0fcdd5c5c013efeb72969663c24a2bddd47' ;; \
    arm64) docs_sha='d12bd7b73c1fc9c64efc79f8157dd713dabd559f1ecf3cfc0f42e32279a155fd'; istio_sha='e7efaf26b6ad60d878994d1ed6febccc11dc291962a92857e53946e663e9bb53' ;; \
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/terraform-docs/terraform-docs/releases/download/v0.24.0/terraform-docs-v0.24.0-linux-${arch}.tar.gz" -o /tmp/terraform-docs.tar.gz; \
    echo "${docs_sha}  /tmp/terraform-docs.tar.gz" | sha256sum -c -; \
    tar -xzf /tmp/terraform-docs.tar.gz -C /usr/local/bin terraform-docs; \
    curl -fsSL "https://github.com/istio/istio/releases/download/1.31.3/istio-1.31.3-linux-${arch}.tar.gz" -o /tmp/istio.tar.gz; \
    echo "${istio_sha}  /tmp/istio.tar.gz" | sha256sum -c -; \
    tar -xzf /tmp/istio.tar.gz -C /tmp istio-1.31.3/bin/istioctl; \
    install -m 0755 /tmp/istio-1.31.3/bin/istioctl /usr/local/bin/istioctl; \
    rm -rf /tmp/terraform-docs.tar.gz /tmp/istio.tar.gz /tmp/istio-1.31.3

# Install SeaweedFS tooling from a pinned, checksum-verified release.
RUN set -eux; \
    seaweed_version='4.51'; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
    amd64) seaweed_sha256='c03be37416a8ee672c77766e20b22a4e1430744c3af2668599a4b6eeaec14cee' ;; \
    arm64) seaweed_sha256='4772e7de6a3b8711eb53587a1c156aee67ebfc24536d15d85cd74a0d70ad53ec' ;; \
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
