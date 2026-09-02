#!/usr/bin/env bash
# interlock-semantic-check.sh — tier-2 semantic conflict gate.
#
# Given a filename-level reservation conflict (tier-1 said "same file"), decide
# whether the two edits actually touch the same region/logic, using local
# embedding similarity (intersearch nomic-embed). This downgrades the common
# false positive — "same file, different function" — from a hard block to an
# allow/warn, which is what makes shared-FS coordination better than blunt
# filename locking.
#
# What it compares: THIS agent's edit region vs. the PEER's reservation reason.
# (The peer's actual in-flight edit content is not available at the reservation
# layer — only their stated reason. A future upgrade stores an embedding of the
# reserved region at reserve-time for true region-vs-region comparison; see
# docs/shared-fs-coordination.md §4.)
#
# Verdict (stdout, one word):
#   no-conflict   cosine < LOW  (0.70)  -> caller may allow
#   conflict      cosine > HIGH (0.90)  -> caller should block/warn
#   escalate      LOW..HIGH             -> ambiguous; caller decides (tier-3 later)
#   unknown       could not compute     -> caller MUST fail-open (treat as block-as-before)
#
# Usage: interlock-semantic-check.sh <edit_region_text> <peer_reason_text>
# Always exits 0; the verdict is on stdout. Fail-open by emitting "unknown".
#
# Env:
#   INTERLOCK_SEMANTIC_ENABLE   master switch (default 0 = off / shadow only)
#   INTERLOCK_SEMANTIC_LOW      low band edge (default 0.70)
#   INTERLOCK_SEMANTIC_HIGH     high band edge (default 0.90)
#   INTERLOCK_INTERSEARCH_DIR   path to intersearch repo; unset = semantic check disabled
#   INTERLOCK_SEMANTIC_TIMEOUT  seconds before giving up (default 3)
set -uo pipefail
trap 'echo unknown; exit 0' ERR

EDIT_REGION="${1:-}"
PEER_REASON="${2:-}"

# Need both sides to compare; missing either -> can't decide -> fail-open.
if [[ -z "$EDIT_REGION" || -z "$PEER_REASON" ]]; then
    echo unknown; exit 0
fi

LOW="${INTERLOCK_SEMANTIC_LOW:-0.70}"
HIGH="${INTERLOCK_SEMANTIC_HIGH:-0.90}"
ISDIR="${INTERLOCK_INTERSEARCH_DIR:-}"
TIMEOUT="${INTERLOCK_SEMANTIC_TIMEOUT:-3}"

# Need intersearch + uv present, else fail-open.
[[ -d "$ISDIR" ]] || { echo unknown; exit 0; }
command -v uv >/dev/null 2>&1 || { echo unknown; exit 0; }

# Pick a timeout wrapper if available (GNU coreutils `timeout` or `gtimeout`).
TO=()
if command -v timeout >/dev/null 2>&1; then TO=(timeout "$TIMEOUT")
elif command -v gtimeout >/dev/null 2>&1; then TO=(gtimeout "$TIMEOUT"); fi

# Compute cosine via intersearch's EmbeddingClient. Text is passed on argv to
# avoid quoting issues; the python reads sys.argv. Offline flags keep it from
# ever reaching the network (model is cached).
COS=$(
    cd "$ISDIR" 2>/dev/null || { echo ""; exit 0; }
    HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 TOKENIZERS_PARALLELISM=false \
    "${TO[@]}" uv run --quiet python - "$EDIT_REGION" "$PEER_REASON" <<'PY' 2>/dev/null
import sys
try:
    from intersearch.embeddings import EmbeddingClient
    a, b = sys.argv[1], sys.argv[2]
    c = EmbeddingClient()
    va, vb = c.embed(a), c.embed(b)
    print(f"{c.cosine_similarity(va, vb):.4f}")
except Exception:
    print("")
PY
)

# No number -> fail-open.
[[ "$COS" =~ ^[0-9]+\.[0-9]+$ ]] || { echo unknown; exit 0; }

# Band comparison via awk (portable float compare).
verdict=$(awk -v c="$COS" -v lo="$LOW" -v hi="$HIGH" 'BEGIN{
    if (c < lo) print "no-conflict";
    else if (c > hi) print "conflict";
    else print "escalate";
}')
[[ -n "$verdict" ]] || verdict="unknown"
echo "$verdict"
exit 0
