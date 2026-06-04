#!/usr/bin/env bash
# Create dist/traffic-desktop.zip for testers (pull images + docker compose, no git clone).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="$ROOT/dist/traffic-desktop"
ZIP="$ROOT/dist/traffic-desktop.zip"

rm -rf "$OUT_DIR" "$ZIP"
mkdir -p "$OUT_DIR/migrations"

cp "$ROOT/deploy/docker-compose.release.yml" "$OUT_DIR/docker-compose.yml"
cp "$ROOT/deploy/.env.desktop.example" "$OUT_DIR/.env"
cp "$ROOT/migrations/001_init.sql" "$OUT_DIR/migrations/001_init.sql"

cat > "$OUT_DIR/run.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"

if ! command -v docker >/dev/null 2>&1; then
  echo "Install Docker Desktop: https://www.docker.com/products/docker-desktop/"
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

echo "Pulling images (first time may take a few minutes)..."
docker compose --env-file .env pull

echo "Starting Traffic lab..."
docker compose --env-file .env up -d

HTTP_PORT="${HTTP_PORT:-8080}"
for i in $(seq 1 60); do
  if curl -sf "http://127.0.0.1:${HTTP_PORT}/health/ready" >/dev/null 2>&1; then
    echo ""
    echo "Ready: http://localhost:${HTTP_PORT}"
    exit 0
  fi
  sleep 2
done

echo "API not ready. Logs: docker compose --env-file .env logs api"
exit 1
EOF

cat > "$OUT_DIR/README.txt" <<'EOF'
Traffic — run on your PC (Docker)

1. Install Docker Desktop
2. Unzip this folder
3. Edit .env — set TRAFFIC_API_IMAGE and TRAFFIC_LOADTEST_IMAGE (from release notes)
4. Run:  chmod +x run.sh && ./run.sh
5. Open: http://localhost:8080

Stop: docker compose down
EOF

chmod +x "$OUT_DIR/run.sh"
mkdir -p "$ROOT/dist"
(cd "$ROOT/dist" && zip -rq traffic-desktop.zip traffic-desktop)
echo "Created $ZIP"
