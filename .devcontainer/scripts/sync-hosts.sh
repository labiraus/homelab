#!/usr/bin/env bash

set -euo pipefail

repo_hosts_file=/workspaces/homelab/.devcontainer/hosts
hosts_file=/etc/hosts
marker_begin="# codex-devcontainer-hosts-begin"
marker_end="# codex-devcontainer-hosts-end"
tmp_file="$(mktemp)"

hostname_value="$(hostname)"
fqdn_value="${hostname_value}.localdomain"

awk -v begin="$marker_begin" -v end="$marker_end" '
  $0 == begin { skip = 1; next }
  $0 == end { skip = 0; next }
  skip != 1 { print }
' "$hosts_file" > "$tmp_file"

cat >> "$tmp_file" <<EOF2
$marker_begin
127.0.1.1 $fqdn_value $hostname_value
EOF2

if [ -f "$repo_hosts_file" ]; then
  cat "$repo_hosts_file" >> "$tmp_file"
fi

cat >> "$tmp_file" <<EOF3
$marker_end
EOF3

cat "$tmp_file" | sudo tee "$hosts_file" > /dev/null
rm -f "$tmp_file"
