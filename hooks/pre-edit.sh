#!/usr/bin/env bash
# PreToolUse:Edit hook -- blocks edits to files reserved by other agents,
# auto-reserves files on first edit by this agent.
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

# --- Check inbox for commit notifications (throttled) ---
# When another session commits, the postcommit hook sends a "commit:<hash>"
# message to our inbox. We pull those changes before checking conflicts so
# our reservation checks are against the latest state.
PULL_FLAG=$(inbox_check_path "$SESSION_ID")
if [[ ! -f "$PULL_FLAG" ]] || ! find "$PULL_FLAG" -mmin -0.5 -print -quit 2>/dev/null | grep -q .; then
    # Cache expired (or first check) — touch flag and query inbox
    touch "$PULL_FLAG" 2>/dev/null || true

    INBOX_JSON=$(intermute_curl GET "/api/messages/inbox?agent=${INTERMUTE_AGENT_ID}&unread=true" 2>/dev/null) || INBOX_JSON=""

    if [[ -n "$INBOX_JSON" ]] && command -v jq &>/dev/null; then
        COMMIT_MSGS=$(echo "$INBOX_JSON" | jq -r '
            [.messages[]? | select(.subject | startswith("commit:"))] | if length > 0 then . else empty end
        ' 2>/dev/null) || COMMIT_MSGS=""

        if [[ -n "$COMMIT_MSGS" && "$COMMIT_MSGS" != "null" ]]; then
            # Pull remote changes (rebase to keep our work on top)
            if git pull --rebase 2>/dev/null; then
                PULL_CONTEXT="INTERLOCK: auto-pulled after commit(s) by other agent(s)."
            else
                # Rebase conflict — abort and warn, but don't block the edit
                git rebase --abort 2>/dev/null || true
                PULL_CONTEXT="INTERLOCK WARNING: auto-pull had conflicts, aborted rebase. Manual merge may be needed."
            fi

            # Acknowledge commit messages so we don't re-process them
            echo "$COMMIT_MSGS" | jq -r '.[].id // empty' 2>/dev/null | while IFS= read -r msg_id; do
                [[ -n "$msg_id" ]] && intermute_curl POST "/api/messages/${msg_id}/ack" 2>/dev/null || true
            done

            # Emit advisory context about the pull (if we have something to say)
            if [[ -n "${PULL_CONTEXT:-}" ]]; then
                cat <<ENDJSON
{"additionalContext": "${PULL_CONTEXT}"}
ENDJSON
            fi
        fi
    fi
fi

# Make file path relative to project root
REL_PATH="$FILE_PATH"
PROJECT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || PROJECT_ROOT=""
if [[ -n "$PROJECT_ROOT" && "$FILE_PATH" == "$PROJECT_ROOT"* ]]; then
    REL_PATH="${FILE_PATH#$PROJECT_ROOT/}"
fi

PROJECT="${INTERMUTE_PROJECT:-$(basename "$PROJECT_ROOT" 2>/dev/null)}"
[[ -n "$PROJECT" ]] || exit 0

# --- Check for conflicts with other agents ---
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

# --- If conflict found: BLOCK the edit ---
if [[ -n "$CONFLICT" ]]; then
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

    REASON_DISPLAY=""
    if [[ -n "$REASON" ]]; then
        REASON_DISPLAY="\"${REASON}\", "
    fi

    # BLOCKING response -- prevents the edit from proceeding
    cat <<ENDJSON
{"decision": "block", "reason": "INTERLOCK: ${REL_PATH} is exclusively reserved by ${HELD_BY} (${REASON_DISPLAY}expires ${EXPIRES_DISPLAY}). Work on other files, use request_release(agent_name=\"${HELD_BY}\"), or wait for expiry."}
ENDJSON
    exit 0
fi

# --- No conflict: auto-reserve this file ---
# Create/renew an exclusive reservation for this file (15min TTL).
# This is best-effort: if it fails, we still allow the edit.
RESERVE_PAYLOAD=$(jq -nc \
    --arg agent "$INTERMUTE_AGENT_ID" \
    --arg project "$PROJECT" \
    --arg pattern "$REL_PATH" \
    --arg reason "auto-reserve: editing" \
    '{agent_id:$agent, project:$project, path_pattern:$pattern, exclusive:true, reason:$reason, ttl_minutes:15}')

intermute_curl POST "/api/reservations" \
    -H "Content-Type: application/json" \
    -d "$RESERVE_PAYLOAD" >/dev/null 2>&1 || true

exit 0
