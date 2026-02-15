---
name: coordination-protocol
description: Use when multiple agents are editing the same repository — guides the reserve-work-release workflow to prevent file conflicts and lost work
---

## Overview

Interlock provides file-level coordination for multi-agent sessions. Core principle: reserve before editing, release when done. This prevents silent overwrites and merge conflicts between concurrent agents.

## Prerequisites

- Agent must be registered via `/interlock:join` before reserving files
- intermute service must be running (verify with `/interlock:status`)

## The Workflow

### 1. RESERVE: Before editing, call `reserve_files` with file patterns
- Specify a reason (what you're doing)
- Set appropriate TTL (default: 15 minutes)
- If conflict returned, switch to conflict-recovery skill

### 2. WORK: Edit reserved files normally
- Call `my_reservations` to verify what you hold
- Extend reservation if work takes longer than expected

### 3. RELEASE: After done, call `release_files` or `release_all`
- Release as soon as edits are committed
- Stop hook auto-releases on session end (safety net, not primary)

## MCP Tools Quick Reference

| Tool | Purpose |
|------|---------|
| `reserve_files` | Reserve files by glob pattern before editing |
| `release_files` | Release specific file reservations |
| `release_all` | Release all your reservations at once |
| `check_conflicts` | Check if files are reserved by another agent |
| `my_reservations` | List your current reservations |
| `list_agents` | Show all active agents in the project |
| `send_message` | Send a message to another agent |
| `fetch_inbox` | Check messages from other agents |
| `request_release` | Ask another agent to release their reservation |

## Best Practices

- **Reserve narrowly** — reserve specific files, not entire directories
- **Short TTLs** — 15 minutes default; extend only if needed
- **Release early** — release as soon as you commit, don't hold until session end
- **Check before reserving** — call `check_conflicts` before `reserve_files` to avoid surprise 409s
- **Include a reason** — helps other agents understand why files are held
- **One concern per reservation** — separate reservations for separate tasks

## Common Mistakes

- Reserving too broadly (e.g., `src/**`) — reserve only files you will edit
- Forgetting to release after committing — always `release_files` or `release_all` after `git commit`
- Ignoring conflict responses — if `reserve_files` returns a conflict, do NOT edit the file; the git pre-commit hook will block the commit
- Not joining first — `reserve_files` fails if agent is not registered; run `/interlock:join` first
