#!/usr/bin/env bash
# One-command local lab: API + UI + k6 runner (Docker Desktop / Engine).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$ROOT/deploy/docker-compose.desktop.yml"
ENV_FILE="$ROOT/deploy/.env.desktop"

cd "$ROOT"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed."
  echo "Install Docker Desktop: https://www.docker.com/products/docker-desktop/"
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose (v2) is required. Update Docker Desktop."
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  cp "$ROOT/deploy/.env.desktop.example" "$ENV_FILE"
  echo "Created $ENV_FILE with defaults."
fi

# shellcheck disable=SC1090
set -a
source "$ENV_FILE"
set +a

HTTP_PORT="${HTTP_PORT:-8080}"

echo "Building / starting Traffic lab (first run may take a few minutes)..."
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --build

echo ""
echo "Waiting for API..."
for i in $(seq 1 60); do
  if curl -sf "http://127.0.0.1:${HTTP_PORT}/health/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

if curl -sf "http://127.0.0.1:${HTTP_PORT}/health/ready" >/dev/null 2>&1; then
  echo ""
  echo "Traffic is ready."
  echo "  Load Control Center: http://localhost:${HTTP_PORT}"
  echo ""
  echo "Stop:  docker compose -f deploy/docker-compose.desktop.yml --env-file deploy/.env.desktop down"
  echo "Logs:  docker compose -f deploy/docker-compose.desktop.yml --env-file deploy/.env.desktop logs -f api loadtest-runner"
else
  echo "API not healthy yet. Check logs:"
  echo "  docker compose -f deploy/docker-compose.desktop.yml --env-file deploy/.env.desktop logs api"
  exit 1
fi
