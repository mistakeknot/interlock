---
name: interlock-status
description: Show multi-agent coordination status — agents, reservations, pending negotiations
---

# Multi-Agent Coordination Status

Gather and display coordination state from the interlock MCP server.

## Steps

1. **Active agents** — call `list_agents` MCP tool. Show count and names.

2. **Reservations** — call `my_reservations` MCP tool. Show file patterns, TTLs, and reasons.

3. **Pending negotiations** — call `fetch_inbox` MCP tool. Check the response for:
   - `negotiation_timeouts` array — pending release requests that have exceeded their timeout
   - Any `release-request` messages without corresponding `release-ack` responses

4. **Display** — format as a compact status report:

```
Interlock Status:
  Agents: <count> active (<names>)
  Reservations: <count> files held
  Negotiations: <count> pending (<count> timed out)

Active Reservations:
  <file_pattern> — <reason> (TTL: <remaining>)

Pending Negotiations:
  <file> — requested by <requester>, held by <holder> (<urgency>, <age>m ago)
```

If no agents are registered, show: `Not active — run /interlock:join to register.`

If the MCP server is unavailable (tool calls fail), show: `FAIL — interlock MCP server not responding. Check intermute service.`
