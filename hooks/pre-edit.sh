#!/usr/bin/env bash
# PreToolUse:Edit hook -- advisory conflict warning (never blocks).
set -euo pipefail

# Guard: fail-open if jq is not available
command -v jq &>/dev/null || exit 0

# Read hook input
INPUT=$(cat)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"

# Skip if not in coordination mode
[[ -n "${INTERMUTE_AGENT_ID:-}" ]] || exit 0

# Extract file path from Edit tool input
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null) || FILE_PATH=""
[[ -n "$FILE_PATH" ]] || exit 0

SESSION_ID="${CLAUDE_SESSION_ID:-unknown}"

# Delegate conflict check to helper script
CONFLICT=$("${SCRIPT_DIR}/../scripts/interlock-check.sh" "$FILE_PATH" "$INTERMUTE_AGENT_ID" 2>/dev/null) || {
    # intermute unreachable -- check if we lost connectivity
    CONNECTED_FLAG="$(connected_flag_path "$SESSION_ID")"
    if [[ -f "$CONNECTED_FLAG" ]]; then
        # First unreachable detection since last connectivity -- emit one-time warning
        rm -f "$CONNECTED_FLAG"
        cat <<ENDJSON
{"additionalContext": "INTERLOCK WARNING: intermute coordination lost. Proceeding without reservation checks."}
ENDJSON
    fi
    exit 0
}

# No conflict -- exit silently
[[ -n "$CONFLICT" ]] || exit 0

# Parse conflict details
HELD_BY=$(echo "$CONFLICT" | jq -r '.held_by // "unknown"' 2>/dev/null) || HELD_BY="unknown"
REASON=$(echo "$CONFLICT" | jq -r '.reason // ""' 2>/dev/null) || REASON=""
EXPIRES=$(echo "$CONFLICT" | jq -r '.expires_at // ""' 2>/dev/null) || EXPIRES=""

# Format expiry for human readability
EXPIRES_DISPLAY="$EXPIRES"
if [[ -n "$EXPIRES" ]] && command -v date &>/dev/null; then
    EXPIRES_EPOCH=$(date -d "$EXPIRES" +%s 2>/dev/null || echo "")
    if [[ -n "$EXPIRES_EPOCH" ]]; then
        NOW_EPOCH=$(date +%s)
        REMAINING_MIN=$(( (EXPIRES_EPOCH - NOW_EPOCH) / 60 ))
        if [[ $REMAINING_MIN -gt 0 ]]; then
            EXPIRES_DISPLAY="in ${REMAINING_MIN}m"
        else
            EXPIRES_DISPLAY="expired"
        fi
    fi
fi

# Build reason display
REASON_DISPLAY=""
if [[ -n "$REASON" ]]; then
    REASON_DISPLAY="\"${REASON}\", "
fi

# Advisory output -- NOT blocking (no decision field, exit 0)
cat <<ENDJSON
{"additionalContext": "INTERLOCK: ${FILE_PATH} reserved by ${HELD_BY} (${REASON_DISPLAY}expires ${EXPIRES_DISPLAY}). Recover: (1) work on other files, (2) request_release(agent_name=\"${HELD_BY}\"), (3) wait for expiry. Note: git commit will block until resolved."}
ENDJSON

exit 0
