---
name: status
description: Show active agents, their reservations, heartbeat status, and human-readable names
---

Show the current state of multi-agent coordination.

## Steps

1. **Check intermute health** (Unix socket first, then TCP). If unreachable, report "intermute service not running" and suggest `/interlock:setup`.

2. **Fetch agents** via `GET /api/agents?project=<project>` (derive project from git root or cwd).

3. **Fetch reservations** via `GET /api/reservations?project=<project>`.

4. **Display as formatted tables:**

   ```
   Interlock Status
   ────────────────────────────────────
   Agents:
   | Name             | Agent ID  | Reservations | Status |
   |------------------|-----------|--------------|--------|

   Reservations:
   | Pattern          | Held By          | Reason       | Expires   |
   |------------------|------------------|--------------|-----------|
   ────────────────────────────────────
   ```

5. **Show own status** if currently joined:
   ```bash
   if [ -f ~/.config/clavain/intermute-joined ]; then
       MY_NAME=$(cat ~/.config/clavain/intermute-agent-name 2>/dev/null || echo "unknown")
       echo "You are: $MY_NAME"
   else
       echo "You are not joined. Run /interlock:join to participate."
   fi
   ```
