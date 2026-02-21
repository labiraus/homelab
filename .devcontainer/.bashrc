# Devcontainer-specific shell customizations
# Load environment variables from ~/.env if present.
if [ -f "/home/vscode/.env" ]; then
  set -a
  . "/home/vscode/.env"
  set +a
fi
alias k='kubectl'