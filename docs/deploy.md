# Deploy Traffic for shared testing

This guide deploys the **Load Control Center** (UI + API + k6 runner) so others can run scenarios without slowing the service for everyone.

## What keeps it fast

The app is built for high concurrency (Redis queue sequencing, rate limits, waiting room caps). For **shared demo hosting**, these extra guards apply:

| Guard | Default | Effect |
|-------|---------|--------|
| `LOADTEST_MAX_CONCURRENT` | 1 | Only one k6 run at a time |
| `LOADTEST_MAX_QUEUE` | 3 | At most a few queued jobs |
| `LOADTEST_MAX_VUS` | 150 | Caps virtual users per run |
| `LOADTEST_MAX_STEADY_SEC` | 90 | Shortens long steady phases |
| Single-flight runner | — | `loadtest-runner` never starts a second k6 while one is running |
| Redis memory cap | 256MB | LRU eviction on cache keys |
| Optional `DEMO_API_KEY` | — | Blocks anonymous abuse of demo/loadtest APIs |

The ticketing API (`/v1/sales/...`, auth) stays available while k6 runs; heavy load is serialized so the VM is not flooded with parallel k6 processes.

---

## Option A — VPS + Docker Compose (recommended)

Best when you need the full UI + k6 sidecar with a shared volume for job queues.

### 1. Provision a server

- **Provider**: DigitalOcean, Hetzner, AWS Lightsail, etc.
- **Size**: 2 vCPU / 4 GB RAM minimum (4 vCPU / 8 GB if many concurrent UI users)
- **OS**: Ubuntu 22.04+ or Debian 12
- Open firewall: **80/443** (and optionally **8080** if you skip a reverse proxy)

### 2. Install Docker on the server

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# log out and back in
```

### 3. Clone the repo on the server

```bash
git clone <your-repo-url> traffic
cd traffic
```

### 4. Configure secrets

```bash
cp deploy/.env.example deploy/.env
nano deploy/.env
```

Generate strong values:

```bash
openssl rand -hex 32   # use for JWT_SECRET, ADMIN_API_KEY, DEMO_API_KEY, POSTGRES_PASSWORD
```

Set at minimum:

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `ADMIN_API_KEY`
- `DEMO_API_KEY` (share this only with testers)

Tune caps if the machine is small (e.g. `LOADTEST_MAX_VUS=80`).

### 5. Start the stack

```bash
make up-prod
# or:
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build
```

Check health:

```bash
curl -s http://localhost:8080/health/ready
curl -s http://localhost:8080/v1/demo/config | jq .
```

### 6. Put HTTPS in front (production)

Use Caddy or nginx on the same host:

**Caddy example** (`/etc/caddy/Caddyfile`):

```
traffic.yourdomain.com {
  reverse_proxy localhost:8080
}
```

Reload Caddy, then share: `https://traffic.yourdomain.com`

### 7. Share with testers

1. Send them the URL.
2. If `DEMO_API_KEY` is set, send them the key — they enter it in **Demo access key** on the UI (stored in the browser session).
3. Remind them: only **one** load test runs at a time; if they see “queue is full”, wait ~1–2 minutes.

### 8. Operations

```bash
# Logs
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env logs -f api loadtest-runner

# Restart after env change
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build

# Stop
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env down
```

Metrics: `https://your-domain/metrics` (restrict in nginx if public).

---

## Option B — Local machine tunnel (quick share)

For a short demo without a VPS:

```bash
make up-prod   # with deploy/.env on your laptop
```

Then use [ngrok](https://ngrok.com/) or Cloudflare Tunnel:

```bash
ngrok http 8080
```

Share the HTTPS URL + `DEMO_API_KEY`. Keep the same caps in `.env` so your laptop stays responsive.

---

## Option C — API-only (no k6 UI runs)

If you only need the ticketing API (no Load Control Center runs), deploy **api + worker + postgres + redis** and omit `loadtest-runner`. Users can still register, join the queue, and checkout; they cannot start k6 from the UI.

Remove the `loadtest-runner` service from `docker-compose.prod.yml` or do not start it.

---

## Environment reference

| Variable | Purpose |
|----------|---------|
| `HTTP_PORT` | Host port mapped to API (default 8080) |
| `JWT_SECRET` | User session tokens |
| `ADMIN_API_KEY` | Admin routes + k6 runner → API |
| `DEMO_API_KEY` | Protects `/v1/demo/*` when set |
| `LOADTEST_MAX_VUS` | Server-side VU cap |
| `LOADTEST_MAX_QUEUE` | Max queued + running load tests |
| `LOADTEST_MAX_CONCURRENT` | Parallel k6 runs (keep at 1) |
| `LOADTEST_MAX_STEADY_SEC` | Max k6 steady duration |
| `DB_MAX_CONNS` | Postgres pool size for API |

---

## Sizing guide

| Concurrent testers | Suggested VM | `LOADTEST_MAX_VUS` |
|--------------------|--------------|---------------------|
| 1–5 | 2 vCPU / 4 GB | 150 |
| 5–20 | 4 vCPU / 8 GB | 150–200 |
| Demo only (no k6) | 2 vCPU / 2 GB | N/A |

If p99 latency rises during a run, lower `LOADTEST_MAX_VUS` or run fewer scenarios with high VU presets.

---

## Security checklist

- [ ] Change all secrets in `deploy/.env`; never commit `.env`
- [ ] Set `DEMO_API_KEY` for any public URL
- [ ] Use HTTPS in front of the API
- [ ] Do not expose Postgres/Redis ports publicly (prod compose does not publish them)
- [ ] Rotate `JWT_SECRET` / keys if leaked

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Load test stuck on “Queued” | Ensure `loadtest-runner` container is running: `docker ps` |
| “load test queue is full” | Wait for current k6 run to finish (~1–3 min) |
| 401 on Start load test | Enter correct **Demo access key** |
| Slow API during tests | Lower `LOADTEST_MAX_VUS`; only one k6 at a time is by design |
| `health/ready` fails | Check postgres/redis logs |
