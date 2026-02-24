# Plan Review: Escalation Timeout for Unresponsive Agents (iv-2jtj)

**Reviewer:** Plan Reviewer (Opus)
**Plan:** `/home/mk/projects/Demarch/interverse/interlock/docs/plans/2026-02-23-escalation-timeout.md`
**Date:** 2026-02-23

---

### Findings Index

| Severity | ID | Section | Title |
|----------|-----|---------|-------|
| Critical | F1 | Task 1 | `hasReleaseAck` is unexported and inaccessible from `tools` package |
| Important | F2 | Task 2 | Client method cannot extract `holderID` from thread alone |
| Important | F3 | Task 1 | TOCTOU race between pre-condition check and force-release |
| Important | F4 | Task 1 | Plan step 3 sends notification to holder but no client method exists |
| Suggestion | F5 | Task 1 | `already_released` case sends redundant ack message |
| Suggestion | F6 | Task 4 | Missing negative test for authorization (requester identity) |
| Suggestion | F7 | Task 5 | Tool count already wrong in AGENTS.md (says 11 but should verify) |

**Verdict: needs-changes**

---

### Summary

The plan is well-structured, follows established patterns closely, and correctly preserves the advisory-only architecture. The task decomposition and sequencing are sound. However, there are two issues that will cause compilation failures (F1, F2) and one important design gap around TOCTOU races (F3) that the correctness review previously flagged in a different context. The plan also references a "notify the holder" step (F4) without specifying how. All issues are low-complexity fixes -- the plan needs a revision pass but not a redesign.

---

### Issues Found

#### F1: `hasReleaseAck` is unexported -- tools package cannot call it [Critical]

**Location:** Plan Task 1, step 5; Task 2 line 74

The plan says:
> 5. Verify no `release-ack` exists (using existing `hasReleaseAck`)

`hasReleaseAck` is defined in `internal/client/client.go:608` as a lowercase function:
```go
func hasReleaseAck(messages []Message) bool {
```

The `tools` package (`internal/tools/tools.go`) is a **separate Go package** from `internal/client`. Unexported functions cannot be called across package boundaries.

The plan routes this logic through a new `ForceReleaseNegotiation` client method (Task 2), which is in the `client` package and CAN call `hasReleaseAck`. However, Task 1 step 5 explicitly says the tool handler will "Verify no `release-ack` exists (using existing `hasReleaseAck`)" -- this reads as the tool layer doing the check directly, which won't compile.

**Fix:** The plan text in Task 1 should clarify that steps 2-5 are all delegated to the client method (Task 2), and the tool handler only calls `c.ForceReleaseNegotiation(...)`. The current wording conflates tool-layer and client-layer responsibilities. If the implementer reads Task 1 literally and puts validation in `tools.go`, it will fail to compile.

#### F2: Client method cannot extract `holderID` from thread messages alone [Important]

**Location:** Plan Task 2, line 74

The plan says Task 2's client method:
> - Takes `threadID`, `file`, `reason`
> - Returns `(released int, holderID string, err error)`

To call `ReleaseByPattern(ctx, holderID, file)`, the method needs the holder's agent ID. The plan says this comes from the thread. Looking at how `release-request` messages are constructed in `negotiateRelease` (`tools.go:396-403`):

```go
bodyBytes, err := json.Marshal(map[string]any{
    "type":      "release-request",
    "file":      file,
    "reason":    reason,
    "requester": c.AgentName(),   // requester's name, NOT holder
    "urgency":   urgency,
    "thread_id": threadID,
})
```

The message is sent TO the holder (`holderID` at line 415), and the `from` field is the requester. Neither the message body nor envelope directly contains the holder's ID in a named field -- the holder is the **recipient** (`to` field). When fetching the thread via `FetchThread`, the `Message.To` field is a `[]string`, but the original request message's `To` would contain the holder.

This works but is fragile. The thread could contain multiple messages. The client method must:
1. Find the `release-request` message in the thread
2. Extract the `To` field (holder is the recipient of that message)

The plan does not specify this extraction logic. If the thread API does not return `To` fields on messages (some inbox APIs strip them), this will silently fail.

**Fix:** Either (a) require the caller to pass `holder_id` as a parameter to the tool (simplest, matches existing `negotiate_release` which already knows the holder), or (b) explicitly document how `holderID` is extracted from the thread and add a test for the extraction.

#### F3: TOCTOU race between pre-condition check and force-release [Important]

**Location:** Plan Task 1, steps 4-6; Plan Task 2

The plan validates pre-conditions (timeout exceeded, no ack) then releases:
```
Step 4: Verify timeout exceeded
Step 5: Verify no release-ack exists
Step 6: Call ReleaseByPattern
```

Between step 5 and step 6, the holder could:
- Send a `release-ack` (voluntary release)
- Call `respond_to_release` (which also calls `ReleaseByPattern`)

This creates two scenarios:
1. **Double ack:** Holder releases voluntarily (sends ack) while requester force-releases. Result: two `release-ack` messages in the thread. Not data corruption, but violates message invariants -- exactly the C1 race from the correctness review (`/home/mk/projects/Demarch/interverse/interlock/docs/research/correctness-review-interlock-negotiation-plan.md:26-56`).
2. **Phantom force-release:** Holder already released, `ReleaseByPattern` returns 0. Plan handles this (step 8: `already_released`). Good.

The advisory-only architecture makes this race window **much smaller** than the old auto-release approach (requires explicit human action vs. background goroutine), so it is lower severity. But the plan should acknowledge it.

**Mitigation:** The plan's step 8 (`already_released` when `released == 0`) handles the benign case. For the double-ack case, consider re-checking `hasReleaseAck` after `ReleaseByPattern` before sending the forced ack. This narrows the window but does not eliminate it. Alternatively, accept the race and document it -- duplicate acks are annoying but not harmful.

#### F4: Plan says "Notify the holder" but no mechanism specified [Important]

**Location:** Plan, On success step 3

The plan says:
> 3. Notify the holder agent that their reservation was force-released

There is no existing "notify holder" method or pattern. The existing `respond_to_release` sends a message to the **requester** (the thread peer), not back to the holder. To notify the holder, the tool would need to call `c.SendMessageFull(ctx, holderID, notificationBody, ...)` -- but:
- What thread does this notification go in? The negotiation thread (where the holder might be watching) or a new message?
- What is the message subject/type? There is no `force-release-notification` type in the protocol.
- The plan does not include this in the task line-count estimate (~60 lines for Task 1).

**Fix:** Either (a) define the notification message format and thread explicitly, or (b) remove step 3 from the plan. The forced `release-ack` with `forced: true` in step 2 already goes into the thread, which the holder can see. A separate notification may be redundant if the holder is watching the thread.

#### F5: `already_released` case still sends message -- potentially noisy [Suggestion]

**Location:** Plan Task 1, step 8

> 8. If `released == 0`: still send notification but mark as `already_released`

The correctness review (`correctness-review-interlock-negotiation-plan.md:45-56`) specifically flagged that sending acks when `released == 0` causes duplicate message spam (C1). The whole point of the advisory-only refactor was to eliminate this class of bug.

If `released == 0`, it means someone else already handled the release. Sending another message into the thread adds noise. Consider returning `already_released` to the caller without sending any message.

#### F6: No authorization check -- any agent can force-release [Suggestion]

**Location:** Plan Task 1, pre-conditions

The plan validates:
1. Thread exists with `release-request`
2. Timeout exceeded
3. No ack exists

Missing: validation that the **caller** is the original requester. Any agent who knows the `thread_id` could call `force_release_negotiation` to release another agent's reservations. In the advisory-only model, this is low risk (agents are cooperating), but the plan should either add a pre-condition check or explicitly document that authorization is out of scope.

**Fix for Task 4 tests:** Add a test case: "Agent C (not the requester) calls force_release_negotiation on a thread between A and B -- should this succeed or fail?"

#### F7: AGENTS.md tool count [Suggestion]

**Location:** Plan Task 5

The plan says to bump to "12 tools" in both plugin.json and AGENTS.md. The current AGENTS.md (`/home/mk/projects/Demarch/interverse/interlock/AGENTS.md`) says "11 tools" in the quick reference table. The CLAUDE.md also says "11 tools, 4 commands, 2 skills, 3 hooks". Both need updating. The plan only mentions plugin.json and AGENTS.md -- CLAUDE.md should also be listed.

---

### Improvements

1. **Add `holder_id` as a tool parameter.** This is the simplest fix for F2 and removes fragile thread-message parsing. The requester already knows the holder from the original `negotiate_release` call. The tool signature becomes:
   ```
   force_release_negotiation(thread_id, holder_id, file, reason)
   ```

2. **Restructure Task 1/Task 2 boundary.** Make it clear that Task 1's handler is a thin wrapper that calls Task 2's client method. All validation (steps 2-5) and release logic (steps 6-8) belong in the client method. The tool handler should only: extract args, call `c.ForceReleaseNegotiation(...)`, format result.

3. **Drop the separate holder notification (step 3).** The forced `release-ack` with `forced: true` already appears in the negotiation thread. Adding a separate notification message is undefined and adds complexity for marginal benefit. If holder notifications are desired, make them a separate follow-up task.

4. **Skip message send when `released == 0`.** Return `{"status": "already_released", "thread_id": ..., "released": 0}` without posting to the thread. This follows the correctness review's recommendation and avoids spam.

5. **Export `HasReleaseAck` from client package.** If validation logic will ever be needed in the tools layer (for future tools), export the function now. Otherwise, keep it unexported and ensure all callers go through client methods.

6. **Add CLAUDE.md to Task 5's file list.** Both CLAUDE.md and AGENTS.md reference the tool count and need updating.

---

### What Was Done Well

- The plan explicitly preserves advisory-only semantics and calls out the design choice clearly
- Task sequencing (2 -> 1 -> 3 -> 4 -> 5) correctly reflects dependencies
- The `can_escalate` / `escalation_hint` addition to timeout response (Task 3) is a clean UX pattern
- Test cases in Task 4 cover the right edge cases (expired, not-expired, already-acked, idempotent)
- Risk assessment is honest and accurate -- this genuinely does follow established patterns
- The plan correctly leverages existing primitives (`ReleaseByPattern`, `FetchThread`, `hasReleaseAck`) rather than building new ones
