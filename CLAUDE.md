# Interlock

> See `AGENTS.md` for full development guide.

## Overview

MCP server wrapping intermute's HTTP API for file reservation and agent coordination. 12 tools, 4 commands, 2 skills, 3 hooks. Companion plugin for Clavain.

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
- Advisory-only PreToolUse:Edit hook, mandatory git pre-commit enforcement
- Join-flag gating: all hooks check ~/.config/clavain/intermute-joined
