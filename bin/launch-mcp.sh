#!/usr/bin/env bash
# Launcher for interlock-mcp: probes known binary paths before falling back to go build.
# Probe order: cache-local → ~/.local/bin → go build.
# Sidesteps envs where `go` is missing from the MCP subprocess PATH.
# The cache-local binary is rebuilt when any Go source is newer than it (a stale
# binary once served February code against September sources); if go is missing
# the stale binary still runs, with a warning on stderr (stdout is the MCP channel).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BINARY="${SCRIPT_DIR}/interlock-mcp"

if [[ -x "$BINARY" ]] && command -v go &>/dev/null; then
    stale_src="$(find "$PROJECT_ROOT/cmd" "$PROJECT_ROOT/internal" "$PROJECT_ROOT/go.mod" "$PROJECT_ROOT/go.sum" \
        \( -name '*.go' -o -name go.mod -o -name go.sum \) -newer "$BINARY" -print -quit 2>/dev/null || true)"
    if [[ -n "$stale_src" ]]; then
        echo "interlock-mcp is older than $stale_src; rebuilding" >&2
        tmp_bin="${BINARY}.tmp.$$"
        if (cd "$PROJECT_ROOT" && go build -o "$tmp_bin" ./cmd/interlock-mcp/ >&2); then
            mv -f "$tmp_bin" "$BINARY"
        else
            rm -f "$tmp_bin"
            echo "interlock-mcp rebuild failed; running the stale binary" >&2
        fi
    fi
elif [[ -x "$BINARY" ]]; then
    if [[ -n "$(find "$PROJECT_ROOT/cmd" "$PROJECT_ROOT/internal" -name '*.go' -newer "$BINARY" -print -quit 2>/dev/null || true)" ]]; then
        echo "interlock-mcp binary is older than its sources and go is unavailable; running it anyway" >&2
    fi
fi

for candidate in \
    "$BINARY" \
    "${HOME}/.local/bin/interlock-mcp"
do
    if [[ -x "$candidate" ]]; then
        exec "$candidate" "$@"
    fi
done

# Fallthrough: attempt build if toolchain available
if ! command -v go &>/dev/null; then
    echo '{"error":"go not found — cannot build interlock-mcp. Install Go 1.23+ and restart."}' >&2
    exit 1
fi
cd "$PROJECT_ROOT"
go build -o "$BINARY" ./cmd/interlock-mcp/ 2>&1 >&2
exec "$BINARY" "$@"
