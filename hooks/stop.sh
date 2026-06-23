#!/usr/bin/env bash
# Stop hook: release all reservations and clean up temp files.
set -uo pipefail
trap 'exit 0' ERR

# Guard: fail-open if jq is not available
command -v jq &>/dev/null || exit 0

# Read hook input
INPUT=$(cat)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"

# Skip if not in coordination mode
[[ -n "${INTERMUTE_AGENT_ID:-}" ]] || exit 0

# Guard: if stop hook is already active, don't re-trigger
STOP_ACTIVE=$(echo "$INPUT" | jq -r '.stop_hook_active // false' 2>/dev/null) || STOP_ACTIVE="false"
[[ "$STOP_ACTIVE" != "true" ]] || exit 0

SESSION_ID="${CLAUDE_SESSION_ID:-unknown}"

# As of 0.2.16 interlock no longer creates per-session worktrees (shared-FS model),
# so there is nothing to retain or reap here. Reservation/temp cleanup only.

# Delegate cleanup to helper script (best effort)
"${SCRIPT_DIR}/../scripts/interlock-cleanup.sh" \
    "$INTERMUTE_AGENT_ID" "$SESSION_ID" 2>/dev/null || true

# Emit signal: agent deregistered
SIGNAL_SCRIPT="${SCRIPT_DIR}/../scripts/interlock-signal.sh"
if [[ -x "$SIGNAL_SCRIPT" ]]; then
    bash "$SIGNAL_SCRIPT" release "agent deregistered: all reservations released" 2>/dev/null || true
fi

exit 0
