#!/usr/bin/env bash

set -euo pipefail

usage() {
	cat <<'EOF'
Usage:
  scripts/generate-client-cert.sh init-ca <output-dir> [ca-common-name]
  scripts/generate-client-cert.sh issue-client <output-dir> <email>
  scripts/generate-client-cert.sh export-p12 <output-dir> <email> [password]

Examples:
  scripts/generate-client-cert.sh init-ca ./.local/auth-certs "Homelab Client CA"
  scripts/generate-client-cert.sh issue-client ./.local/auth-certs oliver@labiraus.com
  scripts/generate-client-cert.sh export-p12 ./.local/auth-certs oliver@labiraus.com changeit
EOF
}

require_openssl() {
	if ! command -v openssl >/dev/null 2>&1; then
		echo "openssl is required" >&2
		exit 1
	fi
}

email_slug() {
	printf '%s' "$1" | tr '@' '_' | tr -c 'a-zA-Z0-9._-' '_'
}

write_client_extensions() {
	local output_file=$1
	local email=$2
	cat >"$output_file" <<EOF
[ v3_client ]
basicConstraints = CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectAltName = @alt_names

[ alt_names ]
email.1 = ${email}
URI.1 = spiffe://homelab/users/${email}
EOF
}

init_ca() {
	local output_dir=$1
	local ca_common_name=${2:-Homelab Client CA}

	mkdir -p "$output_dir"

	openssl req -x509 -sha256 -nodes -days 3650 \
		-newkey rsa:4096 \
		-subj "/CN=${ca_common_name}" \
		-keyout "${output_dir}/ca.key" \
		-out "${output_dir}/ca.crt"

	echo "Created CA key and certificate in ${output_dir}"
}

issue_client() {
	local output_dir=$1
	local email=$2
	local slug
	local ext_file

	slug=$(email_slug "$email")
	ext_file="${output_dir}/${slug}.ext"

	test -f "${output_dir}/ca.key"
	test -f "${output_dir}/ca.crt"

	openssl req -new -nodes -newkey rsa:2048 \
		-subj "/CN=${email}/emailAddress=${email}" \
		-keyout "${output_dir}/${slug}.key" \
		-out "${output_dir}/${slug}.csr"

	write_client_extensions "$ext_file" "$email"

	openssl x509 -req -sha256 -days 825 \
		-in "${output_dir}/${slug}.csr" \
		-CA "${output_dir}/ca.crt" \
		-CAkey "${output_dir}/ca.key" \
		-CAcreateserial \
		-out "${output_dir}/${slug}.crt" \
		-extfile "$ext_file" \
		-extensions v3_client

	rm -f "$ext_file"

	echo "Created client key, CSR, and certificate for ${email} in ${output_dir}"
}

export_p12() {
	local output_dir=$1
	local email=$2
	local password=${3:-changeit}
	local slug

	slug=$(email_slug "$email")

	test -f "${output_dir}/${slug}.key"
	test -f "${output_dir}/${slug}.crt"
	test -f "${output_dir}/ca.crt"

	openssl pkcs12 -export \
		-inkey "${output_dir}/${slug}.key" \
		-in "${output_dir}/${slug}.crt" \
		-certfile "${output_dir}/ca.crt" \
		-out "${output_dir}/${slug}.p12" \
		-passout "pass:${password}"

	echo "Created ${output_dir}/${slug}.p12"
}

main() {
	require_openssl

	if [[ $# -lt 2 ]]; then
		usage
		exit 1
	fi

	local command=$1
	shift

	case "$command" in
		init-ca)
			init_ca "$@"
			;;
		issue-client)
			if [[ $# -ne 2 ]]; then
				usage
				exit 1
			fi
			issue_client "$@"
			;;
		export-p12)
			if [[ $# -lt 2 || $# -gt 3 ]]; then
				usage
				exit 1
			fi
			export_p12 "$@"
			;;
		*)
			usage
			exit 1
			;;
	esac
}

main "$@"
