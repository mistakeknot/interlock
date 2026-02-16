# Documentation Drift Scan: interlock

**Date:** 2026-02-15
**Scope:** All documentation files in `/root/projects/Interverse/plugins/interlock/`
**Trigger:** Recent changes to advisory-only enforcement, background goroutine removal, exported constants, ReleaseByPattern 404 idempotency, and test count increase (95 -> 107).

---

## Methodology

1. Checked for `config/watchables.yaml` -- does not exist; no watchables config found anywhere in the plugin.
2. Read AGENTS.md, CLAUDE.md, and all docs under `docs/` (vision.md, roadmap.md, PRD.md, research/).
3. Compared documentation claims against current codebase state using grep for key terms.
4. Verified test count via `pytest --collect-only` (107 items collected).

---

## Findings Table

| Document | Status | Finding |
|----------|--------|---------|
| `AGENTS.md` | **STALE** | Line 38 says "background + inbox-driven enforcement via `CheckExpiredNegotiations`". The background goroutine has been removed. Enforcement is now inbox-driven only (advisory via `fetch_inbox`). Should read "inbox-driven advisory enforcement" with no mention of background. |
| `CLAUDE.md` | **OK** | Mentions "advisory release-request notifications" on line 13 which is accurate. Does not reference background goroutine or test counts. No drift detected. |
| `docs/roadmap.md` | **STALE** | Line 14 says "9 tools" -- actual count is 11 (added `negotiate_release` and `respond_to_release`). Line 25 says "95 structural tests passing" -- actual count is 107. |
| `docs/vision.md` | **STALE** | Line 43 says `PreToolUse:Edit` "warns about reserved files but does not block edits" -- this is wrong. The edit hook now uses `decision:block` for exclusive conflicts. Line 51 says "It does not auto-reserve files" -- this is wrong. The edit hook now auto-reserves files on first edit (15min TTL). These are pre-Phase-2 claims that were never updated. |
| `docs/PRD.md` | **STALE** | Section 4 says "9 tools" (line 31). Section 5 MCP tools table (line 43) lists only the original 9 tools, missing `negotiate_release` and `respond_to_release`. Tool count should be 11. The rest of the PRD (hooks, workflows, architecture) is accurate and up to date. |
| `.claude-plugin/plugin.json` | **STALE** | Description field says "9 tools" -- actual count is 11. |
| `docs/research/design-phase-3-implementation.md` | **STALE** | Lines 16 and 46 reference "95 structural tests" -- actual count is 107. This is a design doc (less critical) but the baseline assumption is wrong. |
| `docs/research/quality-review-of-implementation.md` | **OK** | Pre-advisory-change review document. Describes the old behavior accurately for its time. Historical artifact, not actionable. |
| `docs/research/correctness-review-of-implementation.md` | **OK** | Same as above -- historical review document describing the old implementation. |
| `docs/research/quality-review-of-diff.md` | **OK** | Documents the advisory-only transition diff. Accurate for what it describes. |
| `docs/research/correctness-review-of-diff.md` | **OK** | Documents the advisory-only transition diff. Accurate for what it describes. Correctly notes background goroutine removal. |
| Interverse `MEMORY.md` | **STALE** | Line 69 says "95 structural tests in interlock cover all coordination features" -- actual count is 107. |

---

## Detail: AGENTS.md Drift

**Current text (line 38):**
```
Timeout escalation is enforced at protocol level: `urgent` requests escalate at 5 minutes,
`normal` at 10 minutes, with background + inbox-driven enforcement via `CheckExpiredNegotiations`.
```

**What changed:**
- The background goroutine (a singleton started on first `negotiate_release` call that ran `CheckExpiredNegotiations` every 2 seconds) has been completely removed.
- `CheckExpiredNegotiations` is now called only from the `fetch_inbox` tool handler.
- `CheckExpiredNegotiations` is now advisory-only: it returns `NegotiationTimeout` structs but does NOT call `ReleaseByPattern` or send `release-ack` messages. The holder agent sees advisory context on next edit (via `pre-edit.sh` when `INTERLOCK_AUTO_RELEASE=1`).
- Exported constants `NormalTimeoutMinutes = 10`, `UrgentTimeoutMinutes = 5`, `NegotiationPollInterval = 2 * time.Second` are now in `internal/client/client.go`.

**Suggested fix:**
```
Timeout escalation uses advisory-only enforcement: `CheckExpiredNegotiations` (called from `fetch_inbox`)
identifies expired negotiations and returns advisory information. Holder agents see timeout context on
their next edit via `pre-edit.sh` (when `INTERLOCK_AUTO_RELEASE=1`). Constants: `NormalTimeoutMinutes=10`,
`UrgentTimeoutMinutes=5`, `NegotiationPollInterval=2s` (exported from `internal/client`).
```

---

## Detail: vision.md Drift

**Current text (lines 41-47):**
```markdown
Interlock enforces **advisory early warning** and **mandatory terminal enforcement**.

- Advisory phase (`PreToolUse:Edit`) warns about reserved files but does not block edits,
  so agents can pivot immediately.
- Mandatory phase (git pre-commit hook) blocks commits that include files reserved by
  other active agents.
```

**What changed:**
- `PreToolUse:Edit` now issues `decision:block` for files exclusively reserved by another agent. It is no longer advisory-only at the edit phase.
- The hook also auto-reserves files on first edit (15min TTL, auto-renewing).

The entire "Advisory vs Mandatory Philosophy" section needs rewriting. The current design is:
- **Blocking edit enforcement** for exclusive reservation conflicts (`decision:block`).
- **Advisory negotiation context** for pending release requests (via `additionalContext` when `INTERLOCK_AUTO_RELEASE=1`).
- **Mandatory commit gate** via git pre-commit hook.

**Current text (line 51):**
```
It does not auto-reserve files, route tasks, or replace human decision-making.
```

**What changed:**
- Interlock now auto-reserves files on first edit. This statement is factually wrong.

---

## Detail: Tool Count Drift (9 vs 11)

Multiple documents reference "9 tools" but the actual count is 11. The two tools added since the original count:
1. `negotiate_release` -- start a release negotiation with urgency + optional blocking wait
2. `respond_to_release` -- resolve negotiation by releasing now or deferring with ETA

**Affected documents:**
- `docs/roadmap.md` line 14: "9 tools"
- `docs/PRD.md` line 31: "9 tools"
- `docs/PRD.md` line 43: MCP tools table lists only 9 tools
- `.claude-plugin/plugin.json` line 4: description says "9 tools"

**Not affected (already correct):**
- `AGENTS.md` line 11: "11 tools" (correct)
- `CLAUDE.md` line 7: "11 tools" (correct)
- `tests/structural/test_structure.py` line 62: asserts 11 tools (correct)

---

## Detail: Test Count Drift (95 vs 107)

`pytest --collect-only` collects 107 test items. Several documents still reference 95:

| Document | Line | Text |
|----------|------|------|
| `docs/roadmap.md` | 25 | "95 structural tests passing" |
| `docs/research/design-phase-3-implementation.md` | 16, 46 | "95 structural tests" |
| Interverse `MEMORY.md` | 69 | "95 structural tests in interlock" |

---

## Priority Summary

| Priority | Document | Action Needed |
|----------|----------|---------------|
| **HIGH** | `AGENTS.md` | Remove "background" from timeout enforcement description; clarify advisory-only semantics |
| **HIGH** | `docs/vision.md` | Rewrite "Advisory vs Mandatory" section; remove "does not auto-reserve" claim |
| **MEDIUM** | `docs/PRD.md` | Update tool count to 11; add `negotiate_release` and `respond_to_release` to tools table |
| **MEDIUM** | `docs/roadmap.md` | Update tool count (9->11) and test count (95->107) |
| **MEDIUM** | `.claude-plugin/plugin.json` | Update description to say "11 tools" and mention negotiation |
| **LOW** | Interverse `MEMORY.md` | Update test count (95->107) |
| **LOW** | `docs/research/design-phase-3-implementation.md` | Update test count baseline (95->107); design doc so less critical |

---

## No Watchables Config Found

There is no `config/watchables.yaml` or any watchables configuration file in the interlock plugin directory. If interwatch is intended to monitor these docs, a watchables config should be created.
