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

# --- Check inbox for release-request messages (advisory, feature-flagged) ---
if [[ "${INTERLOCK_AUTO_RELEASE:-0}" == "1" ]]; then
    NEG_FLAG=$(negotiation_check_path "$SESSION_ID")
    if [[ ! -f "$NEG_FLAG" ]] || ! find "$NEG_FLAG" -mmin -0.5 -print -quit 2>/dev/null | grep -q .; then
        touch "$NEG_FLAG" 2>/dev/null || true

        # Fetch inbox with circuit breaker (fail-open on timeout/error)
        NEG_INBOX=$(intermute_curl_fast GET "/api/messages/inbox?agent=${INTERMUTE_AGENT_ID}&unread=true&limit=50" 2>/dev/null) || NEG_INBOX=""

        if [[ -n "$NEG_INBOX" ]]; then
            # Find release-request messages
            RELEASE_REQS=$(echo "$NEG_INBOX" | jq -r '
                [.messages[]? | select(
                    (.subject // "") == "release-request" or
                    ((.body // "") | try fromjson | .type) == "release-request"
                )] | if length > 0 then . else empty end
            ' 2>/dev/null) || RELEASE_REQS=""

            if [[ -n "$RELEASE_REQS" && "$RELEASE_REQS" != "null" ]]; then
                # Build advisory context -- tell agent about pending release requests
                ADVISORY=""
                while IFS= read -r req_msg; do
                    REQ_BODY=$(echo "$req_msg" | jq -r '.body // ""' 2>/dev/null) || continue
                    REQ_FILE=$(echo "$REQ_BODY" | jq -r 'try fromjson | .file // .pattern // empty' 2>/dev/null) || continue
                    REQ_THREAD=$(echo "$req_msg" | jq -r '.thread_id // empty' 2>/dev/null) || REQ_THREAD=""
                    REQ_FROM=$(echo "$req_msg" | jq -r '.from // empty' 2>/dev/null) || REQ_FROM=""
                    REQ_URGENCY=$(echo "$REQ_BODY" | jq -r 'try fromjson | .urgency // "normal"' 2>/dev/null) || REQ_URGENCY="normal"

                    [[ -z "$REQ_FILE" ]] && continue

                    ADVISORY="${ADVISORY}${REQ_FROM} requests release of ${REQ_FILE} (urgency: ${REQ_URGENCY}). "
                    if [[ -n "$REQ_THREAD" ]]; then
                        ADVISORY="${ADVISORY}Use respond_to_release(thread_id='${REQ_THREAD}', requester='${REQ_FROM}', action='release', file='${REQ_FILE}') to release, or action='defer' to request more time. "
                    fi
                done < <(echo "$RELEASE_REQS" | jq -c '.[]' 2>/dev/null)

                if [[ -n "${ADVISORY:-}" ]]; then
                    jq -nc --arg ctx "INTERLOCK: ${ADVISORY}" '{"additionalContext": $ctx}'
                fi
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

# --- Check for conflicts and auto-reserve ---
# Preferred: use ic coordination (atomic reserve, eliminates TOCTOU).
# Fallback: use intermute HTTP API via interlock-check.sh.
if command -v ic &>/dev/null && ic version &>/dev/null 2>&1; then
    # Single atomic reserve call: if conflict exists, returns exit 1 with conflict info.
    # If clear, creates the reservation (no separate check-then-reserve race).
    # SAFETY: use jq --arg to prevent shell injection from file paths and blocker values.
    SCOPE="${PROJECT_ROOT:-$PWD}"
    result=$(ic --json coordination reserve \
        --owner="$INTERMUTE_AGENT_ID" \
        --scope="$SCOPE" \
        --pattern="$REL_PATH" \
        --ttl=900 \
        --reason="auto-reserve: editing" 2>/dev/null)
    rc=$?

    if [[ $rc -eq 1 ]]; then
        # Conflict found — Reserve returned conflict info
        blocker=$(echo "$result" | jq -r '.conflict.blocker_owner // "unknown"' 2>/dev/null) || blocker="unknown"
        reason=$(echo "$result" | jq -r '.conflict.blocker_reason // ""' 2>/dev/null) || reason=""
        reason_display=""
        if [[ -n "$reason" ]]; then
            reason_display="\"${reason}\", "
        fi
        jq -nc --arg fp "$REL_PATH" --arg bl "$blocker" --arg rd "$reason_display" \
            '{"decision":"block","reason":"INTERLOCK: \($fp) reserved by \($bl) (\($rd)use request_release or wait for expiry)."}'
        exit 0
    elif [[ $rc -eq 0 ]]; then
        # Reserved successfully — allow the edit
        exit 0
    fi
    # rc >= 2 means ic error — fall through to HTTP path
fi

# Fallback: check via intermute HTTP API
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
