#!/usr/bin/env bash
set -euo pipefail
BASE="${BASE_URL:-http://localhost:8080}"
ADMIN_KEY="${ADMIN_KEY:-dev-admin-key}"

echo "Seeding demo sale at $BASE"

EVENT=$(curl -s -X POST "$BASE/v1/admin/events" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"Arena Night","venue":"Stadium","starts_at":"2026-12-01T20:00:00Z"}')
EVENT_ID=$(echo "$EVENT" | python3 -c "import sys,json; print(json.load(sys.stdin)['event_id'])")

OPENS=$(date -u -v-1M +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '1 min ago' +%Y-%m-%dT%H:%M:%SZ)
ENDS=$(date -u -v+7d +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '7 days' +%Y-%m-%dT%H:%M:%SZ)

SALE=$(curl -s -X POST "$BASE/v1/admin/sales" \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"event_id\":\"$EVENT_ID\",\"opens_at\":\"$OPENS\",\"ends_at\":\"$ENDS\",\"total_seats\":500,\"admit_per_minute\":6000,\"waiting_room_cap\":100000}")
SALE_ID=$(echo "$SALE" | python3 -c "import sys,json; print(json.load(sys.stdin)['sale_id'])")

echo "SALE_ID=$SALE_ID"
echo "EVENT_ID=$EVENT_ID"
echo "$SALE_ID" > /tmp/traffic_sale_id
echo "export SALE_ID=$SALE_ID"
