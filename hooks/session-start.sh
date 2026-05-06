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

# --- Per-session git index isolation ---
# Each session gets its own GIT_INDEX_FILE so concurrent git-add operations
# don't contend on .git/index.lock. The index is initialized from HEAD so
# the session sees the current repo state. Commits are serialized separately
# via flock in the pre-commit hook.
#
# We install a git() shell function (not a global export) so the per-session
# index applies only when cwd is inside the project root. A global export of
# GIT_INDEX_FILE would leak into git operations on sibling repos (e.g.
# `ic publish` shelling out to `git -C marketplace`), which corrupts their
# state by reading the wrong index against the wrong tree.
GIT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || GIT_ROOT=""
if [[ -n "$GIT_ROOT" && -n "${CLAUDE_ENV_FILE:-}" ]]; then
    SESSION_INDEX="${GIT_ROOT}/.git/index-${SESSION_ID}"
    cat >> "$CLAUDE_ENV_FILE" <<EOF
git() {
  # If any flag explicitly redirects git's working tree or git dir, we are
  # targeting a different repo than the project root — never apply our
  # per-session GIT_INDEX_FILE. This catches \`git -C /other/repo …\` which
  # the cwd-only check missed.
  local _arg
  for _arg in "\$@"; do
    case "\$_arg" in
      -C|--git-dir|--git-dir=*|--work-tree|--work-tree=*)
        ( unset GIT_INDEX_FILE; command git "\$@" )
        return \$?
        ;;
    esac
  done
  local _cwd
  _cwd=\$(pwd -P)
  if [[ "\$_cwd" == "${GIT_ROOT}" || "\$_cwd" == "${GIT_ROOT}"/* ]]; then
    # Walk from cwd up to GIT_ROOT; if any intermediate directory carries
    # its own .git, we are inside a nested repo (not a submodule of the
    # project root). Applying the project's session index there would
    # write trees into the wrong object store and produce
    # "fatal: unable to read <hash>" on subsequent reads.
    local _check="\$_cwd"
    while [[ "\$_check" != "${GIT_ROOT}" && "\$_check" != "/" ]]; do
      if [[ -e "\$_check/.git" ]]; then
        ( unset GIT_INDEX_FILE; command git "\$@" )
        return \$?
      fi
      _check=\$(dirname "\$_check")
    done
    GIT_INDEX_FILE="${SESSION_INDEX}" command git "\$@"
  else
    ( unset GIT_INDEX_FILE; command git "\$@" )
  fi
}
export -f git 2>/dev/null || true
EOF
    # Initialize the session index from HEAD if it doesn't exist yet.
    # Use `command git` to bypass any wrapper that might already exist.
    if [[ ! -f "$SESSION_INDEX" ]]; then
        GIT_INDEX_FILE="$SESSION_INDEX" command git read-tree HEAD 2>/dev/null || true
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
    "additionalContext": "INTERLOCK: Coordination active. Registered as '${AGENT_NAME}' (${AGENT_ID:0:8}...). ${AGENT_COUNT} agent(s) online. Per-session git index isolation enabled. File reservations enforced via git pre-commit hook. Commits serialized via lockfile."
  }
}
ENDJSON

exit 0
