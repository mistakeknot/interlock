#!/usr/bin/env bash
D="$(cd "$(dirname "$0")" && pwd)"
cd <checkout>
XPATH="$D/shim:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
run() {
  printf '{"session_id":"782217ff-59cc-433f-9254-fb716b2abb0c","cwd":"<checkout>","tool_name":"Edit","tool_input":{"file_path":"<checkout>/interverse/interlore/%s","old_string":"y","new_string":"z"}}' "$1" \
  | env -i HOME="$HOME" PATH="$XPATH" INTERMUTE_URL=http://127.0.0.1:7338 INTERMUTE_AGENT_ID=5c38c20c-4c13-4438-977e-d39b5655e078 INTERMUTE_AGENT_NAME=sweep-probe INTERMUTE_PROJECT=Sylveste INTERLOCK_PROJECT_ROOT=<checkout> CLAUDE_SESSION_ID=782217ff-59cc-433f-9254-fb716b2abb0c INTERLOCK_JOIN_FLAG="$D/joined" bash interverse/interlock/hooks/pre-edit.sh
  echo " [exit $?]"
}
echo '---- foreign hold (blocker): expect block naming blocker'; run PROBE.md
echo '---- same-name hold (twin named sweep-probe): expect no output'; run OTHER.md
echo "---- same-name fresh path: expect no output"; run FOURTH.md
echo '---- unheld file: expect no output, then an auto-reserve row'; run FREE.md
sqlite3 "$D/intermute.db" "select substr(agent_id,1,8), path_pattern, reason from file_reservations where path_pattern like '%FREE.md' and released_at is null"
