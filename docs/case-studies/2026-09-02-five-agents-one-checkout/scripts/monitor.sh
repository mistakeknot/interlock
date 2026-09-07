#!/usr/bin/env bash
# Summarize the run every 5 minutes; exit when all five lanes are done or after 5 hours.
D="$(cd "$(dirname "$0")" && pwd)"
LANES="scaffold jargon-a jargon-b release ci"
for i in $(seq 1 60); do
  echo "=== $(date '+%H:%M:%S')"
  done_n=0
  for l in $LANES; do
    p=$(wc -l < "$D/progress-$l.tsv" 2>/dev/null | tr -d ' '); p=${p:-0}
    f=$(grep -c '^## ' "$D/friction-$l.md" 2>/dev/null || echo 0)
    d=""; [ -f "$D/done-$l" ] && { d="DONE"; done_n=$((done_n+1)); }
    printf '  %-10s repos=%-3s friction=%-3s %s\n' "$l" "$p" "$f" "$d"
  done
  echo "  reservations: $(sqlite3 "$D/intermute.db" 'select count(*) from file_reservations') total, $(sqlite3 "$D/intermute.db" 'select count(distinct agent_id) from file_reservations') agents; messages: $(sqlite3 "$D/intermute.db" 'select count(*) from messages') ($(sqlite3 "$D/intermute.db" "select count(*) from messages where thread_id like 'negotiate-%'") in negotiate threads)"
  echo "  commits on job branches: $(for r in $(cut -f1 "$D/../job-repos.tsv"); do git -C "$r" rev-list --count main..sweep/2026-09-02 2>/dev/null || git -C "$r" rev-list --count master..sweep/2026-09-02 2>/dev/null; done | awk '{s+=$1} END {print s}')"
  [ "$done_n" -eq 5 ] && { echo "ALL LANES DONE"; exit 0; }
  sleep 300
done
echo "monitor timed out"
