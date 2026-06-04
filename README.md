# Traffic — Concert Ticketing (BookMyShow-Proof)

High-concurrency concert ticket sales with a **server-authoritative waiting room**, anti-scalping controls, and **reproducible load tests** for every major failure mode from the BookMyShow sale incidents.

## Problems solved

| Issue | Fix | Test scenario |
|-------|-----|---------------|
| Site crash at sale open | Virtual waiting room + admission drip + rate limits | `launch_spike` |
| HTML/API backdoor | No client queue secrets; phase-gated join; admission JWT for checkout | `backdoor_race` |
| Late users got ahead | Atomic FIFO position at join; idempotent re-join | `queue_fairness`, `late_surge`, `second_sale_regression` |
| Black market / bots | Non-transferable tickets, caps, rate limits | `bot_checkout`, `scalper_network` |
| Refresh loses queue spot | Position stored server-side per account | `refresh_storm`, `reconnect_churn` |

## Quick start (Docker on your PC)

**Easiest — one command:**

```bash
git clone <repo-url> traffic && cd traffic
make desktop
open http://localhost:8080
```

Or: `./scripts/run-traffic.sh` — see **[Run with Docker](docs/run-with-docker.md)** for zip download, pre-built images, and sharing with testers.

**Developers (same stack, manual compose):**

```bash
make up          # postgres, redis, api, worker, loadtest-runner
open http://localhost:8080   # Load Control Center UI
```

The UI lets you pick a scenario, set **peak VUs**, ramp timings, and sale capacity (admit rate, waiting room cap, seats), then **Start load test**. k6 runs in the `loadtest-runner` sidecar; results show **PASS/FAIL** against SLO thresholds. The black sidebar shows live queue depth, admits, and a queue sparkline during the run.

```bash
make seed        # optional CLI demo sale
make assert-queue SALE_ID=$SALE_ID
```

## Load Control Center (UI)

1. Open http://localhost:8080
2. Use a **preset** (T+0 launch, Refresh storm, Fairness audit, …) or configure manually
3. **Reset sale** for a clean queue (optional; auto-created on first run)
4. **Start load test** — poll status until PASS/FAIL
5. Watch **Live performance** while k6 runs

### Tunables

| Control | Env (CLI) | Default |
|---------|-----------|---------|
| Peak VUs | `K6_VUS_MAX` | 500 |
| Ramp up / steady / down | `K6_RAMP_UP`, `K6_STEADY`, `K6_RAMP_DOWN` | 10s / 30s / 10s |
| Admit rate | `K6_ADMIT_PER_MIN` | 6000 |
| Waiting room cap | `K6_WAITING_ROOM_CAP` | 100000 |
| Total seats | `K6_TOTAL_SEATS` | 500 |

## Run load tests (CLI)

```bash
make up
make sim SCENARIO=launch_spike
make sim SCENARIO=launch_spike K6_VUS_MAX=200 K6_STEADY=20s
```

Requires the `loadtest` compose profile (included in `make up`).

After a test, verify queue invariants:

```bash
go run ./cmd/assert-queue -sale $SALE_ID
curl -s -H "X-Admin-Key: dev-admin-key" http://localhost:8080/v1/admin/sales/$SALE_ID/queue-entries | jq .
```

## API highlights

- `POST /v1/auth/register` — create account
- `POST /v1/sales/{id}/waiting-room/join` — join queue (idempotent)
- `GET /v1/sales/{id}/waiting-room/status` — poll; **refreshing does not change position**
- Seat map / holds / checkout require `Authorization: Admission <jwt>` from status when admitted
- `POST /v1/tickets/transfer` — always **403** (non-transferable)

**Demo lab API (local dev only):**

- `POST /v1/demo/reset` — fresh demo sale with optional capacity params
- `POST /v1/demo/loadtest` — queue a k6 run (`scenario`, `vus_max`, ramps, sale settings)
- `GET /v1/demo/loadtest/{runId}` — run status + PASS/FAIL + metrics
- `GET /v1/demo/loadtests` — recent runs + scenario list
- `GET /v1/demo/dashboard` — live queue depth, tickets, error rate

Admin (`X-Admin-Key: dev-admin-key`): `POST /v1/admin/events`, `POST /v1/admin/sales`, stats, queue audit

## Observability

- Metrics: http://localhost:8080/metrics
- Health: `/health/live`, `/health/ready`
- k6 summaries: Docker volume `loadtest_results` (also surfaced via API)

## Share with others

- **Git clone on each PC (easiest):** **[Share with testers](docs/share-with-testers.md)** — `git clone` → `make desktop`
- **Pre-built images + zip:** **[Run with Docker](docs/run-with-docker.md)**
- **Cloud (optional):** **[Deploy guide](docs/deploy.md)** or **[Oracle Always Free](docs/deploy-oracle-free.md)**

## Docs

- [How it works](docs/how-it-works.md) — plain-language overview of the system and anti-crash logic
- [Deploy for shared testing](docs/deploy.md)
- [BookMyShow postmortem mapping](docs/bookmyshow-postmortem.md)
- [Capacity assumptions](docs/capacity.md)
- [Load test results template](docs/load-test-results/README.md)
- [ADRs](docs/adr/)

## Stack

Go, PostgreSQL, Redis, chi, Prometheus, k6, Docker Compose.
# traffic-backend-queue
