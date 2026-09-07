#!/usr/bin/env bash
cd "$(dirname "$0")"
exec codex exec --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox -c 'mcp_servers.interlock.command="<checkout>/interverse/interlock/bin/interlock-mcp"' -c 'mcp_servers.interlock.env={INTERMUTE_URL="http://127.0.0.1:7339",INTERLOCK_AGENT_NAME="codex-demo",INTERLOCK_PROJECT="demo"}' "$(cat prompt-codex.txt)"
