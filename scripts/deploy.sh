#!/usr/bin/env bash
set -euo pipefail

COMPOSE_DIR="$HOME/services/nouveauricheglobalgroup"
KRAKEND_CONFIG_DIR="/etc/krakend/config"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options] [service...]

Stops, pulls, and restarts specified Docker Compose services.
If no services are specified, operates on ALL services.

Options:
  -k, --krakend    Deploy KraKend config and restart the KraKend service
  -h, --help       Show this help

Examples:
  $(basename "$0")                              # all docker services
  $(basename "$0") subscription-external        # single service
  $(basename "$0") subscription-external landing-web  # multiple services
  $(basename "$0") --krakend                    # krakend config only
  $(basename "$0") --krakend subscription-external  # krakend + docker service
EOF
  exit 0
}

deploy_krakend=false
services=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage ;;
    -k|--krakend) deploy_krakend=true; shift ;;
    *) services+=("$1"); shift ;;
  esac
done

# --- KraKend deployment ---
if [[ "$deploy_krakend" == true ]]; then
  echo "==> Deploying KraKend config"

  # Validate config before applying (requires flexible-config env vars)
  if command -v krakend &>/dev/null; then
    echo "  -> Validating config..."
    FC_ENABLE=1 \
    FC_SETTINGS="$KRAKEND_CONFIG_DIR/settings/do" \
    FC_PARTIALS="$KRAKEND_CONFIG_DIR/partials" \
    FC_TEMPLATES="$KRAKEND_CONFIG_DIR/templates" \
      krakend check -t -c "$KRAKEND_CONFIG_DIR/krakend.tmpl" || {
        echo "ERROR: KraKend config validation failed. Aborting."
        exit 1
      }
  fi

  echo "  -> Restarting KraKend service..."
  sudo systemctl restart krakend
  echo "  -> KraKend status:"
  systemctl is-active krakend
fi

# --- Docker services deployment ---
if [[ ${#services[@]} -gt 0 ]]; then
  cd "$COMPOSE_DIR"
  echo "==> Deploying: ${services[*]}"
  docker compose stop "${services[@]}"
  docker compose rm -f "${services[@]}"
  docker compose pull "${services[@]}"
  docker compose up -d "${services[@]}"
  echo "==> Done. Current status:"
  docker compose ps
elif [[ "$deploy_krakend" == false ]]; then
  # No --krakend flag and no services = deploy all docker services
  cd "$COMPOSE_DIR"
  echo "==> Deploying ALL services"
  docker compose down
  docker compose pull
  docker compose up -d
  echo "==> Done. Current status:"
  docker compose ps
fi
