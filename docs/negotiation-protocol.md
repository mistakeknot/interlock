# Release negotiation protocol

interlock lets one agent ask another to give up a file reservation, and lets the asker take the file after a bounded wait if the holder never answers. This document is the reference for that exchange: who sends what, in which order, with which timeouts. Tool names are interlock MCP tools; every message travels as an intermute message in one thread.

## Roles and transport

- **Requester** — the agent that wants a file another agent has reserved.
- **Holder** — the agent whose active reservation matches the requested pattern.
- **Thread** — one intermute message thread per negotiation, id `negotiate-<uuid>`. Every message in the exchange carries this `thread_id`, so either side can reconstruct the state by reading the thread.

Message bodies are JSON with a `type` field. The intermute `subject` mirrors the type, so a client that cannot parse the body still sees what it is.

| Type | Direction | Body fields |
|---|---|---|
| `release-request` | requester → holder | `file`, `reason`, `requester` (display name), `requester_id`, `holder` (agent ID), `reservation_id`, `urgency`, `thread_id` |
| `release-ack` | holder → requester (or requester → holder when forced) | `file`, `released: true`, `released_by`, `released_cnt`; on escalation also `forced: true`, `reason: "escalation-timeout"` |
| `release-defer` | holder → requester | `file`, `eta_minutes` (0–60), `reason`, `released: false` |

## Sequence

```
requester                         intermute                          holder
   |  negotiate_release(holder, file, reason, urgency)                 |
   |-- check: holder has an active reservation matching file --------->|
   |-- release-request (thread T) ------------------------------------>|
   |                                                                   |  fetch_inbox
   |                                                                   |  respond_to_release(T, action)
   |<-- release-ack  (holder released every reservation matching file) |
   |      or                                                           |
   |<-- release-defer (eta_minutes, reason) ---------------------------|
   |                                                                   |
   |  (no answer within the timeout window)                            |
   |  force_release_negotiation(T, file, reason)                       |
   |-- validate window elapsed or holder dead; release holder's match  |
   |-- release-ack {forced} ------------------------------------------>|
```

### 1. Request

`negotiate_release(agent_name, file, reason, urgency="normal", wait_seconds=0)`

- Preconditions: `agent_name` (name or id) holds an active reservation whose pattern conflicts with `file`. Otherwise `NOT_FOUND`, and nothing is sent.
- `urgency` is `normal` or `urgent`. Urgent requests are sent with intermute importance `urgent` and `ack_required`, so they surface in the holder's stale-ack list if ignored.
- With `wait_seconds = 0` the call returns `{status: "pending", thread_id, to, urgency}` at once.
- With `wait_seconds > 0` the requester polls the thread every 2 seconds until an answer or the deadline, then returns one of:
  - `released` — a `release-ack` arrived (`released_by`).
  - `deferred` — a `release-defer` arrived (`eta_minutes`, `reason`).
  - `timeout` — nothing arrived; the result carries `can_escalate: true` and an `escalation_hint`.
- The newest message in the thread wins: a defer followed by an ack reads as released.

### 2. Response

`respond_to_release(thread_id, requester, action, file, eta_minutes?, reason?)`

- `action = "release"`: the holder releases **every** reservation of its own that matches `file`, then sends `release-ack` with the count. Release happens before the ack is sent, so an ack always means the file is free.
- `action = "defer"`: the holder keeps the reservation and sends `release-defer`. `eta_minutes` is clamped to 0–60. A deferral is information for the requester, not a lease extension: it does not move the escalation window (see below).

### 3. Escalation

`force_release_negotiation(thread_id, file, reason)`

Preconditions, all checked by the requester's client against the thread as stored in intermute:

1. The thread exists and contains a `release-request`; its recipient is the holder.
2. No `release-ack` is already in the thread (if there is one, the file is free and the call fails with "already has a release-ack").
3. Either the holder is **dead** — no intermute heartbeat for 5 minutes — or the window has elapsed: **10 minutes** after the request for `normal`, **5 minutes** for `urgent`. A holder that answered with a defer is still subject to the same window; the ETA is advisory.

4. The caller is the requester recorded on the thread.

If the preconditions hold, the requester releases the pinned reservation (see § Reservation pinning), sends a `release-ack` with `forced: true` to the holder (only when something was actually released, to avoid thread noise), records an `escalation` signal, and returns `force_released`; if the pinned reservation is gone or held by someone else it returns `reservation_changed` and releases nothing (`already_released` is only reachable for threads from older clients that carry no reservation id). `reason` is stored for the audit trail.

If intermute cannot be reached for the liveness check, the call falls back to the time window alone; it never blocks on the failure.

## Timeouts at a glance

| Urgency | Window before force is allowed | Message importance |
|---|---|---|
| normal | 10 minutes | normal |
| urgent | 5 minutes | urgent, ack required |
| holder dead (no heartbeat 5 min) | none | — |

Poll interval while waiting: 2 seconds. Maximum deferral ETA: 60 minutes.

## Enforcement boundary

Negotiation is cooperative; enforcement is layered so that a cooperative failure cannot silently corrupt a checkout:

- The `PreToolUse:Edit` hook **blocks** an edit to a file another agent holds exclusively (`hooks/pre-edit.sh`). It downgrades to a warning in two cases: the optional tier-2 region check decides the edit does not overlap the holder's stated work, or intermute is unreachable, in which case the hook proceeds and says so. Hooks fail open by design: a dead coordination server never wedges a session.
- The git pre-commit hook (`scripts/interlock-precommit-hook`) **blocks** a commit that touches a file another agent has reserved. It is the backstop for edits that bypassed the first hook.
- Timeouts are never enforced in the background. An earlier version force-released reservations from a goroutine when a negotiation timed out; that produced a read-then-delete race and released files under a holder mid-edit. Escalation is now an explicit call by the requester, validated at call time (see `docs/design/advisory-only-timeouts.md`).

## Reservation pinning

A negotiation is about one reservation, not a pattern. The `release-request` carries the `reservation_id` the requester saw when it checked conflicts, and the holder's agent ID. Escalation releases exactly that reservation, and only if it is still active and still held by that agent. If the holder has released it and reserved something else in the meantime, escalation reports `reservation_changed` and releases nothing: the requester re-checks conflicts and, if needed, starts a new negotiation. Only the requester recorded on the thread may escalate it; only the holder it was sent to may respond to it.

## Deprecated

`request_release` sends a one-shot `release-request` with no thread and no timeout. It exists for older clients; new code uses `negotiate_release`.
