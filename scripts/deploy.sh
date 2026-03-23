#!/usr/bin/env bash
set -euo pipefail

COMPOSE_DIR="$HOME/services/nouveauricheglobalgroup"

usage() {
  cat <<EOF
Usage: $(basename "$0") [service...]

Stops, pulls, and restarts specified Docker Compose services.
If no services are specified, operates on ALL services.

Examples:
  $(basename "$0")                              # all services
  $(basename "$0") subscription-external        # single service
  $(basename "$0") subscription-external landing-web  # multiple services
EOF
  exit 0
}

[[ "${1:-}" == "-h" || "${1:-}" == "--help" ]] && usage

cd "$COMPOSE_DIR"

if [[ $# -eq 0 ]]; then
  echo "==> Deploying ALL services"
  docker compose down
  docker compose pull
  docker compose up -d
else
  echo "==> Deploying: $*"
  docker compose stop "$@"
  docker compose rm -f "$@"
  docker compose pull "$@"
  docker compose up -d "$@"
fi

echo "==> Done. Current status:"
docker compose ps
