# Interlock Roadmap

**Version:** 0.1.0
**Last updated:** 2026-02-15
**Vision:** [`docs/vision.md`](vision.md)
**PRD:** [`docs/PRD.md`](PRD.md)

## Where We Are

Interlock is a shipped Clavain companion with a complete operational baseline: MCP coordination tools, session lifecycle hooks, and mandatory commit-time safety checks.

## What's Working

- MCP server is shipped and wired to `intermute` with 9 tools for reservation, messaging, and discovery.
- File coordination hooks are installed: `SessionStart` auto-registration, advisory `PreToolUse:Edit`, and `Stop` cleanup.
- Git pre-commit enforcement is installed via `interlock-install-hooks` and blocks commits when staged files conflict with active reservations.
- User-facing command surface is available: `/interlock:join`, `/interlock:leave`, `/interlock:status`, `/interlock:setup`.
- Advisory recovery guidance is documented in two skills (`coordination-protocol`, `conflict-recovery`).
- Signals are emitted for reserve/release/message events for status integrations.
- Graceful failure mode is implemented: if `intermute` is unavailable, hooks and shell commands fail open where appropriate and proceed safely.

## What's Next

- Improve coordination telemetry (signal quality, conflict rates, and conflict resolution latency) for operational feedback.
- Improve failure visibility when joining or checking reservations fails intermittently (without changing the no-blocking edit philosophy).
- Expand `/interlock:status` and signal-driven indicators with clearer per-pattern hold reasons and human-readable holder names.
- Add more structured onboarding/health diagnostics for environments where `intermute` is present but misconfigured.
- Continue extraction and hardening with interwatch/interline integrations as those surfaces evolve.

## Current Baseline

All future work should preserve the core principles:

1. Socket-first, TCP-safe transport design.
2. Advisory edit warnings + mandatory commit gate.
3. No silent policy decisions; coordination failures are explicit.

## Exit Criteria for Next Bump

- Conflict detection is observable from session start to pre-commit gate.
- New diagnostics and status improvements are documented in PRD and user-facing command docs.
- No changes to the 12-feature coordination contract that reduce fallback safety.

