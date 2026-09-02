#!/usr/bin/env bash
# Two agents, one file, no collision — runnable from a clean clone.
#
# Starts a private intermute, two interlock-mcp agents, and an intermux
# watching a private tmux server, then walks the reservation / negotiation /
# release loop and asks intermux what it can see. Nothing touches your real
# tmux server or your real intermute.
#
# Requirements: go 1.24+, tmux, python3. Optional: DEMO_LOCAL=1 builds from
# sibling checkouts (../../../intermute etc.) instead of `go install @latest`.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/interlock-demo.XXXXXX")"
BIN="$WORK/bin"; mkdir -p "$BIN" "$WORK/proj/src"
export GOBIN="$BIN"
PORT="${DEMO_PORT:-$(( 20000 + RANDOM % 20000 ))}"
TMUX_NAME="ilkdemo-$$"

cleanup() {
  tmux -L "$TMUX_NAME" kill-server 2>/dev/null || true
  [[ -n "${INTERMUTE_PID:-}" ]] && kill "$INTERMUTE_PID" 2>/dev/null || true
  rm -rf "$WORK"
}
trap cleanup EXIT

say() { printf '\n\033[1m%s\033[0m\n' "$*"; }

say "Building intermute, interlock-mcp, intermux-mcp into $BIN"
if [[ "${DEMO_LOCAL:-}" == "1" ]]; then
  ROOT="$(cd "$HERE/../.." && pwd)"                      # interlock checkout
  (cd "$ROOT" && go build -o "$BIN/interlock-mcp" ./cmd/interlock-mcp)
  (cd "$ROOT/../../core/intermute" && go build -o "$BIN/intermute" ./cmd/intermute)
  (cd "$ROOT/../intermux" && go build -o "$BIN/intermux-mcp" ./cmd/intermux-mcp)
else
  go install github.com/mistakeknot/intermute/cmd/intermute@latest
  go install github.com/mistakeknot/interlock/cmd/interlock-mcp@latest
  go install github.com/mistakeknot/intermux/cmd/intermux-mcp@latest
fi

say "Starting intermute on 127.0.0.1:$PORT (temp database)"
(cd "$WORK" && exec "$BIN/intermute" serve --db "$WORK/intermute.db" --port "$PORT" --host 127.0.0.1) >"$WORK/intermute.log" 2>&1 &
INTERMUTE_PID=$!
for _ in $(seq 1 50); do
  curl -sf "http://127.0.0.1:$PORT/health" >/dev/null 2>&1 && break
  sleep 0.1
done
curl -sf "http://127.0.0.1:$PORT/health" >/dev/null || { echo "intermute did not start:"; cat "$WORK/intermute.log"; exit 1; }

say "Starting a private tmux server with two agent sessions"
tmux -L "$TMUX_NAME" new -d -s claude-demo-alpha-1 -c "$WORK/proj" 'printf "✻ Thinking…\n"; sleep 600'
tmux -L "$TMUX_NAME" new -d -s claude-demo-beta-1  -c "$WORK/proj" 'printf "$ "; sleep 600'
TMUX_SOCKET="$(tmux -L "$TMUX_NAME" display -p '#{socket_path}')"

say "Two agents negotiate one file"
INTERMUTE_URL="http://127.0.0.1:$PORT" DEMO_PROJECT="$WORK/proj" DEMO_LOG_DIR="$WORK" \
INTERLOCK_MCP="$BIN/interlock-mcp" INTERMUX_MCP="$BIN/intermux-mcp" TMUX_SOCKET="$TMUX_SOCKET" \
  python3 "$HERE/demo.py"

say "Done. Everything above ran against throwaway state in $WORK."
