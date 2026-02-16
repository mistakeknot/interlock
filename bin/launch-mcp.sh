#!/usr/bin/env bash
# Launcher for interlock-mcp: auto-builds if binary is missing.
# Used as the MCP command in plugin.json so `claude plugins install`
# works without a postInstall hook.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="${SCRIPT_DIR}/interlock-mcp"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

if [[ ! -x "$BINARY" ]]; then
    # Check Go is available
    if ! command -v go &>/dev/null; then
        echo '{"error":"go not found — cannot build interlock-mcp. Install Go 1.23+ and restart."}' >&2
        exit 1
    fi
    # Auto-build on first run
    cd "$PROJECT_ROOT"
    go build -o "$BINARY" ./cmd/interlock-mcp/ 2>&1 >&2
fi

exec "$BINARY" "$@"
