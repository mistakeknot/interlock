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

# NOTE: CLAUDE_SESSION_ID is written by the host's session-start hook when present
# (canonical writer). Do NOT duplicate here — both hooks run async, creating a race
# condition (iv-erb1).

# --- Shared-filesystem coordination model (interlock 0.2.16) ---
# Interlock no longer creates a per-session git worktree. Each session created an
# unconditional `git worktree add` under ~/.cache/interlock/worktrees/ with no
# cleanup path (stop.sh retained them; the planned TTL sweeper was never built),
# leaking GB of orphaned worktrees over time. The isolation it provided was also
# redundant with consuming projects' own worktree discipline (e.g. elf-revel's
# session-spawn.sh) and self-contradictory: agents isolated in private worktrees
# cannot collide, which made the file-reservation layer — interlock's actual value —
# coordinate nothing.
#
# Shared-FS model: all sessions work in the ONE real checkout. Collisions are
# prevented (not merge-resolved) via the reservation layer:
#   - pre-edit.sh reserves files before editing (advisory conflict warning)
#   - commits serialized via .git/commit.lock (commit_lock_path)
#   - the pre-commit hook blocks committing a peer's reserved file
# See docs/shared-fs-coordination.md for the tiered semantic-conflict roadmap.
GIT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || GIT_ROOT=""
if [[ -n "$GIT_ROOT" && -n "${CLAUDE_ENV_FILE:-}" ]]; then
    printf 'export INTERLOCK_PROJECT_ROOT=%q\n' "$GIT_ROOT" >> "$CLAUDE_ENV_FILE"
fi

# Self-heal: reclaim worktrees leaked by interlock <=0.2.15 (best-effort,
# throttled to once per day per machine via a flag file). Backgrounded so it
# never delays session start.
SWEEP_FLAG="${HOME}/.cache/interlock/.last-sweep"
SWEEP_SCRIPT="${SCRIPT_DIR}/../scripts/interlock-orphan-sweep.sh"
if [[ -x "$SWEEP_SCRIPT" ]]; then
    if [[ -z "$(find "$SWEEP_FLAG" -mtime -1 2>/dev/null)" ]]; then
        mkdir -p "$(dirname "$SWEEP_FLAG")" 2>/dev/null || true
        touch "$SWEEP_FLAG" 2>/dev/null || true
        ( bash "$SWEEP_SCRIPT" >/dev/null 2>&1 & ) || true
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
    "additionalContext": "INTERLOCK: Coordination active. Registered as '${AGENT_NAME}' (${AGENT_ID:0:8}...). ${AGENT_COUNT} agent(s) online. Shared-filesystem mode: you share this working tree with other agents — there is no private worktree. Reserve files before editing (the pre-edit hook does this automatically and warns on conflict) so you don't clobber a peer's in-progress work. Commits are serialized via a lockfile; the pre-commit hook blocks committing a file another agent has reserved."
  }
}
ENDJSON

exit 0
