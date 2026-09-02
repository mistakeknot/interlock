# Two agents, one file, no collision

A runnable walk through the whole stack, from a clean clone, in about a minute:

- **intermute** holds the reservations and delivers the messages.
- **interlock** is what each agent talks to: reserve, check, negotiate, release.
- **intermux** watches the tmux sessions the agents live in and reports who is doing what.

Nothing here touches your real tmux server or a real intermute. The script starts private copies of both and removes them when it exits.

## Run it

```bash
git clone https://github.com/mistakeknot/interlock
cd interlock/examples/two-agent-demo
./demo.sh
```

Requirements: Go 1.24 or newer, tmux, python3. The script installs the three binaries into a temporary directory with `go install ...@latest`. To build from sibling checkouts of the three repos instead, run `DEMO_LOCAL=1 ./demo.sh`.

## What you will see

1. **alpha reserves** `src/**/*.go` with a reason and a 30-minute TTL.
2. **beta checks** the same pattern before editing and gets a conflict card naming alpha, the reason, the expiry, and the suggested next move.
3. **beta asks alpha to release**, without blocking. The request goes out as a message in a fresh negotiation thread.
4. **alpha reads its inbox** and finds the request: who wants what, why, and how urgently.
5. **alpha releases** in reply to the thread. The reservation is gone before the acknowledgement is sent.
6. **beta reserves** the pattern, which now succeeds.
7. **beta lists the agents** intermute knows about.
8. **beta releases** everything it holds.
9. **intermux** lists the two tmux sessions, classifies one as active and one as idle from their screen contents, and answers "who is editing `src/`" (nobody has touched a file in this demo, so the answer is an empty list).

`demo.py` is a plain MCP client over stdio (JSON-RPC, standard library only). Read it to see the exact tool calls and arguments; every step is one `tools/call`.

## Where to go next

- The negotiation rules, timeouts, and escalation path: [`docs/negotiation-protocol.md`](../../docs/negotiation-protocol.md).
- Installing interlock into your own MCP client, with or without Claude Code: [`docs/install.md`](../../docs/install.md).
