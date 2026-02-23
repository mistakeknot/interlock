# Plan: Escalation Timeout for Unresponsive Agents (iv-2jtj)

**Bead:** iv-2jtj
**Date:** 2026-02-23
**Status:** Plan

## Problem

When `negotiate_release` times out (holder doesn't respond), the requester gets `{"status":"timeout"}` but has **no tool to act on it**. The requester must manually figure out what to do next. There's no escalation path.

The system is deliberately **advisory-only** — `CheckExpiredNegotiations` reports timeouts but does NOT auto-force-release (this was a conscious design choice to eliminate 4 P0 bugs from the original force-release approach). The escalation tool must respect this: the **requester explicitly decides** to force-release.

## Design

### New MCP tool: `force_release_negotiation`

A requester-initiated tool that force-releases a reservation after a negotiation has timed out. This is the explicit escalation step — the requester sees the timeout advisory and consciously decides to force.

**Parameters:**
- `thread_id` (required) — The negotiation thread that timed out
- `file` (required) — The file pattern being negotiated
- `reason` (required) — Why force-releasing (audit trail)

**Pre-conditions (validated server-side):**
1. The negotiation thread exists and contains a `release-request`
2. The request has exceeded its timeout window (5min urgent / 10min normal)
3. No `release-ack` exists in the thread (holder hasn't already responded)

If any pre-condition fails, the tool returns an error — no silent force-release.

**On success:**
1. Call `ReleaseByPattern` to delete the holder's reservation
2. Send `release-ack` with `reason: "escalation-timeout"` and `forced: true` to the thread
3. Notify the holder agent that their reservation was force-released
4. Return the count of released reservations

### Changes to `negotiate_release`

When `negotiate_release` returns `status: "timeout"`, include actionable context:
- `can_escalate: true` — signals that `force_release_negotiation` is available
- `escalation_hint` — human-readable instruction

### Changes to `fetch_inbox`

Already surfaces `negotiation_timeouts` from `CheckExpiredNegotiations`. No changes needed — the advisory info is already there.

## Tasks

### Task 1: Add `force_release_negotiation` tool (~60 lines)

**File:** `internal/tools/tools.go`

Add new function `forceReleaseNegotiation(c *client.Client) server.ServerTool` following the existing tool pattern. Register it in `RegisterAll` (bumping tool count to 12).

**Logic:**
1. Extract and validate `thread_id`, `file`, `reason`
2. Call `c.FetchThread(ctx, threadID)` to get the negotiation thread
3. Find the original `release-request` message, extract urgency
4. Verify timeout exceeded: `age > timeoutMinutes(urgency)`
5. Verify no `release-ack` exists (using existing `hasReleaseAck`)
6. Call `c.ReleaseByPattern(ctx, holderID, file)` — get released count
7. If `released > 0`: send `release-ack` with `forced: true` to thread
8. If `released == 0`: still send notification but mark as `already_released`
9. Emit signal `"escalation"` for observability
10. Return structured result

### Task 2: Add `ForceReleaseNegotiation` client method (~30 lines)

**File:** `internal/client/client.go`

Add a method that encapsulates the validation + release logic so the tools layer stays thin. The method:
- Takes `threadID`, `file`, `reason`
- Returns `(released int, holderID string, err error)`
- Validates thread state (timeout exceeded, no ack)
- Calls `ReleaseByPattern`

### Task 3: Enhance `negotiate_release` timeout response

**File:** `internal/tools/tools.go`

In the timeout return path (line ~486), add `can_escalate: true` and `escalation_hint`:
```go
return jsonResult(map[string]any{
    "status":          "timeout",
    "thread_id":       threadID,
    "waited":          waitSeconds,
    "can_escalate":    true,
    "escalation_hint": "Call force_release_negotiation with this thread_id to force-release the reservation",
})
```

### Task 4: Tests

**File:** `internal/client/client_test.go`

Add tests for `ForceReleaseNegotiation`:
- Happy path: expired negotiation → force-release succeeds
- Pre-condition: not expired yet → error
- Pre-condition: already acked → error
- Idempotency: holder already released (0 reservations) → success with `already_released`

### Task 5: Update plugin.json and AGENTS.md

- Bump description to "12 tools" in plugin.json
- Add `force_release_negotiation` to AGENTS.md tool reference

## Sequence

Tasks 2 → 1 → 3 → 4 → 5 (2 is the client method that 1 depends on; 3 is independent but logically after 1; 4 tests both; 5 is docs)

## Risk

**Low.** This follows established patterns exactly:
- Same tool registration pattern as all 11 existing tools
- Uses existing `ReleaseByPattern` (already idempotent with 404 handling)
- Uses existing `FetchThread` + `hasReleaseAck` for validation
- Advisory-only architecture preserved — requester explicitly opts in
