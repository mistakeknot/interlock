#!/usr/bin/env bash
# Release all reservations and clean up temp files.
# Args: $1 = agent_id, $2 = session_id
# Exit: 0 always (best effort cleanup)
set -euo pipefail

AGENT_ID="${1:?agent_id required}"
SESSION_ID="${2:?session_id required}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
source "${SCRIPT_DIR}/../hooks/lib.sh"

# Get agent's active reservations
RESERVATIONS=$(intermute_curl GET "/api/reservations?agent=${AGENT_ID}" 2>/dev/null) || RESERVATIONS=""

# Release each reservation
if [[ -n "$RESERVATIONS" ]]; then
    echo "$RESERVATIONS" | jq -r '.reservations[]? | select(.is_active == true) | .id' 2>/dev/null | while read -r RES_ID; do
        [[ -n "$RES_ID" ]] || continue
        intermute_curl DELETE "/api/reservations/${RES_ID}" \
            -H "X-Agent-ID: ${AGENT_ID}" \
            2>/dev/null || true
    done
fi

# Clean up temp files
rm -f "$(agent_file_path "$SESSION_ID")" 2>/dev/null || true
rm -f "$(connected_flag_path "$SESSION_ID")" 2>/dev/null || true

# Clean up this agent's interband coordination snapshot (best effort).
PROJECT_SLUG="${INTERLOCK_PROJECT_SLUG:-}"
if [[ -z "$PROJECT_SLUG" ]]; then
    PROJECT_SLUG=$(basename "$(git rev-parse --show-toplevel 2>/dev/null)" 2>/dev/null || echo "unknown")
fi
INTERBAND_DIR="${INTERBAND_ROOT:-${HOME}/.interband}"
rm -f "${INTERBAND_DIR}/interlock/coordination/${PROJECT_SLUG}-${AGENT_ID}.json" 2>/dev/null || true

# Clean up stale temp files from previous sessions (>60 min old)
find /tmp -maxdepth 1 -name 'interlock-agent-*.json' -mmin +60 -delete 2>/dev/null || true
find /tmp -maxdepth 1 -name 'interlock-connected-*' -mmin +60 -delete 2>/dev/null || true

exit 0
