#!/usr/bin/env bash
# SubagentStop hook: release reservations held by a completing subagent.
#
# Problem: When subagents (spawned via the Agent tool) crash or finish without
# calling release_files, their reservations linger as phantom locks. The
# parent session's Stop hook only fires at full-session end, so phantom locks
# can accumulate across many subagent invocations.
#
# This hook fires on SubagentStop (Claude Code 2.x) and runs the same
# cleanup path used by the Stop hook, scoped to the subagent's agent ID.
#
# Behavior:
# - If subagent never registered with interlock (no INTERMUTE_AGENT_ID inherited),
#   no-op.
# - If subagent had reservations, release them.
# - Always exits 0 (never blocks parent session).
set -uo pipefail
trap 'exit 0' ERR

# Guard: jq required for payload parsing
command -v jq &>/dev/null || exit 0

# Read hook payload (Claude Code SubagentStop sends JSON to stdin).
INPUT=$(cat 2>/dev/null || true)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh" 2>/dev/null || exit 0

# Resolve agent ID. Subagents inherit env from parent, so INTERMUTE_AGENT_ID
# is the parent's identifier — that's still useful because reservations are
# tracked per agent. If the subagent ran completely outside coordination
# (no INTERMUTE_AGENT_ID), we have nothing to clean.
AGENT_ID="${INTERMUTE_AGENT_ID:-}"
[[ -n "$AGENT_ID" ]] || exit 0

# Resolve session ID. The SubagentStop payload typically includes session_id
# (it's the parent session that spawned the subagent).
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty' 2>/dev/null) || SESSION_ID=""
SESSION_ID="${SESSION_ID:-${CLAUDE_SESSION_ID:-unknown}}"

# Optional: subagent ID, if Claude Code includes it. Used for finer-grained
# release if interlock supports per-subagent reservation scoping in future.
SUBAGENT_ID=$(echo "$INPUT" | jq -r '.subagent_id // empty' 2>/dev/null) || SUBAGENT_ID=""

# Check whether the parent agent currently has ANY reservations. If not,
# skip the cleanup pass — saves a network round-trip.
RESERVATIONS=$(intermute_curl GET "/api/reservations?agent=${AGENT_ID}" 2>/dev/null) || RESERVATIONS=""
if [[ -z "$RESERVATIONS" ]]; then
    exit 0
fi

ACTIVE_COUNT=$(echo "$RESERVATIONS" | jq '[.reservations[]? | select(.is_active == true)] | length' 2>/dev/null) || ACTIVE_COUNT=0
[[ "$ACTIVE_COUNT" -gt 0 ]] || exit 0

# Reuse the cleanup helper. It releases reservations for the agent ID
# and best-effort exits 0.
"${SCRIPT_DIR}/../scripts/interlock-cleanup.sh" \
    "$AGENT_ID" "$SESSION_ID" 2>/dev/null || true

# Optional: signal release event for observability
SIGNAL_SCRIPT="${SCRIPT_DIR}/../scripts/interlock-signal.sh"
if [[ -x "$SIGNAL_SCRIPT" ]]; then
    bash "$SIGNAL_SCRIPT" release \
        "subagent stopped: ${ACTIVE_COUNT} reservations released (subagent=${SUBAGENT_ID:-?})" \
        2>/dev/null || true
fi

# Persist event to cross-session hook log (best-effort; library may not be present).
if [[ -f "${HOME}/.claude/hooks/lib-hook-log.sh" ]]; then
    # shellcheck disable=SC1091
    source "${HOME}/.claude/hooks/lib-hook-log.sh" 2>/dev/null && \
        declare -f hook_log_info >/dev/null 2>&1 && \
        hook_log_info "interlock-subagent-stop" "released ${ACTIVE_COUNT} reservations (agent=${AGENT_ID} subagent=${SUBAGENT_ID:-?})"
fi

exit 0
