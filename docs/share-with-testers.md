# Share Traffic with testers (Option 1 — git clone)

Give testers these steps. They run everything **locally** on their PC; nothing is hosted in the cloud.

## What testers need

- A Mac, Windows, or Linux PC
- **Docker Desktop** installed and **running** (whale icon in the menu bar / system tray)
- **Git** ([download](https://git-scm.com/downloads))
- ~10 minutes on first run (Docker downloads/builds images)

## Steps for testers

### 1. Install Docker Desktop

https://www.docker.com/products/docker-desktop/

Open Docker Desktop and wait until it says **Engine running**.

### 2. Clone the repo

Replace `YOUR_GITHUB_USER` and repo name with yours:

```bash
git clone https://github.com/YOUR_GITHUB_USER/traffic.git
cd traffic
```

### 3. Start the lab

```bash
make desktop
```

Or without Make:

```bash
chmod +x scripts/run-traffic.sh
./scripts/run-traffic.sh
```

### 4. Open the UI

http://localhost:8080

Use presets (T+0 launch, Fairness audit, etc.) → **Start load test** → watch results.

### 5. Stop when done

```bash
docker compose -f deploy/docker-compose.desktop.yml --env-file deploy/.env.desktop down
```

---

## Copy-paste message for Slack / email

```
Traffic load-test lab (runs on your machine):

1. Install Docker Desktop and start it
2. git clone https://github.com/YOUR_GITHUB_USER/traffic.git
3. cd traffic && make desktop
4. Open http://localhost:8080

First start takes a few minutes. Only one heavy k6 run at a time so laptops stay responsive.
Stop: docker compose -f deploy/docker-compose.desktop.yml --env-file deploy/.env.desktop down
```

---

## Troubleshooting (for testers)

| Problem | Fix |
|---------|-----|
| `Cannot connect to Docker` | Start Docker Desktop |
| Port 8080 already in use | Add `HTTP_PORT=8081` to `deploy/.env.desktop`, use http://localhost:8081 |
| Load test stays "Queued" | Run `docker ps` — need container `loadtest-runner` running; re-run `make desktop` |
| `make: command not found` | Use `./scripts/run-traffic.sh` instead |

---

## For you (maintainer) before sharing

1. Push the `traffic` project to its **own** GitHub repo (see below).
2. Run `make desktop` once on your machine to confirm it works.
3. Send testers the clone URL + steps above.
