# Load test results

## UI-driven runs

After a run from the **Load Control Center** (http://localhost:8080), note:

- Scenario name and peak VUs
- PASS/FAIL and failure reasons from the results card
- p99 latency and error rate from the run summary

Paste a short record here for docs or regression tracking:

```text
Date: 2026-06-02
Scenario: launch_spike
VUs: 500
Ramp: 10s / steady 30s / down 10s
Sale: <sale_id>
Result: PASS
p99: 312ms
Error rate: 0.2%
Notes: queue depth peaked at 48k; no 5xx on join
```

## CLI runs

After `make sim SCENARIO=...`, copy k6 console output or read the result file:

```bash
docker compose -f deploy/docker-compose.yml exec api ls /loadtest/results
# Or poll API:
curl -s http://localhost:8080/v1/demo/loadtests | jq .
```

## Thresholds

SLO defaults live in [scripts/traffic/config.yaml](../../scripts/traffic/config.yaml) and are evaluated in [scripts/traffic/thresholds.js](../../scripts/traffic/thresholds.js).

| Scenario | Key gates |
|----------|-----------|
| `launch_spike` | error rate &lt; 1%, join p99 &lt; 500ms |
| `queue_fairness` | zero position inversions |
| `late_surge` / `second_sale_regression` | late batch must not leapfrog early batch |
