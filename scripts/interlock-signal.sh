#!/usr/bin/env bash
# Signal file writer for interlock companion plugin.
#
# Emits normalized JSONL signal events for interline consumption.
# Append-only writes using >> (O_APPEND, atomic for <4KB on Linux).
#
# Usage: interlock-signal.sh <event-type> <text>
# Event types: reserve (lock/3), release (unlock/3), message (mail/4)
#
# Environment:
#   INTERMUTE_AGENT_ID       — agent UUID (required, no-op if missing)
#   INTERLOCK_PROJECT_SLUG   — project slug (optional, derived from git if unset)
#   INTERLOCK_SIGNAL_DIR     — signal directory (default: /var/run/intermute/signals)
#   INTERBAND_LIB            — optional path to shared interband library
set -euo pipefail

# Guard: fail-open if jq is missing
if ! command -v jq &>/dev/null; then
    exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"

_load_interband() {
    local repo_root=""
    repo_root="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel 2>/dev/null || true)"

    local candidate
    for candidate in \
        "${INTERBAND_LIB:-}" \
        "${SCRIPT_DIR}/../../../infra/interband/lib/interband.sh" \
        "${SCRIPT_DIR}/../../../interband/lib/interband.sh" \
        "${repo_root}/../interband/lib/interband.sh" \
        "${HOME}/.local/share/interband/lib/interband.sh"
    do
        if [[ -n "$candidate" && -f "$candidate" ]]; then
            # shellcheck source=/dev/null
            source "$candidate" && return 0
        fi
    done
    return 1
}

EVENT_TYPE="${1:-}"
TEXT="${2:-}"

# Validate event type
case "$EVENT_TYPE" in
    reserve)  ICON="lock";   PRIORITY=3 ;;
    release)  ICON="unlock"; PRIORITY=3 ;;
    message)  ICON="mail";   PRIORITY=4 ;;
    *)
        echo "error: unknown event type: $EVENT_TYPE (expected: reserve, release, message)" >&2
        exit 1
        ;;
esac

# Guard: no-op without agent identity
AGENT_ID="${INTERMUTE_AGENT_ID:-}"
if [[ -z "$AGENT_ID" ]]; then
    exit 0
fi

# Derive project slug
SLUG="${INTERLOCK_PROJECT_SLUG:-}"
if [[ -z "$SLUG" ]]; then
    SLUG=$(basename "$(git rev-parse --show-toplevel 2>/dev/null)" 2>/dev/null || echo "unknown")
fi

# Signal directory (default: /var/run/intermute/signals)
SIGNAL_DIR="${INTERLOCK_SIGNAL_DIR:-/var/run/intermute/signals}"

# Create directory with mode 0700 if missing
if [[ ! -d "$SIGNAL_DIR" ]]; then
    mkdir -p -m 0700 "$SIGNAL_DIR"
fi

# ISO 8601 UTC timestamp
TS=$(date -u +%FT%TZ)

# Construct JSON line and append
jq -nc \
    --argjson version 1 \
    --arg layer "coordination" \
    --arg icon "$ICON" \
    --arg text "$TEXT" \
    --argjson priority "$PRIORITY" \
    --arg ts "$TS" \
    '{version:$version,layer:$layer,icon:$icon,text:$text,priority:$priority,ts:$ts}' \
    >> "${SIGNAL_DIR}/${SLUG}-${AGENT_ID}.jsonl"

# Structured interband mirror (latest signal snapshot) — best effort.
if _load_interband && type interband_path >/dev/null 2>&1 && type interband_write >/dev/null 2>&1; then
    signal_payload=$(jq -nc \
        --arg layer "coordination" \
        --arg icon "$ICON" \
        --arg text "$TEXT" \
        --argjson priority "$PRIORITY" \
        --arg ts "$TS" \
        '{layer:$layer,icon:$icon,text:$text,priority:$priority,ts:$ts}' 2>/dev/null) || signal_payload=""
    if [[ -n "$signal_payload" ]]; then
        interband_file=$(interband_path "interlock" "coordination" "${SLUG}-${AGENT_ID}" 2>/dev/null) || interband_file=""
        if [[ -n "$interband_file" ]]; then
            interband_write "$interband_file" "interlock" "coordination_signal" "${CLAUDE_SESSION_ID:-}" "$signal_payload" \
                2>/dev/null || true
        fi
    fi
fi
