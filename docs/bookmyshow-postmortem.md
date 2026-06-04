# BookMyShow-style failures → our fixes

## 1. Systems crash + code tricks

**Incident:** Infrastructure collapsed when millions hit at once; some users found queue URLs in page source.

**Our fix:**
- Waiting room absorbs joins; worker drips `next_admit_position` per `admit_per_minute`.
- Join only via authenticated `POST`; no queue URL in static assets.
- Checkout requires short-lived admission JWT.

**Tests:** `launch_spike`, `backdoor_race`

## 2. Late users got ahead (2nd sale)

**Incident:** Early joiners received huge queue numbers; late logins jumped ahead.

**Our fix:**
- Redis `INCR` assigns position once; Postgres `UNIQUE (sale_id, user_id)`.
- Re-join is idempotent — same position returned.
- Admission strictly by ascending position.

**Tests:** `queue_fairness`, `late_surge`, `second_sale_regression`, `assert-queue`

## 3. Black market sentiment

**Incident:** Tickets on resale sites within hours.

**Our fix (backend):** Account-bound tickets, purchase caps, bot rate limits, transfer API blocked.

**Tests:** `scalper_network`, `bot_checkout`, admin `/stats` unique_buyers metric.

**Out of scope:** Removing third-party listings (legal/identity partners). Coupons, lottery presales, and bank-exclusive windows were **not** part of the booking failures — we skip those in the product.

## Web lab

Open http://localhost:8080 to toggle scenarios and watch the live monitor sidebar.

## 4. Anxious process (refresh / offline)

**Incident:** Users feared refresh would lose their place.

**Our fix:** Position is server state keyed by `user_id`; status poll and duplicate join return the same number; `retry_after_ms` guides polling.

**Tests:** `refresh_storm`, `reconnect_churn`
