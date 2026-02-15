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
- User-facing command surface: `/interlock:join`, `/interlock:leave`, `/interlock:status`, `/interlock:setup`.
- Recovery guidance documented in two skills (`coordination-protocol`, `conflict-recovery`).
- Signals emitted for reserve/release/message events for status integrations.
- Graceful failure mode: if `intermute` is unavailable, hooks fail open and proceed safely.
- 95 structural tests passing.

## What's Next

### Phase 3: Workflow Integration (Clavain)
- Auto-join on SessionStart (eliminate manual `/interlock:join` requirement).
- Sprint pre-flight: show active agents, their reservations, and dirty tree status.
- Bead-agent binding: prevent two sessions from claiming the same issue.
- Post-commit rebase notification: trigger `git pull --rebase` in other sessions after a commit.

### Phase 4: UX Polish
- Interline statusline integration (live agent/reservation awareness).
- Conflict resolution automation (auto-merge → message → escalate).
- Agent Teams bridge evaluation.

### Operational Improvements
- Coordination telemetry (conflict rates, resolution latency).
- Expanded `/interlock:status` with per-pattern hold reasons and human-readable holder names.
- Structured onboarding/health diagnostics for misconfigured environments.

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

