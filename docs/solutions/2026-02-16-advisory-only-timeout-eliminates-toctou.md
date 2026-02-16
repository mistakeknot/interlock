---
title: "Advisory-Only Timeout Eliminates TOCTOU Race in Multi-Agent Coordination"
category: concurrency
tags: [race-condition, toctou, idempotency, advisory-pattern, go]
severity: P0
discovery: flux-drive-review
applies_to: [interlock, intermute]
date: 2026-02-16
---

# Advisory-Only Timeout Eliminates TOCTOU Race

## Problem

A multi-agent file coordination system had a background goroutine that force-released reservations when negotiation timeouts expired. This single design choice caused four interrelated P0 bugs:

1. **TOCTOU Race**: The timeout checker read reservation state, decided to force-release, then issued a DELETE. Between the read and the DELETE, the holder agent re-acquired the reservation via a pre-edit hook. Result: holder has a fresh reservation but the message bus contains a stale `release-ack`, causing the requester to believe the file is free when it is not.

2. **Non-Idempotent DELETE**: `ReleaseByPattern` iterated active reservations and called DELETE for each. When two agents concurrently released overlapping reservations, the second agent received 404 errors that propagated as failures -- even though the desired end state (reservations gone) was already achieved.

3. **Consent Violation**: The background goroutine auto-released reservations on timeout without holder consent. A holder agent could be mid-edit when its reservation vanished, breaking the coordination invariant that the holder controls its own locks.

4. **Goroutine Leak**: A `sync.Once` goroutine spawned on the first `negotiate_release` call and never stopped. No context cancellation, no lifecycle management, no way to shut it down cleanly.

## Root Cause

All four bugs stem from one architectural flaw: **a background process that writes to shared mutable state (reservations and message bus) based on stale reads**. The gap between "read state" and "mutate state" is inherently racy in concurrent systems. Adding more locks or coordination to close the gap increases complexity without eliminating the fundamental problem -- there will always be an interleaving window.

## Solution

### Pattern 1: Advisory-Only Enforcement

Convert the timeout checker from a state-mutating actor to a read-only observer:

- **Remove** the `ReleaseByPattern` call from `CheckExpiredNegotiations`
- **Remove** the `release-ack` message send from the timeout path
- **Return** `Released: 0` in timeout structs -- purely informational
- **Requester** agent decides whether to act (explicit `respond_to_release` tool call)
- **Holder** agent sees advisory context in its pre-edit hook (via `INTERLOCK_AUTO_RELEASE=1` feature flag)

The function becomes a pure query: scan inbox for release-requests, check timestamps, filter for those past their urgency-based threshold, verify no `release-ack` exists in the thread, and return a list of timeout-eligible negotiations. No writes anywhere.

### Pattern 2: Idempotent DELETE with Typed Error Detection

For paths that must still perform concurrent deletes (e.g., explicit `respond_to_release`), make DELETE idempotent:

```go
if err := c.DeleteReservation(ctx, r.ID); err != nil {
    if !isNotFound(err) {
        return released, fmt.Errorf("delete reservation %q: %w", r.ID, err)
    }
    // 404 = already deleted by another goroutine/session, count as success.
}
released++
```

The `isNotFound` helper uses `errors.As` to unwrap a typed `*IntermuteError` and check its HTTP status code:

```go
func isNotFound(err error) bool {
    var ie *IntermuteError
    if errors.As(err, &ie) {
        return ie.Code == http.StatusNotFound
    }
    return false
}
```

This ensures that two agents releasing the same reservation concurrently both succeed, because the second DELETE's 404 is treated as "already done" rather than "failure."

## Key Insight

**Read-only code cannot race.** By making timeout checks purely advisory -- no writes to reservation state, no writes to the message bus -- the number of interleaving hazards drops to zero. There is no window between "read" and "write" because there is no write. The holder retains full control of its reservations, and the requester gets accurate timeout information to act on explicitly.

This is a specific instance of a general principle: **push mutation to the edges**. Instead of a background process silently mutating shared state, surface the information to the agents and let them make explicit, auditable decisions. This trades a small amount of latency (the agent must act) for total elimination of concurrency bugs.

## Reusable Pattern

**When to apply advisory-only enforcement:**

1. A background process monitors shared state for a condition (timeout, threshold, quota)
2. When the condition is met, it mutates shared state on behalf of another actor
3. The mutation races with the actor's own operations on the same state

**The fix:** Convert the background process from "detect + act" to "detect + report." The actor who owns the state performs the mutation explicitly when ready. This works when:

- The actor can tolerate brief delays in responding to the condition
- The advisory information is sufficient for the actor to make a decision
- The system can degrade gracefully if the actor ignores the advisory

**When NOT to apply:** Hard real-time systems where the actor cannot be trusted to respond, or where the condition requires sub-second enforcement that no advisory loop can guarantee.

**Complementary pattern:** When concurrent deletes are unavoidable, make them idempotent by treating 404 (or equivalent "already gone" errors) as success. Use typed errors with `errors.As` (Go) or equivalent error-narrowing in other languages to distinguish "not found" from genuine failures.

## Related

- [TOCTOU on Wikipedia](https://en.wikipedia.org/wiki/Time-of-check_to_time-of-use) -- the general class of race condition
- CRDTs solve a related problem (convergent state without coordination) but are heavier machinery than needed here
- The "outbox pattern" in event-driven systems similarly separates state mutation from notification
- interlock `pre-edit.sh` hook: advisory release-request display (feature-flagged via `INTERLOCK_AUTO_RELEASE=1`)
- interlock `CheckExpiredNegotiations` in `internal/client/client.go`: the advisory-only implementation
- interlock `ReleaseByPattern` in `internal/client/client.go`: idempotent DELETE implementation
