#!/usr/bin/env bash
# launch.sh <lane> [stranger|maintainer]  — start one executor in a tmux session on the default server.
set -euo pipefail
LANE="${1:?lane}"; MODE="${2:-stranger}"
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SESS="sweep-${LANE}"
NAME="sweep-${LANE}"
IL=<checkout>/interverse/interlock
IM=<checkout>/interverse/intermux
if [[ "$MODE" == "stranger" ]]; then
  XPATH="$D/shim:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:$(go env GOPATH)/bin"
else
  XPATH="$D/shim:$PATH"
fi
mkdir -p "$D/intermux-mappings"; chmod 700 "$D/intermux-mappings"
tmux kill-session -t "$SESS" 2>/dev/null || true
tmux new-session -d -s "$SESS" -c <checkout> -x 200 -y 50 \
  -e "PATH=$XPATH" \
  -e "INTERLOCK_JOIN_FLAG=$D/joined" \
  -e "INTERLOCK_AGENT_NAME=$NAME" \
  -e "INTERMUTE_URL=http://127.0.0.1:7338" \
  -e "INTERMUTE_PROJECT=Sylveste" \
  -e "INTERLOCK_PROJECT=Sylveste" \
  -e "INTERMUX_MAPPING_DIR=$D/intermux-mappings" \
  -e "SWEEP_LANE=$LANE" \
  -e "SWEEP_D=$D" ${EXTRA_ENV:+-e "$EXTRA_ENV"}
tmux set-option -w -t "$SESS" allow-set-title off
tmux set-option -w -t "$SESS" automatic-rename off
tmux rename-window -t "$SESS" "$NAME"
tmux select-pane -t "$SESS" -T "$NAME"
tmux send-keys -t "$SESS" "claude --dangerously-skip-permissions --model sonnet --settings $D/executor-settings.json --plugin-dir $IL --plugin-dir $IM" Enter
# answer the first-run trust dialog if it appears, then wait for the prompt
for i in $(seq 1 90); do
  P="$(tmux capture-pane -p -t "$SESS")"
  if grep -q 'trust this folder' <<<"$P"; then tmux send-keys -t "$SESS" Down; sleep 1; tmux send-keys -t "$SESS" Enter; sleep 3; continue; fi
  if grep -q -E '^❯ (Try "|$)|bypass permissions on' <<<"$P"; then break; fi
  sleep 1
done
sleep 2
tmux send-keys -t "$SESS" "Read $D/briefs/$LANE.md and follow it exactly. Start now."
sleep 2
tmux send-keys -t "$SESS" Enter
sleep 3
if tmux capture-pane -p -t "$SESS" | grep -q "^❯ Read "; then tmux send-keys -t "$SESS" Enter; fi
echo "launched $SESS (mode=$MODE)"
