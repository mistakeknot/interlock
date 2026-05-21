#!/usr/bin/env bash
set -uo pipefail
trap 'exit 0' ERR

# Read hook input from stdin (must happen before anything else consumes it)
HOOK_INPUT=$(cat)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"

# Check join flag -- if not joined, exit silently
is_joined || exit 0

# Extract session_id from hook JSON
SESSION_ID=$(echo "$HOOK_INPUT" | jq -r '.session_id // empty' 2>/dev/null) || SESSION_ID=""
[[ -n "$SESSION_ID" ]] || exit 0

# NOTE: CLAUDE_SESSION_ID is written by Clavain's session-start.sh (canonical writer).
# Do NOT duplicate here — both hooks run async, creating a race condition (iv-erb1).

# --- Per-session git worktree isolation ---
# Each session gets a linked worktree instead of a synthetic git index.
# A real worktree has its own working directory and index, so stale session
# indexes cannot commit peer changes as phantom deletes.
GIT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || GIT_ROOT=""
SESSION_WORKTREE=""
if [[ -n "$GIT_ROOT" && -n "${CLAUDE_ENV_FILE:-}" ]]; then
    SESSION_WORKTREE="$(session_worktree_path "$SESSION_ID" "$GIT_ROOT")"
    if [[ -n "$SESSION_WORKTREE" ]]; then
        mkdir -p "$(dirname "$SESSION_WORKTREE")" 2>/dev/null || true
        if [[ ! -e "$SESSION_WORKTREE" ]]; then
            command git -C "$GIT_ROOT" worktree add --detach "$SESSION_WORKTREE" HEAD >/dev/null 2>&1 || true
        fi

        if command git -C "$SESSION_WORKTREE" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
            printf 'export INTERLOCK_PROJECT_ROOT=%q\n' "$GIT_ROOT" >> "$CLAUDE_ENV_FILE"
            printf 'export INTERLOCK_SESSION_WORKTREE=%q\n' "$SESSION_WORKTREE" >> "$CLAUDE_ENV_FILE"
            echo "export INTERLOCK_WORKTREE_READY=1" >> "$CLAUDE_ENV_FILE"
        else
            SESSION_WORKTREE=""
        fi
    fi
fi

# Delegate registration to helper script
RESULT=$("${SCRIPT_DIR}/../scripts/interlock-register.sh" "$SESSION_ID" 2>/dev/null) || RESULT=""

if [[ -z "$RESULT" ]]; then
    # Registration failed (intermute unreachable or error) -- silent degradation
    exit 0
fi

# Parse agent_id and agent_name from result
AGENT_ID=$(echo "$RESULT" | jq -r '.agent_id // empty' 2>/dev/null) || AGENT_ID=""
AGENT_NAME=$(echo "$RESULT" | jq -r '.name // empty' 2>/dev/null) || AGENT_NAME=""

# Export agent identity to CLAUDE_ENV_FILE
if [[ -n "${CLAUDE_ENV_FILE:-}" && -n "$AGENT_ID" ]]; then
    echo "export INTERMUTE_AGENT_ID=${AGENT_ID}" >> "$CLAUDE_ENV_FILE"
    echo "export INTERMUTE_AGENT_NAME=${AGENT_NAME}" >> "$CLAUDE_ENV_FILE"
fi

# Write agent details to temp file
echo "$RESULT" > "$(agent_file_path "$SESSION_ID")"

# Mark connectivity established
touch "$(connected_flag_path "$SESSION_ID")"

# Emit signal: agent registered
SIGNAL_SCRIPT="${SCRIPT_DIR}/../scripts/interlock-signal.sh"
if [[ -x "$SIGNAL_SCRIPT" ]]; then
    bash "$SIGNAL_SCRIPT" reserve "agent registered: ${AGENT_NAME}" 2>/dev/null || true
fi

# Inject coordination context
AGENT_COUNT=$(echo "$RESULT" | jq -r '.agent_count // "?"' 2>/dev/null) || AGENT_COUNT="?"
cat <<ENDJSON
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "INTERLOCK: Coordination active. Registered as '${AGENT_NAME}' (${AGENT_ID:0:8}...). ${AGENT_COUNT} agent(s) online. Per-session git worktree isolation enabled${SESSION_WORKTREE:+ at ${SESSION_WORKTREE}}. Edit files in INTERLOCK_SESSION_WORKTREE; the original checkout is read-only while coordination is active. File reservations enforced via git pre-commit hook. Commits serialized via lockfile."
  }
}
ENDJSON

exit 0
