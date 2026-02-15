---
name: conflict-recovery
description: Use when a file edit is blocked because another agent holds a reservation — guides recovery from check status through escalation
---

## Overview

When you try to reserve or edit a file held by another agent, you hit a conflict. The PreToolUse:Edit hook warns you; the git pre-commit hook blocks commits. This skill teaches the escalation ladder for resolving conflicts without losing work.

## When This Applies

- `reserve_files` returns a conflict (409)
- PreToolUse:Edit hook warns that a file is reserved by another agent
- `git commit` is rejected by the pre-commit hook due to reserved files
- You discover via `/interlock:status` that files you need are held

## Recovery Ladder

### Step 1: Check Status
Call `check_conflicts` or run `/interlock:status` to see:
- Who holds the reservation (agent name + ID)
- Why (the reason string)
- When it expires (expires_at timestamp)

### Step 2: Work Elsewhere
If other unreserved files need attention, work on those first. The reservation may expire or be released while you work. Call `my_reservations` to see what you already hold.

### Step 3: Request Release
Call `negotiate_release` with the holding agent's name/ID, file pattern, and reason. Use `urgency='urgent'` only when truly blocking critical work. For blocking-wait mode, set `wait_seconds` (for example `wait_seconds=120`) so the call polls the negotiation thread before returning.

### Step 4: Wait for Expiry
Check the `expires_at` timestamp from Step 1. If expiry is <5 minutes away, wait it out. Stale reservations are auto-cleaned by intermute every 60 seconds.

### Step 5: Escalate to User
If the reservation holder is unresponsive and the work is urgent:
- Report the conflict to the user with agent name, file, and reason
- User can manually run `/interlock:status` and decide
- User can force-release via `git commit --no-verify` (last resort)

## Key MCP Tools for Recovery

| Tool | When to Use |
|------|-------------|
| `check_conflicts` | Step 1: see who holds the file |
| `list_agents` | Identify the holding agent |
| `negotiate_release` | Step 3: ask them to release (optionally wait with `wait_seconds`) |
| `fetch_inbox` | Check if they responded |

## Common Mistakes

- Immediately requesting release without checking expiry — if it expires in 2 minutes, just wait
- Editing the file anyway despite the warning — the git pre-commit hook will block your commit
- Using `git commit --no-verify` without user approval — bypasses safety, risks overwriting another agent's work
- Not checking your own reservations — you might hold files the other agent needs; consider a mutual release
