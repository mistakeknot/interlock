# Interlock Roadmap

**Version:** 0.1.1
**Last updated:** 2026-02-15
**Vision:** [`docs/vision.md`](vision.md)
**PRD:** [`docs/PRD.md`](PRD.md)

## Where We Are

Interlock has shipped Phase 1+2 of multi-session coordination: per-session git index isolation, commit serialization, blocking edit enforcement, and automatic file reservation. The system now provides a complete safety layer from first edit through commit.

## What's Working

- MCP server is shipped and wired to `intermute` with 9 tools for reservation, messaging, and discovery.
- **Blocking `PreToolUse:Edit` hook** prevents edits to files exclusively reserved by another session (`decision:block`).
- **Auto-reserve on first edit** — any file edited is automatically reserved (15min TTL, auto-renewing on subsequent edits).
- **Per-session git index** — `SessionStart` sets `GIT_INDEX_FILE=.git/index-$SESSION_ID` so each session stages independently.
- **Commit serialization** — `mkdir`-based lock ensures only one session commits at a time.
- **Post-commit hook** — refreshes session index via `git read-tree HEAD`, auto-releases reservations for committed files, broadcasts commit event via Intermute.
- Git pre-commit enforcement blocks commits when staged files conflict with active reservations held by other agents.
- User-facing command surface: `/interlock:join`, `/interlock:leave`, `/interlock:interlock-status`, `/interlock:interlock-setup`.
- Recovery guidance documented in two skills (`coordination-protocol`, `conflict-recovery`).
- Signals emitted for reserve/release/message events for status integrations.
- Graceful failure mode: if `intermute` is unavailable, hooks fail open and proceed safely.
- 95 structural tests passing.

## What's Next

### P2.1 — Workflow Integration (Clavain)
- [ILK-N1] **Auto-join on startup** — enable session onboarding without manual `/interlock:join`.
- [ILK-N2] **Sprint visibility pre-flight** — surface active agents, reservations, and dirty tree state before work.
- [ILK-N3] **Bead-agent binding** — prevent two sessions from claiming the same issue.
- [ILK-N4] **Post-commit broadcast** — trigger session rebase guidance in active sessions after commit.

### P2.3 — UX and Recovery
- [ILK-P1] **Interline reservation visibility** — show live agent and hold state in statusline.
- [ILK-P2] **Automated conflict resolution** — add auto-merge → message → escalation path.
- [ILK-P3] **Agent Teams bridge** — evaluate external team handoff compatibility.

### Operational Improvements
- [ILK-N5] **Coordination telemetry** — export conflict rates and resolution latency for diagnostics.
- [ILK-N6] **Status reason transparency** — include per-pattern hold reasons and holder names.
- [ILK-N7] **Onboarding diagnostics** — run deterministic health checks for misconfigured environments.

## Current Baseline

All future work should preserve the core principles:

1. Socket-first, TCP-safe transport design.
2. Blocking edit enforcement for exclusive reservations + mandatory commit gate.
3. Per-session git index isolation for safe concurrent staging.
4. No silent policy decisions; coordination failures are explicit.

## Exit Criteria for Next Bump

- Phase 3 workflow integration: sessions auto-join and sprint shows coordination state.
- Bead-agent binding prevents duplicate work claims.
- No changes to the coordination contract that reduce fallback safety.

## From Interverse Roadmap

Items from the [Interverse roadmap](../../../docs/roadmap.json) that involve this module:

- **iv-1aug** [Next] F1: Release Response Protocol — `release_ack` / `release_defer` (Phase 4a prerequisite is complete)
- **iv-5ijt** [Next] F3: Structured `negotiate_release` MCP tool (blocked by iv-1aug)
- **iv-6u3s** [Next] F4: Sprint Scan release visibility (blocked by iv-1aug)
- **iv-2jtj** [Next] F5: Escalation timeout for unresponsive agents (blocked by iv-5ijt)
