# interlock — Development Guide

MCP server for intermute-backed file reservation and agent coordination. Companion plugin for Clavain.

## Quick Reference

| Item | Value |
|------|-------|
| Namespace | `interlock:` |
| Manifest | `.claude-plugin/plugin.json` |
| Components | 11 tools, 4 commands, 2 skills, 3 hooks |
| Binary | `bin/interlock-mcp` |

## MCP Tools (11 total)

| Tool | Purpose |
|------|---------|
| `reserve_files` | Reserve one or more file patterns before editing. |
| `release_files` | Release reservations by reservation ID. |
| `release_all` | Release all active reservations for the current agent. |
| `check_conflicts` | Dry-run conflict check for file patterns. |
| `my_reservations` | List current active reservations for this agent. |
| `send_message` | Send a direct message to another agent. |
| `fetch_inbox` | Fetch inbox messages and run negotiation-timeout checks. |
| `list_agents` | List active agents in the current project. |
| `request_release` | Legacy release request tool (deprecated; use negotiation tools). |
| `negotiate_release` | Start a release negotiation with urgency + optional blocking wait. |
| `respond_to_release` | Resolve negotiation by releasing now or deferring with ETA. |

## Negotiation Protocol

- `negotiate_release` sends a `release-request` message with `urgency` (`normal` or `urgent`) and a generated `thread_id` for tracking.
- `wait_seconds` on `negotiate_release` enables blocking-wait mode: the tool polls the negotiation thread and returns `release`, `defer`, or `timeout` status.
- `respond_to_release` handles both actions:
  - `action='release'` releases matching reservations and sends `release-ack`.
  - `action='defer'` keeps reservation, includes `eta_minutes`/`reason`, and sends `release-defer`.
- `INTERLOCK_AUTO_RELEASE=1` enables advisory mode in `hooks/pre-edit.sh`: pending release requests are surfaced as context with suggested `respond_to_release(...)` calls.
- Timeout escalation is enforced at protocol level: `urgent` requests escalate at 5 minutes, `normal` at 10 minutes, with background + inbox-driven enforcement via `CheckExpiredNegotiations`.
- `/interlock:status` includes a pending negotiations table showing requester, holder, file, urgency, age, and current status.

