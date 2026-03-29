#!/usr/bin/env bash

set -euo pipefail

APP_NAME="${APP_NAME:-mc-debug-ftp}"
NAMESPACE="${NAMESPACE:-minecraft}"
MINECRAFT_DEPLOYMENT="${MINECRAFT_DEPLOYMENT:-minecraft}"
AUTH_SECRET_NAME="${AUTH_SECRET_NAME:-mc-debug-ftp-auth}"
BACKUP_CRONJOB="${BACKUP_CRONJOB:-minecraft-world-backup}"
FTP_PORT="${FTP_PORT:-2121}"
PASV_MIN_PORT="${PASV_MIN_PORT:-21100}"
PASV_MAX_PORT="${PASV_MAX_PORT:-21105}"

usage() {
  cat <<EOF
Usage: $(basename "$0") <command>

Commands:
  up            Scale down Minecraft and scale up the FTP debug deployment
  down          Scale down the FTP debug deployment and restore the Minecraft deployment
  status        Show debug and Minecraft deployment status
  port-forward  Forward localhost ports for FileZilla access
  logs          Tail the FTP pod logs

Environment overrides:
  APP_NAME      Resource name prefix (default: ${APP_NAME})
  NAMESPACE     Namespace containing the Minecraft PVC (default: ${NAMESPACE})
  MINECRAFT_DEPLOYMENT
                Minecraft deployment to scale down/up (default: ${MINECRAFT_DEPLOYMENT})
  AUTH_SECRET_NAME
                Secret that stores FileZilla credentials (default: ${AUTH_SECRET_NAME})
  BACKUP_CRONJOB
                CronJob that should be suspended during debug access (default: ${BACKUP_CRONJOB})
  FTP_PORT      Local FTP control port for port-forward (default: ${FTP_PORT})
  PASV_MIN_PORT First passive mode port (default: ${PASV_MIN_PORT})
  PASV_MAX_PORT Last passive mode port (default: ${PASV_MAX_PORT})

Example:
  $(basename "$0") up
  $(basename "$0") port-forward
EOF
}

service_name() {
  printf "%s" "${APP_NAME}"
}

passive_ports_csv() {
  local ports=()
  local port
  for ((port = PASV_MIN_PORT; port <= PASV_MAX_PORT; port++)); do
    ports+=("${port}")
  done
  local joined
  joined=$(IFS=,; echo "${ports[*]}")
  printf "%s" "${joined}"
}

port_forward_args() {
  local args=("${FTP_PORT}:21")
  local port
  for ((port = PASV_MIN_PORT; port <= PASV_MAX_PORT; port++)); do
    args+=("${port}:${port}")
  done
  printf "%s " "${args[@]}"
}

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Error: missing required command: $1" >&2
    exit 1
  }
}

ensure_namespace() {
  kubectl get namespace "${NAMESPACE}" >/dev/null
}

ensure_deployment() {
  kubectl -n "${NAMESPACE}" get deployment "$1" >/dev/null
}

cronjob_exists() {
  kubectl -n "${NAMESPACE}" get cronjob "$1" >/dev/null 2>&1
}

suspend_cronjob() {
  local cronjob="$1"
  if cronjob_exists "${cronjob}"; then
    kubectl -n "${NAMESPACE}" patch cronjob "${cronjob}" --type=merge -p '{"spec":{"suspend":true}}' >/dev/null
  fi
}

resume_cronjob() {
  local cronjob="$1"
  if cronjob_exists "${cronjob}"; then
    kubectl -n "${NAMESPACE}" patch cronjob "${cronjob}" --type=merge -p '{"spec":{"suspend":false}}' >/dev/null
  fi
}

warn_if_backup_job_running() {
  local cronjob="$1"
  local active_jobs

  if ! cronjob_exists "${cronjob}"; then
    return 0
  fi

  active_jobs="$(kubectl -n "${NAMESPACE}" get jobs -l "batch.kubernetes.io/cronjob-name=${cronjob}" -o jsonpath='{range .items[*]}{.metadata.name} {.status.active}{"\n"}{end}')"
  if printf "%s" "${active_jobs}" | rg -q ' [1-9][0-9]*$'; then
    echo "Warning: ${cronjob} still has an active backup job using the PVC." >&2
    echo "Wait for the job to finish or delete the active backup pod/job before retrying debug access." >&2
    kubectl -n "${NAMESPACE}" get jobs -l "batch.kubernetes.io/cronjob-name=${cronjob}"
    exit 1
  fi
}

get_secret_value() {
  local key="$1"
  kubectl -n "${NAMESPACE}" get secret "${AUTH_SECRET_NAME}" -o "jsonpath={.data.${key}}" | base64 -d
}

wait_for_replicas() {
  local deployment="$1"
  local expected="$2"
  local replicas=""

  for _ in $(seq 1 60); do
    replicas="$(kubectl -n "${NAMESPACE}" get deployment "${deployment}" -o jsonpath='{.status.replicas}')"
    replicas="${replicas:-0}"
    if [[ "${replicas}" == "${expected}" ]]; then
      return 0
    fi
    sleep 2
  done

  echo "Error: ${deployment} did not reach ${expected} replicas in time." >&2
  exit 1
}

show_status() {
  kubectl -n "${NAMESPACE}" get deployment "${MINECRAFT_DEPLOYMENT}" "${APP_NAME}"
  kubectl -n "${NAMESPACE}" get service "$(service_name)"
  kubectl -n "${NAMESPACE}" get pod -l "app.kubernetes.io/name=${APP_NAME}"
  if cronjob_exists "${BACKUP_CRONJOB}"; then
    kubectl -n "${NAMESPACE}" get cronjob "${BACKUP_CRONJOB}"
  fi
  if kubectl -n "${NAMESPACE}" get secret "${AUTH_SECRET_NAME}" >/dev/null 2>&1; then
    echo
    echo "FileZilla settings:"
    echo "  Protocol: FTP"
    echo "  Host: 127.0.0.1"
    echo "  Port: ${FTP_PORT}"
    echo "  Username: $(get_secret_value ftp-user)"
    echo "  Password: $(get_secret_value ftp-pass)"
    echo "  Encryption: Only use plain FTP (insecure)"
    echo "  Transfer Mode: Passive"
    echo "  Passive ports: $(passive_ports_csv)"
  fi
}

cmd_up() {
  ensure_namespace
  ensure_deployment "${MINECRAFT_DEPLOYMENT}"
  ensure_deployment "${APP_NAME}"
  echo "Suspending ${BACKUP_CRONJOB}..."
  suspend_cronjob "${BACKUP_CRONJOB}"
  warn_if_backup_job_running "${BACKUP_CRONJOB}"
  echo "Scaling ${MINECRAFT_DEPLOYMENT} down..."
  kubectl -n "${NAMESPACE}" scale deployment "${MINECRAFT_DEPLOYMENT}" --replicas=0 >/dev/null
  wait_for_replicas "${MINECRAFT_DEPLOYMENT}" 0
  echo "Scaling ${APP_NAME} up..."
  kubectl -n "${NAMESPACE}" scale deployment "${APP_NAME}" --replicas=1 >/dev/null
  echo "Waiting for ${APP_NAME} deployment to become ready..."
  kubectl -n "${NAMESPACE}" rollout status deploy/"${APP_NAME}" --timeout=180s
  show_status
  echo
  echo "Run '$(basename "$0") port-forward' and then connect with FileZilla."
}

cmd_down() {
  ensure_namespace
  ensure_deployment "${MINECRAFT_DEPLOYMENT}"
  ensure_deployment "${APP_NAME}"
  echo "Scaling ${APP_NAME} down..."
  kubectl -n "${NAMESPACE}" scale deployment "${APP_NAME}" --replicas=0 >/dev/null
  wait_for_replicas "${APP_NAME}" 0
  echo "Scaling ${MINECRAFT_DEPLOYMENT} up..."
  kubectl -n "${NAMESPACE}" scale deployment "${MINECRAFT_DEPLOYMENT}" --replicas=1 >/dev/null
  kubectl -n "${NAMESPACE}" rollout status deployment/"${MINECRAFT_DEPLOYMENT}" --timeout=300s
  echo "Resuming ${BACKUP_CRONJOB}..."
  resume_cronjob "${BACKUP_CRONJOB}"
  echo "Scaled ${APP_NAME} down and restored ${MINECRAFT_DEPLOYMENT}."
}

cmd_status() {
  show_status
}

cmd_port_forward() {
  ensure_namespace
  echo "Starting port-forward for FTP control and passive ports..."
  echo "FileZilla host: 127.0.0.1"
  echo "FileZilla port: ${FTP_PORT}"
  kubectl -n "${NAMESPACE}" port-forward svc/"$(service_name)" $(port_forward_args)
}

cmd_logs() {
  kubectl -n "${NAMESPACE}" logs -f deploy/"${APP_NAME}"
}

main() {
  require kubectl

  case "${1:-}" in
    up)
      cmd_up
      ;;
    down)
      cmd_down
      ;;
    status)
      cmd_status
      ;;
    port-forward)
      cmd_port_forward
      ;;
    logs)
      cmd_logs
      ;;
    ""|-h|--help|help)
      usage
      ;;
    *)
      echo "Error: unknown command '${1}'" >&2
      echo >&2
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
