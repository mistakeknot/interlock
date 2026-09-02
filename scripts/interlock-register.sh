#!/usr/bin/env bash
# Register this agent with intermute.
# Args: $1 = session_id
# Output: JSON with agent_id, name, session_id, agent_count on stdout
# Exit: 0 on success, 1 on failure
set -euo pipefail

SESSION_ID="${1:?session_id required}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
source "${SCRIPT_DIR}/../hooks/lib.sh"

# Determine agent name
AGENT_NAME=""
NAME_FILE="${HOME}/.config/interlock/agent-name"
LEGACY_NAME_FILE="${HOME}/.config/clavain/intermute-agent-name"
if [[ -f "$NAME_FILE" ]]; then
    AGENT_NAME="$(head -1 "$NAME_FILE" 2>/dev/null | tr -d '\n')"
elif [[ -f "$LEGACY_NAME_FILE" ]]; then
    AGENT_NAME="$(head -1 "$LEGACY_NAME_FILE" 2>/dev/null | tr -d '\n')"
fi
if [[ -z "$AGENT_NAME" ]] && command -v tmux &>/dev/null; then
    AGENT_NAME="$(tmux display-message -p '#T' 2>/dev/null || true)"
fi
if [[ -z "$AGENT_NAME" ]]; then
    AGENT_NAME="claude-${SESSION_ID:0:8}"
fi

# Detect project name from git or directory
PROJECT=""
if command -v git &>/dev/null && git rev-parse --show-toplevel &>/dev/null 2>&1; then
    PROJECT="$(basename "$(git rev-parse --show-toplevel 2>/dev/null)")"
else
    PROJECT="$(basename "$PWD")"
fi

# Collect capabilities from installed plugins' agentCapabilities in plugin.json,
# then merge any per-agent override file. Result: deduplicated JSON array.
CAPABILITIES="[]"

# 1. Scan installed plugins for agentCapabilities declarations
PLUGIN_CACHE="${HOME}/.claude/plugins/cache"
if [[ -d "$PLUGIN_CACHE" ]]; then
    PLUGIN_CAPS=$(find "$PLUGIN_CACHE" -path '*/.claude-plugin/plugin.json' -print0 \
        | xargs -0 jq -r '.agentCapabilities // {} | [.[]] | add // empty | .[]' 2>/dev/null \
        | sort -u | jq -Rn '[inputs | select(length > 0)]') || PLUGIN_CAPS="[]"
    if [[ "$PLUGIN_CAPS" != "[]" ]] && [[ -n "$PLUGIN_CAPS" ]]; then
        CAPABILITIES="$PLUGIN_CAPS"
    fi
fi

# 2. Merge per-agent override file (backward compatible supplement)
CAPS_FILE="${HOME}/.config/clavain/capabilities-${AGENT_NAME}.json"
if [[ -f "$CAPS_FILE" ]]; then
    AGENT_CAPS=$(jq -c '.' "$CAPS_FILE" 2>/dev/null)
    if [[ -n "$AGENT_CAPS" ]] && [[ "$AGENT_CAPS" != "null" ]]; then
        CAPABILITIES=$(jq -nc --argjson a "$CAPABILITIES" --argjson b "$AGENT_CAPS" '$a + $b | unique')
    fi
fi

# --- Window identity resolution ---
# Resolve a stable window UUID so agent_id persists across session restarts.
# Priority: INTERLOCK_WINDOW_ID env > SHA1-UUID from TMUX_PANE > empty (no persistence)
WINDOW_UUID=""
if [[ -n "${INTERLOCK_WINDOW_ID:-}" ]]; then
    WINDOW_UUID="$INTERLOCK_WINDOW_ID"
elif [[ -n "${TMUX_PANE:-}" ]]; then
    # Derive deterministic UUID from TMUX_PANE using sha1sum
    WINDOW_UUID=$(printf '%s' "interlock:${TMUX_PANE}" | sha1sum | cut -c1-32)
fi

AGENT_ID_DEFAULT="claude-${SESSION_ID:0:8}"
AGENT_ID_OVERRIDE=""

if [[ -n "$WINDOW_UUID" ]]; then
    # Look up existing window identity
    WINDOWS_RESP=$(intermute_curl GET "/api/windows?project=${PROJECT}" 2>/dev/null) || WINDOWS_RESP=""
    if [[ -n "$WINDOWS_RESP" ]]; then
        EXISTING=$(echo "$WINDOWS_RESP" | jq -r --arg uuid "$WINDOW_UUID" \
            '.windows[]? | select(.window_uuid == $uuid)' 2>/dev/null) || EXISTING=""
        if [[ -n "$EXISTING" ]]; then
            AGENT_ID_OVERRIDE=$(echo "$EXISTING" | jq -r '.agent_id // empty' 2>/dev/null) || AGENT_ID_OVERRIDE=""
            EXISTING_NAME=$(echo "$EXISTING" | jq -r '.display_name // empty' 2>/dev/null) || EXISTING_NAME=""
            if [[ -n "$EXISTING_NAME" ]]; then
                AGENT_NAME="$EXISTING_NAME"
            fi
        fi
    fi
fi

REGISTER_ID="${AGENT_ID_OVERRIDE:-$AGENT_ID_DEFAULT}"

# POST to intermute /api/agents
RESPONSE=$(intermute_curl POST "/api/agents" \
    -H "Content-Type: application/json" \
    -d "$(jq -n \
        --arg id "$REGISTER_ID" \
        --arg name "$AGENT_NAME" \
        --arg project "$PROJECT" \
        --arg session_id "$SESSION_ID" \
        --argjson capabilities "$CAPABILITIES" \
        '{id: $id, name: $name, project: $project, session_id: $session_id, capabilities: $capabilities}')" \
    2>/dev/null) || exit 1

AGENT_ID=$(echo "$RESPONSE" | jq -r '.agent_id // .id // empty' 2>/dev/null) || exit 1
[[ -n "$AGENT_ID" ]] || exit 1

# Create/update window identity mapping if we have a window UUID
if [[ -n "$WINDOW_UUID" ]]; then
    intermute_curl POST "/api/windows" \
        -H "Content-Type: application/json" \
        -d "$(jq -n \
            --arg project "$PROJECT" \
            --arg window_uuid "$WINDOW_UUID" \
            --arg agent_id "$AGENT_ID" \
            --arg display_name "$AGENT_NAME" \
            '{project: $project, window_uuid: $window_uuid, agent_id: $agent_id, display_name: $display_name}')" \
        >/dev/null 2>&1 || true  # Non-fatal: window identity is advisory

    # Export for subsequent hooks in this session
    export INTERLOCK_WINDOW_ID="$WINDOW_UUID"
fi

# Get agent count for context injection
AGENTS_RESPONSE=$(intermute_curl GET "/api/agents?project=${PROJECT}" 2>/dev/null) || AGENTS_RESPONSE=""
AGENT_COUNT=$(echo "$AGENTS_RESPONSE" | jq -r '.agents | length // 0' 2>/dev/null) || AGENT_COUNT="?"

# Output structured result
jq -n \
    --arg agent_id "$AGENT_ID" \
    --arg name "$AGENT_NAME" \
    --arg session_id "$SESSION_ID" \
    --arg project "$PROJECT" \
    --arg agent_count "$AGENT_COUNT" \
    '{agent_id: $agent_id, name: $name, session_id: $session_id, project: $project, agent_count: $agent_count}'

exit 0
