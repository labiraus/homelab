#!/usr/bin/env bash
set -euo pipefail

if ! command -v ssh >/dev/null 2>&1; then
	echo "ssh is required for MinIO host checks" >&2
	exit 1
fi

target="${MINIO_HOST_SSH_TARGET:-svartalfheim}"

print_section() {
	printf '\n== %s ==\n' "$1"
}

ssh "$target" 'bash -s' <<'EOF'
set -euo pipefail

print_section() {
	printf '\n== %s ==\n' "$1"
}

print_section "Mount Status"
mount | grep " /srv/minio " || echo "not-mounted"
echo
sudo grep -n "srv/minio\|C2629529629522E9" /etc/fstab || true

print_section "Filesystem"
lsblk -f
echo
df -h /srv/minio /srv/minio/minio-data 2>/dev/null || true
echo
ls -ld /srv/minio /srv/minio/minio-data /srv/minio/minio-data/.minio.sys 2>/dev/null || true

print_section "MinIO Service"
systemctl status --no-pager minio | sed -n "1,24p"
echo
sudo grep -n "MINIO_" /etc/default/minio

print_section "Recent MinIO Errors"
sudo journalctl -u minio -n 120 --no-pager | grep -E "Storage resources are insufficient|no online disks found|Read failed|ERROR:" || true
EOF
