#!/usr/bin/env bash
set -euo pipefail

# Read hook input from stdin (must happen before anything else consumes it)
HOOK_INPUT=$(cat)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"

# Check join flag -- if not joined, exit silently
is_joined || exit 0

# Extract session_id from hook JSON
SESSION_ID=$(echo "$HOOK_INPUT" | jq -r '.session_id // empty' 2>/dev/null) || SESSION_ID=""
[[ -n "$SESSION_ID" ]] || exit 0

# Persist session_id to CLAUDE_ENV_FILE for downstream hooks
if [[ -n "${CLAUDE_ENV_FILE:-}" ]]; then
    echo "export CLAUDE_SESSION_ID=${SESSION_ID}" >> "$CLAUDE_ENV_FILE"
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
    "additionalContext": "INTERLOCK: Coordination active. Registered as '${AGENT_NAME}' (${AGENT_ID:0:8}...). ${AGENT_COUNT} agent(s) online. File reservations enforced via git pre-commit hook."
  }
}
ENDJSON

exit 0
