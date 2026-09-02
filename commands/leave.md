---
name: leave
description: Leave multi-agent coordination — release all reservations, deregister, remove onboarding flag
---

Leave multi-agent file coordination.

## Steps

1. **Check if currently joined:**
   ```bash
   if [ ! -f ~/.config/interlock/joined ]; then
       echo "Not currently joined. Nothing to do."
   fi
   ```
   If not joined, inform the user and stop.

2. **Release all reservations** by calling the `release_all` MCP tool or running `scripts/interlock-cleanup.sh`. If intermute is unreachable, proceed silently (reservations expire via heartbeat timeout).

3. **Remove flag files:**
   ```bash
   rm -f ~/.config/interlock/joined
   rm -f ~/.config/interlock/agent-name
   rm -f /tmp/interlock-agent-${CLAUDE_SESSION_ID}.json
   rm -f /tmp/interlock-connected-${CLAUDE_SESSION_ID}
   ```

4. **Confirm:** "Left coordination. Reservations released, agent deregistered. SessionStart hook will no longer auto-register."
