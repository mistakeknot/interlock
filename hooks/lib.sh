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
