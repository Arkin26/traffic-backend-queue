# ADR 001: Server-authoritative waiting room

## Status

Accepted

## Context

Client-side queues expose backdoors and break on refresh. BookMyShow’s HTML leak let users skip the visible UI.

## Decision

- Position assigned only by server at `POST .../join`.
- Stored in Redis (`INCR`) and Postgres (`queue_entries`).
- Idempotent join per `(sale_id, user_id)`.

## Consequences

- Frontend is a thin poll/SSE client.
- Fairness testable via `assert-queue` and k6 scenarios.
