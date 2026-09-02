# Interlock — Vision and Philosophy

**Version:** 0.1.0
**Last updated:** 2026-02-15

## What Interlock Is

Interlock is the multi-agent file coordination companion for Claude Code sessions that uses `intermute` as its coordination backend.
It is intentionally lightweight: a Go MCP server plus Bash hooks and scripts that make coordination practical without changing existing editing workflows.

Interlock exposes the coordination capabilities through MCP tools (`reserve_files`, conflict checks, messages, releases, and agent discovery) while delegating persistence and conflict resolution rules to `intermute`.

## Why This Exists

Multiple concurrent agents can edit the same repository without visibility into each other's intent. The result is noisy merge conflicts, silent overwrite risk, and avoidable rework.
Interlock exists to make that coordination explicit and cheap:

- reserve before editing,
- detect conflicts before work is blocked,
- communicate across agents,
- and enforce final safety at commit time.

## Relationship to `intermute`

`intermute` remains the source of truth for agent registry, reservations, reservation lifecycle, and messages.
Interlock is a companion layer:

- `interlock-mcp` is a protocol adapter from Claude Code tooling to `intermute` APIs,
- hooks and scripts translate session events into `intermute` calls (and are no-ops when unavailable),
- signals emitted by hooks are normalized for downstream status consumers like `interline`.

This keeps responsibilities clean:

- intermute: coordination state and APIs,
- interlock: agent UX and host integration.

## Advisory vs Mandatory Philosophy

Interlock enforces **advisory early warning** and **mandatory terminal enforcement**.

- Advisory phase (`PreToolUse:Edit`) warns about reserved files but does not block edits, so agents can pivot immediately.
- Mandatory phase (git pre-commit hook) blocks commits that include files reserved by other active agents.
- Session lifecycle hooks (SessionStart/Stop) and signal emission use graceful degradation for best-effort behavior when connectivity drops.

The design principle is: help agents make good decisions first, then guarantee no bad commit can sneak through. Every reservation, conflict check, and release produces a durable event in intermute — coordination failures are the highest-signal data for multi-agent quality.

## Scope and Limits

Interlock solves coordination, not policy. It does not auto-reserve files, route tasks, or replace human decision-making.
Its core mission is to make multi-agent work safer, faster, and inspectable with minimal workflow friction.

