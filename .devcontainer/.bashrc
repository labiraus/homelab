# Devcontainer-specific shell customizations
# Load environment variables from ~/.env if present.
if [ -f "/home/vscode/.env" ]; then
  set -a
  . "/home/vscode/.env"
  set +a
fi

# Ensure an SSH agent is available for tools that require SSH auth
# (for example the Proxmox provider when proxmox_ve_ssh_agent=true).
if [ -n "${PS1:-}" ]; then
  _agent_env="/tmp/ssh-agent-vscode.env"

  _start_agent() {
    eval "$(ssh-agent -s)" >/dev/null
    {
      echo "export SSH_AUTH_SOCK=${SSH_AUTH_SOCK}"
      echo "export SSH_AGENT_PID=${SSH_AGENT_PID}"
    } > "${_agent_env}"
    chmod 600 "${_agent_env}"
  }

  if [ -f "${_agent_env}" ]; then
    # shellcheck disable=SC1090
    . "${_agent_env}" >/dev/null 2>&1 || true
  fi

  if ! ssh-add -l >/dev/null 2>&1; then
    _start_agent
  fi

  if ! ssh-add -l 2>/dev/null | grep -q "ED25519"; then
    ssh-add "/home/vscode/.ssh/id_ed25519" >/dev/null 2>&1 || true
  fi
fi

alias k='kubectl'
