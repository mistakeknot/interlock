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

# Check each reservation for path conflict (excluding our own)
CONFLICT=$(echo "$RESPONSE" | jq -r --arg path "$REL_PATH" --arg us "$OUR_AGENT_ID" '
    .reservations[]
    | select(.agent_id != $us)
    | select(.is_active == true)
    | select(.exclusive == true)
    | select(
        ($path | startswith(.path_pattern | rtrimstr("*"))) or
        (.path_pattern == $path)
    )
    | {held_by: .agent_id, reason: .reason, expires_at: .expires_at, pattern: .path_pattern}
' 2>/dev/null | head -1) || CONFLICT=""

# Output conflict (empty string means no conflict)
echo "$CONFLICT"
exit 0
