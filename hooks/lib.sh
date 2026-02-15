#!/usr/bin/env bash
# Shared utilities for interlock hooks.

JOIN_FLAG="${HOME}/.config/clavain/intermute-joined"

# is_joined returns 0 if the user has opted into coordination.
is_joined() {
    [[ -f "$JOIN_FLAG" ]]
}

# intermute_url returns the base URL for intermute.
intermute_url() {
    if [[ -n "${INTERMUTE_SOCKET:-}" && -S "${INTERMUTE_SOCKET}" ]]; then
        echo "socket"
    else
        echo "${INTERMUTE_URL:-http://127.0.0.1:7338}"
    fi
}

# intermute_curl wraps curl for intermute API calls.
# Usage: intermute_curl METHOD PATH [extra-curl-args...]
intermute_curl() {
    local method="$1"; shift
    local path="$1"; shift

    local curl_args=(curl -sf --connect-timeout 2 --max-time 5 -X "$method")

    if [[ -n "${INTERMUTE_SOCKET:-}" && -S "${INTERMUTE_SOCKET}" ]]; then
        curl_args+=(--unix-socket "$INTERMUTE_SOCKET" "http://localhost${path}")
    else
        local base="${INTERMUTE_URL:-http://127.0.0.1:7338}"
        curl_args+=("${base}${path}")
    fi

    curl_args+=("$@")
    "${curl_args[@]}"
}

# agent_file_path returns the temp file path for agent details.
agent_file_path() {
    echo "/tmp/interlock-agent-${1}.json"
}

# connected_flag_path returns the connectivity flag file path.
connected_flag_path() {
    echo "/tmp/interlock-connected-${1}"
}

# git_root returns the git repository root, or empty string.
git_root() {
    git rev-parse --show-toplevel 2>/dev/null || echo ""
}

# session_index_path returns the per-session git index file path.
# Usage: session_index_path <session_id>
session_index_path() {
    local root
    root=$(git_root)
    [[ -n "$root" ]] && echo "${root}/.git/index-${1}" || echo ""
}

# commit_lock_path returns the flock file path for serialized commits.
commit_lock_path() {
    local root
    root=$(git_root)
    [[ -n "$root" ]] && echo "${root}/.git/commit.lock" || echo ""
}

# inbox_check_path returns the throttle flag file path for inbox polling.
# Used by pre-edit.sh to avoid querying inbox on every edit (30s cache).
inbox_check_path() {
    echo "/tmp/interlock-pull-checked-${1}"
}

# negotiation_check_path returns the throttle flag for release-request inbox checks.
negotiation_check_path() {
    echo "/tmp/interlock-negotiate-checked-${1}"
}

# intermute_curl_fast wraps intermute_curl with --max-time 2 for hook-critical paths.
intermute_curl_fast() {
    local method="$1"; shift
    intermute_curl "$method" "$@" --max-time 2
}
