# interlock — Development Guide

## Canonical References
1. `CLAUDE.md` — implementation details, architecture, testing, and release workflow.

MCP server for intermute-backed file reservation and agent coordination. Works standalone with any MCP client (see `docs/install.md`), and ships as a Claude Code plugin.

## Quick Reference

| Item | Value |
|------|-------|
| Namespace | `interlock:` |
| Manifest | `.claude-plugin/plugin.json` |
| Components | 20 tools, 4 commands, 2 skills, 3 hooks |
| Binary | `bin/interlock-mcp` |

## MCP Tools (20)

Reservations

| Tool | Purpose |
|------|---------|
| `reserve_files` | Reserve one or more file patterns before editing. |
| `release_files` | Release reservations by reservation ID. |
| `release_all` | Release all active reservations for the current agent. |
| `check_conflicts` | Dry-run conflict check for file patterns; returns collision cards. |
| `my_reservations` | List current active reservations for this agent. |

Negotiation

| Tool | Purpose |
|------|---------|
| `negotiate_release` | Ask a holder to release, with urgency and optional blocking wait; pins the reservation. |
| `respond_to_release` | Holder releases now or defers with an ETA (max 60 minutes). |
| `force_release_negotiation` | Requester escalates after the window elapses; releases only the pinned reservation. |
| `request_release` | Deprecated one-shot request without a thread; use `negotiate_release`. |
| `expire_window` | Soft-delete a window identity when an agent leaves for good. |

Messaging

| Tool | Purpose |
|------|---------|
| `send_message` | Direct message to another agent (optionally live into its tmux pane). |
| `broadcast_message` | Message every agent in the project. |
| `fetch_inbox` | Read this agent's inbox, cursor-paginated. |
| `fetch_stale_acks` | Ack-required messages nobody acknowledged in time. |
| `list_topic_messages` | Messages on a named topic. |

Agents and identity

| Tool | Purpose |
|------|---------|
| `list_agents` | Agents registered in the project, optionally by capability. |
| `list_window_identities` | Persistent window UUID to agent mappings. |
| `rename_window` | Set the display name on a window identity. |

Contact policy

| Tool | Purpose |
|------|---------|
| `get_contact_policy` | Read this agent's contact policy. |
| `set_contact_policy` | Set it: open, auto, contacts_only, or block_all. |

## Negotiation Protocol

- `negotiate_release` sends a `release-request` message with `urgency` (`normal` or `urgent`) and a generated `thread_id` for tracking.
- `wait_seconds` on `negotiate_release` enables blocking-wait mode: the tool polls the negotiation thread and returns `release`, `defer`, or `timeout` status. On timeout, the response includes `can_escalate: true` with a hint to use `force_release_negotiation`.
- `respond_to_release` handles both actions:
  - `action='release'` releases matching reservations and sends `release-ack`.
  - `action='defer'` keeps reservation, includes `eta_minutes`/`reason`, and sends `release-defer`.
- `force_release_negotiation` is the escalation path: when `negotiate_release` times out, the requester can explicitly force-release the holder's reservation. Validates pre-conditions (timeout exceeded, no existing ack) before releasing. Sends `release-ack` with `forced: true` to the thread and notifies the holder.
- `INTERLOCK_AUTO_RELEASE=1` enables advisory mode in `hooks/pre-edit.sh`: pending release requests are surfaced as context with suggested `respond_to_release(...)` calls.
- Timeout escalation uses advisory-only enforcement: `CheckExpiredNegotiations` (called from `fetch_inbox`) identifies expired negotiations and returns advisory information — it does NOT force-release reservations. Holder agents see timeout context on their next edit via `pre-edit.sh` (when `INTERLOCK_AUTO_RELEASE=1`). Thresholds: `urgent` at 5 minutes, `normal` at 10 minutes. Constants exported from `internal/client`: `NormalTimeoutMinutes`, `UrgentTimeoutMinutes`, `NegotiationPollInterval`.
- `/interlock:status` includes a pending negotiations table showing requester, holder, file, urgency, age, and current status.

