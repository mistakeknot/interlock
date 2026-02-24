# interlock

Multi-agent file coordination for Claude Code.

## What This Does

When two agents try to edit the same file simultaneously, you get a mess. interlock prevents this by wrapping the intermute coordination service with an MCP server that provides file reservation, conflict detection, and inter-agent messaging.

The workflow: before editing a file, an agent reserves it. If another agent already holds the reservation, interlock offers a negotiated release protocol — the requesting agent sends a `negotiate_release` with urgency and optional blocking wait; the holding agent responds with either a release or a deferral with ETA. This is cooperative, not preemptive.

A git pre-commit hook provides mandatory enforcement — if you try to commit a file that's reserved by another agent, the commit is blocked. The PreToolUse:Edit hook is advisory (warns but doesn't block) so agents can still make emergency edits.

## Installation

First, add the [interagency marketplace](https://github.com/mistakeknot/interagency-marketplace) (one-time setup):

```bash
/plugin marketplace add mistakeknot/interagency-marketplace
```

Then install the plugin:

```bash
/plugin install interlock
```

Requires the intermute service running (the Go coordination backend).

## Usage

Join coordination:

```
/interlock:join
```

Check who's working on what:

```
/interlock:status
```

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

11 MCP tools cover the full reservation lifecycle. Connects to intermute via Unix socket (preferred) or TCP fallback.

## Design Decisions

- Go binary for MCP server, bash for hooks
- Join-flag gating: all hooks check `~/.config/clavain/intermute-joined` before running
- `INTERLOCK_AUTO_RELEASE=1` enables advisory release-request notifications in the pre-edit hook
