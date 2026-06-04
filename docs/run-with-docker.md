# Run Traffic on your PC (Docker)

No cloud deploy needed. Testers install **Docker Desktop**, then start the full lab (UI + API + k6) locally.

## For testers (easiest)

### Option A — Clone and run (builds images on first start)

```bash
git clone <repo-url> traffic
cd traffic
chmod +x scripts/run-traffic.sh
./scripts/run-traffic.sh
```

Open **http://localhost:8080**

### Option B — Download zip + pull images (no build)

1. Get `traffic-desktop.zip` from a [GitHub Release](https://github.com) (or run `./scripts/package-desktop.sh` as maintainer).
2. Unzip, edit `.env` with image names from the release notes.
3. Run:

```bash
chmod +x run.sh
./run.sh
```

### Option C — Pull images only (Docker Hub / GHCR)

If images are published (example names):

```bash
docker pull traffic-lab/api:latest
docker pull traffic-lab/loadtest:latest
```

Then use Option A or B with:

```env
TRAFFIC_API_IMAGE=traffic-lab/api:latest
TRAFFIC_LOADTEST_IMAGE=traffic-lab/loadtest:latest
```

## Requirements

- **Docker Desktop** (Mac/Windows) or Docker Engine (Linux)
- **4 GB RAM** free for Docker (8 GB recommended for k6 runs)
- Ports **8080** available

## Stop / reset

```bash
docker compose -f deploy/docker-compose.desktop.yml --env-file deploy/.env.desktop down
```

Remove all data (fresh DB):

```bash
docker compose -f deploy/docker-compose.desktop.yml --env-file deploy/.env.desktop down -v
```

## Laptop-friendly defaults

Desktop compose caps load tests so one machine stays responsive:

- Max **200** VUs (configurable in `deploy/.env.desktop`)
- **One** k6 run at a time
- Short max steady phase

## For maintainers — publish images

```bash
chmod +x scripts/build-images.sh scripts/package-desktop.sh

# Local build (Mac Apple Silicon + Intel)
./scripts/build-images.sh

# Push to Docker Hub or GHCR
REGISTRY=docker.io/yourusername TAG=v0.1.0 PUSH=1 ./scripts/build-images.sh

# Zip for release assets
./scripts/package-desktop.sh
# Upload dist/traffic-desktop.zip to GitHub Releases
```

Update `.env` in the zip with the real `TRAFFIC_*_IMAGE` URLs after pushing.

## Troubleshooting

| Issue | Fix |
|-------|-----|
| Port 8080 in use | Set `HTTP_PORT=8081` in `deploy/.env.desktop` |
| Load test stuck queued | `docker ps` — `loadtest-runner` must be running |
| Slow during k6 | Lower `LOADTEST_MAX_VUS` in `.env.desktop` |
| Apple Silicon build slow | First `./scripts/run-traffic.sh` builds once; later starts are fast |
