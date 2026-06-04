#!/usr/bin/env bash
set -euo pipefail
SCENARIOS=(
  backdoor_race
  queue_fairness
  refresh_storm
  reconnect_churn
  late_surge
  second_sale_regression
  bot_checkout
  scalper_network
  launch_spike
  mixed_realistic
)
for s in "${SCENARIOS[@]}"; do
  echo "=== Running $s ==="
  make sim SCENARIO="$s" || echo "FAILED: $s"
done
if [[ -n "${SALE_ID:-}" ]]; then
  make assert-queue SALE_ID="$SALE_ID"
fi
