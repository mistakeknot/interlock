# Interlock

> See `AGENTS.md` for full development guide.

## Overview

MCP server wrapping intermute's HTTP API for file reservation and agent coordination. 20 tools, 4 commands, 2 skills, 3 hooks.

## Negotiation Protocol

- `negotiate_release` — request file release with urgency + optional blocking wait
- `respond_to_release` — acknowledge (release) or defer with ETA
- `INTERLOCK_AUTO_RELEASE=1` — enable advisory release-request notifications in pre-edit hook

## Quick Commands

```bash
# Build binary
bash scripts/build.sh

# Run Go tests
go test ./...

# Validate structure
python3 -c "import json; json.load(open('.claude-plugin/plugin.json'))"
```

## Design Decisions (Do Not Re-Ask)

- Go binary for MCP server (mark3labs/mcp-go), bash for hooks
- Unix socket preferred, TCP fallback for intermute connection
- PreToolUse:Edit hook blocks edits to files another agent holds exclusively (warning-only when the optional region check clears it or intermute is down); git pre-commit hook is the backstop
- Join-flag gating: all hooks check ~/.config/interlock/joined (older installs: the previous config directory, still honored)
