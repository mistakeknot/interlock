#!/usr/bin/env bash
# Check if a file path conflicts with any active reservation.
# Args: $1 = file_path, $2 = our_agent_id
# Output: JSON conflict details on stdout (empty if no conflict)
# Exit: 0 on success, 1 on intermute unreachable
set -euo pipefail

FILE_PATH="${1:?file_path required}"
OUR_AGENT_ID="${2:?agent_id required}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
source "${SCRIPT_DIR}/../hooks/lib.sh"

# Detect project
PROJECT=""
if command -v git &>/dev/null && git rev-parse --show-toplevel &>/dev/null 2>&1; then
    PROJECT="$(basename "$(git rev-parse --show-toplevel 2>/dev/null)")"
else
    PROJECT="$(basename "$PWD")"
fi

# Make file path relative to project root
REL_PATH="$FILE_PATH"
if command -v git &>/dev/null; then
    PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo "")"
    if [[ -n "$PROJECT_ROOT" && "$FILE_PATH" == "$PROJECT_ROOT"* ]]; then
        REL_PATH="${FILE_PATH#$PROJECT_ROOT/}"
    fi
fi

# Query active reservations for this project
RESPONSE=$(intermute_curl GET "/api/reservations?project=${PROJECT}" 2>/dev/null) || exit 1

# Agents in the project, so a reservation held by an agent that shares our name
# (the MCP server and the hook register separately, see issue #4) counts as ours.
OUR_NAME="${INTERMUTE_AGENT_NAME:-}"
AGENTS=$(intermute_curl GET "/api/agents?project=${PROJECT}" 2>/dev/null) || AGENTS=""
if [[ -z "$AGENTS" ]] || ! echo "$AGENTS" | jq -e . >/dev/null 2>&1; then
    AGENTS='{"agents":[]}'
fi

# Check each active exclusive reservation for a path conflict, excluding ours.
# `. as $r` matters: inside `$path | startswith(...)` the input is the path
# string, so `.path_pattern` there used to index a string and jq aborted,
# which read as "no conflict" (issue #3).
CONFLICT=$(echo "$RESPONSE" | jq -rc --arg path "$REL_PATH" --arg us "$OUR_AGENT_ID" --arg name "$OUR_NAME" --argjson agents "$AGENTS" '
    ([$us] + [ $agents.agents[]? | select($name != "" and .name == $name) | .agent_id ]) as $self
    | .reservations[]?
    | select(.is_active == true)
    | select(.exclusive == true)
    | select(.agent_id as $a | ($self | any(. == $a)) | not)
    | . as $r
    | select(($path | startswith($r.path_pattern | rtrimstr("*"))) or ($r.path_pattern == $path))
    | {held_by: .agent_id,
       held_by_name: ([ $agents.agents[]? | select(.agent_id == $r.agent_id) | .name ][0] // ""),
       reason: .reason, expires_at: .expires_at, pattern: .path_pattern}
' 2>/dev/null | head -1) || CONFLICT=""

# Output conflict (empty string means no conflict)
echo "$CONFLICT"
exit 0
