# How it works

Plain-language guide to what Traffic is and how it stays up under pressure. Come back here anytime you need a refresher.

---

## What we built

Traffic is a **practice ticket-selling system** — a lab for high-traffic concert sales, inspired by real incidents (e.g. BookMyShow-style crashes and queue unfairness).

You get:

- A **ticket sale** with a waiting room, checkout, and anti-bot rules  
- A **Load Control Center** (web UI at http://localhost:8080) to run serious load tests  
- **Fake buyers** (k6) that flood the system — hundreds or thousands at once  
- **Live stats**: queue size, how many admitted, tickets sold, errors, speed  
- **PASS / FAIL** when a test ends, against simple performance targets  

You pick a scenario (launch rush, refresh spam, bots, fairness checks, etc.), set how many virtual users and sale settings, and press **Start**. No need to manually “join the queue” as one fan — the lab is built for **system testing**, not demo shopping.

**Stack (for reference):** Go API, PostgreSQL, Redis, background worker, k6 load tests, Docker Compose.

---

## How we avoid crashing

Picture a **nightclub with one door** — not everyone charging the bar at once.

### 1. Waiting room (queue)

When the sale opens, users don’t all hit “buy” at once. They **join a line** and get a **position number**. That number lives on the **server**, not in the browser.

### 2. Let people in slowly (admission drip)

Only a **fixed number per minute** may enter checkout. Everyone else waits. That stops checkout and the database from being overwhelmed at second zero.

### 3. Fair line (FIFO)

Who joined first stays ahead. **Refreshing the page does not improve your place.** Reconnecting keeps the same spot (idempotent join).

### 4. No skipping the line

Checkout and seat holds require an **admission token** issued only when you’re actually admitted. Random API calls without that token are **rejected**.

### 5. Bots and abuse

Rate limits, caps per user, non-transferable tickets, and blocked checkout paths slow down scalpers and automated abuse.

### 6. Test before you trust it

The **Load Control Center** runs heavy fake traffic so you can see whether the queue, admits, and blocks still work **before** a real on-sale.

**In one sentence:** Fake buyers line up in a server-controlled queue; only a controlled drip is let in to buy; cheats and refresh tricks can’t jump the line; and you can crank up traffic in the lab to see if it holds.

---

## What each test scenario is checking

| Scenario | What it simulates | What “good” means |
|----------|-------------------|-------------------|
| `launch_spike` | Everyone hits at T+0 | Site stays up; joins aren’t mostly errors |
| `queue_fairness` | Many joins in order | Later joins don’t get better positions than earlier ones |
| `refresh_storm` | Users hammer refresh | Position doesn’t change |
| `late_surge` / `second_sale_regression` | Late wave of joiners | Late users don’t leapfrog early users |
| `backdoor_race` | Direct checkout without queue | Blocked |
| `bot_checkout` / `scalper_network` | Bots and resale tricks | Blocked or rate-limited |
| `reconnect_churn` | Drop off and come back | Same queue position |
| `mixed_realistic` | Mix of humans and aggressive bots | System stays controlled |

---

## How a load test run works (UI)

1. **Reset sale** (optional) — fresh concert sale with your admit rate, cap, and seat count  
2. **Start load test** — API writes a job; `loadtest-runner` picks it up and runs k6  
3. Status: **Queued** → **Running** → **Passed** or **Failed**  
4. Sidebar updates live while k6 runs; result card shows p99, error rate, and failure reasons  

```bash
make up    # starts API, DB, Redis, worker, loadtest-runner
open http://localhost:8080
```

---

## Deeper technical docs

- [BookMyShow postmortem mapping](bookmyshow-postmortem.md) — incident → fix → test  
- [Capacity assumptions](capacity.md) — scale math and SLO targets  
- [ADR: server-authoritative queue](adr/001-server-authoritative-queue.md) — why the queue lives on the server  
- [README](../README.md) — commands, API list, CLI load tests  
