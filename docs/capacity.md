# Capacity assumptions

## Scale story (BookMyShow-shaped)

| Metric | Example value |
|--------|----------------|
| Interested fans | 13,000,000 |
| Seats | 20,000 |
| Waiting room cap | 1,000,000 concurrent entries |
| Admit rate | 10,000 / minute (~167/sec into checkout) |
| Peak join RPS (simulated) | 50,000–200,000 at T+0 (k6 scaled locally) |

## Math

- 13M interested, 20k seats → **99.85%** must be turned away gracefully.
- At 10k admits/min, draining 1M queue positions ≈ **100 minutes** (acceptable for a multi-hour sale window).
- Join path is O(1) Redis INCR + single Postgres insert.

## Local load test equivalence

`launch_spike` uses 500 VUs — document your machine results in `docs/load-test-results/` and extrapolate with linear RPS estimates, not “we ran 13M users.”

## SLO targets (MVP)

| Endpoint | p99 target | Error budget |
|----------|------------|----------------|
| `waiting-room/join` | &lt; 500ms | &lt; 1% non-429 errors |
| `waiting-room/status` | &lt; 200ms | &lt; 0.5% |
| Checkout (admitted) | &lt; 1s | 0% oversell |
