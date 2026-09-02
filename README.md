# interlock

Multi-agent file coordination for Claude Code.

## What this does

When two agents try to edit the same file simultaneously, you get a mess. interlock prevents this by wrapping the intermute coordination service with an MCP server that provides file reservation, conflict detection, and inter-agent messaging.

The workflow: before editing a file, an agent reserves it. If another agent already holds the reservation, interlock offers a negotiated release protocol: the requesting agent sends a `negotiate_release` with urgency and optional blocking wait; the holding agent responds with either a release or a deferral with ETA. This is cooperative, not preemptive.

The pre-edit hook (`PreToolUse:Edit`) blocks an edit to a file another agent holds exclusively. It downgrades to a warning when a tier-2 region check finds no overlap between the two edits, or when intermute is unreachable — coordination fails open, not closed. The git pre-commit hook is the backstop: it blocks a commit that touches a file still reserved by another agent, so a bypassed or missed pre-edit warning can't sneak a conflicting change through.

## Installation

First, add the [interagency marketplace](https://github.com/mistakeknot/interagency-marketplace) (one-time setup):

```bash
/plugin marketplace add mistakeknot/interagency-marketplace
```

Then install the plugin:

```bash
/plugin install interlock
```

Requires intermute; see [docs/install.md](docs/install.md) for the standalone path (any MCP client, no Claude Code plugin) — `go install github.com/mistakeknot/intermute/cmd/intermute@latest && intermute serve`.

## Usage

Join coordination:

```
/interlock:join
```

Check who's working on what:

```
/interlock:status
```

Reserve files with bead correlation:

```
reserve_files(
  patterns=["internal/client/*"],
  reason="Wave 3 collision-card edit",
  active_bead_id="sylveste-kgfi.3"
)
```

Check conflicts with bead-aware collision cards:

```
check_conflicts(
  patterns=["internal/client/*"],
  active_bead_id="sylveste-kgfi.3"
)
```

The v0 bead metadata convention and collision-card response shape are documented in
[`docs/bead-aware-reservations.md`](docs/bead-aware-reservations.md).

Leave and release all reservations:

```
/interlock:leave
```

## Architecture

```
bin/launch-mcp.sh        MCP server launcher (Go binary, mark3labs/mcp-go)
skills/                  coordination-protocol, conflict-recovery
commands/                join, leave, status, setup
hooks/                   PreToolUse (advisory), PostToolUse, git pre-commit
```

20 MCP tools cover the full reservation lifecycle (see [Tools](#tools) below). Connects to intermute via Unix socket (preferred) or TCP fallback.

## Tools

Reservations:

- `reserve_files` — reserve file patterns before editing; blocks other agents from touching them
- `release_files` — release specific reservations by ID
- `release_all` — release all your active reservations at once
- `my_reservations` — list your current active reservations
- `check_conflicts` — dry-run conflict check for file patterns (creates no reservation)

Negotiation:

- `negotiate_release` — ask another agent to release a file, with urgency and optional blocking wait
- `respond_to_release` — resolve a negotiation: release now, or defer with an ETA
- `force_release_negotiation` — force-release a reservation after a negotiation has timed out
- `request_release` (deprecated) — legacy release-request tool; use `negotiate_release`
- `expire_window` — soft-delete a window identity by setting its expiration to now

Messaging:

- `send_message` — send a message to another agent
- `broadcast_message` — send a message to every agent in the project
- `fetch_inbox` — check your inbox for messages from other agents
- `fetch_stale_acks` — find messages needing acknowledgment that missed their TTL
- `list_topic_messages` — list messages by topic, for late-joining or oversight agents

Agents and identity:

- `list_agents` — list agents registered with intermute, optionally filtered by capability
- `list_window_identities` — list active window identities (tmux window UUID to persistent agent ID) for this project
- `rename_window` — update the display name for a window identity

Contact policy:

- `get_contact_policy` — get your agent's current contact policy
- `set_contact_policy` — set who can message you: open, auto, contacts_only, or block_all

## Design decisions

- Go binary for MCP server, bash for hooks
- Join-flag gating: all hooks check `~/.config/interlock/joined` before running
- `INTERLOCK_AUTO_RELEASE=1` enables advisory release-request notifications in the pre-edit hook
- Negotiation protocol (reservation-ID pinning, participant checks): [docs/negotiation-protocol.md](docs/negotiation-protocol.md)
