#!/usr/bin/env bash
# Build multi-arch images for testers who only "docker pull".
# Usage:
#   ./scripts/build-images.sh
#   REGISTRY=ghcr.io/youruser TAG=v0.1.0 PUSH=1 ./scripts/build-images.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REGISTRY="${REGISTRY:-traffic-lab}"
TAG="${TAG:-latest}"
PUSH="${PUSH:-0}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"

API_IMAGE="${REGISTRY}/api:${TAG}"
LOADTEST_IMAGE="${REGISTRY}/loadtest:${TAG}"

cd "$ROOT"

if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx is required."
  exit 1
fi

docker buildx inspect traffic-builder >/dev/null 2>&1 || \
  docker buildx create --name traffic-builder --use

extra_flags=()
if [[ "$PUSH" == "1" ]]; then
  extra_flags=(--push)
else
  extra_flags=(--load)
fi

docker buildx build \
  --platform "$PLATFORMS" \
  -f deploy/Dockerfile \
  -t "$API_IMAGE" \
  "${extra_flags[@]}" \
  .

docker buildx build \
  --platform "$PLATFORMS" \
  -f deploy/Dockerfile.loadtest-runner \
  -t "$LOADTEST_IMAGE" \
  "${extra_flags[@]}" \
  .

echo ""
echo "Built:"
echo "  $API_IMAGE"
echo "  $LOADTEST_IMAGE"
if [[ "$PUSH" == "1" ]]; then
  echo ""
  echo "Testers set in .env:"
  echo "  TRAFFIC_API_IMAGE=$API_IMAGE"
  echo "  TRAFFIC_LOADTEST_IMAGE=$LOADTEST_IMAGE"
fi
