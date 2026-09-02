# Five agents, one checkout: what the swarm stack looked like from the inside

Date: 2026-09-02. Raw evidence beside this file in `2026-09-02-five-agents-one-checkout/`: intermute's agents, reservations, and messages tables as CSV, the five lanes' friction logs, progress files, and reports, the probe reports, the briefs, the client recordings, and the scripts that ran it all. Local paths are replaced with `<checkout>` and `<scratch>`. The per-minute agent and intermux snapshots (7 MB of JSON lines) are omitted.

## The job

Fifty-six public plugin repositories under one working tree each needed the same four things: contributing and security files, a jargon scrub of user-facing docs, a changelog and install section, and a CI workflow on both Linux and macOS. Instead of one agent per repository, five agents each took one concern across all fifty-six, in different orders, so every repository's README would be edited by four of them and collisions were certain.

Each agent was an interactive Claude Code session (Sonnet) in its own tmux session, loading interlock v0.2.19 and intermux v0.1.14 from their tagged checkouts, with `ic` removed from PATH so the hooks took the path a stranger's machine takes. intermute v0.1.0 ran locally. The agents were told to reserve before editing, negotiate on conflict and move on, answer release requests, commit with pathspecs, and write down every surprise.

## What the stack did

The lanes ran from 13:17 to 14:03 PDT. In that hour:

| | |
|---|---|
| Repositories swept | 56 |
| Lane commits on job branches | 207 |
| Reservations made | 788, by 10 agent ids (5 lanes, each registered twice) |
| Reservations made by the pre-edit hook | 6 |
| Conflicts reported by `reserve_files` | 13 across the five lanes' logs |
| Release requests sent | 9 |
| Release requests answered | 3 |
| Commit notifications delivered | 203 |
| Acceptance after the run | scaffold 56/56, CI 56/56, release 55/56, jargon 29/56 by the strict line |

Every conflict resolved. Not one resolved through the negotiation protocol working end to end as designed: the three acks that were sent all reported zero reservations released, because the holder had already released after committing, and the six unanswered requests either reached a holder who had also already finished or reached the wrong half of a split identity. Agents got their files by coming back later and trying again. That is the honest headline: the reservation layer prevented every collision, and the messaging layer around it did not carry its weight.

The jargon line's 29/56 is the strict grep over everything under `docs/`. The residue is inside internal artifact directories (research notes, brainstorms, old plans, generated `roadmap.json`, diagrams), plus a handful of terms that are the product itself: interlore proposes changes to a file called PHILOSOPHY.md, and intername ships a naming theme called Demarch. Whether those artifact directories belong in public repositories at all is a separate decision. The user-facing files are clean.

## What broke, in the order we found it

1. **The pre-edit hook never blocked anything** (interlock #3). A jq precedence slip made every conflict check error silently, so the hook auto-reserved over other agents' exclusive holds and let edits through. Found with a probe before the lanes launched, fixed, and reinstalled first. Nobody had noticed because on the maintainer's machine the hook takes a different path entirely, through intercore, whenever `ic` is on PATH (#7).
2. **One session was two agents** (interlock #4). The session-start hook and the MCP server register separately, and a four-month-old legacy name file was naming the hook's half after a project nobody had touched since May. During the run this surfaced as `negotiate_release` failing on a file the conflict card said was held: the holder's MCP row had released, its hook row still held a 15-minute auto-reserve, and the name resolved to both. An interim rule shipped (a hold by an agent with your own name is yours); the real fix is one identity per session.
3. **Commit notifications went nowhere** (interlock #5). The post-commit hook read `.id` where the API returns `agent_id`, addressed its notification to a list of empty strings, and its auto-release sent a DELETE without the agent header intermute requires. Fixed.
4. **Every commit printed JSON into git's output** (interlock #6). Fixed.
5. **The inbox never paged** (interlock #8). The client sent a cursor parameter intermute does not read and read a field intermute does not set. Once #5 was fixed and commit broadcasts started arriving, every lane's inbox grew with every other lane's commits until the tool's result outgrew the client's output limit; three of five lanes cut their polling on purpose and said so. The hook's own inbox poll hit a route that returns 405. Fixed; the fan-out design question stays open.
6. **The hook watches the wrong tools** (interlock #9). It matches Edit and Write. Autonomous agents write through Bash. Six hook reservations against 782 from the tools.
7. **intermute accepts holds for agents that do not exist** (intermute #7). A mistyped id produced a reservation nobody could negotiate with.
8. **intermute stacks a holder's own reservations** (intermute #8). The pre-edit hook reserves the file it is about to edit; four edits of one file in three minutes gave one agent four simultaneous exclusive holds on it, each counting against the per-agent cap and each needing its own release.
9. **intermux could not see the swarm** (intermux #7, #8). Sessions not named the maintainer's way are filtered out even when a mapping file identifies them, and status is read from the last screen line, which in every current client is a status bar, so all 29 sessions it did see were "unknown".

## What the agents said

Each lane was asked for the three most annoying things about coordinating. The answers agree.

> `fetch_inbox` has no way to see only what matters. It returns the entire message history every single call. By repo 17 a single call already exceeded the harness's own tool-result limit. (scaffold)

> `negotiate_release` doesn't reliably find the reservation `reserve_files` just told me about. Same holder, same file, reported seconds apart: one call returns `pending`, the next returns `NOT_FOUND` under either the display name or the raw agent UUID. (scaffold)

> Conflicts can be stale by the time the protocol tells you to act on them. The reservation had already cleared in the few hundred milliseconds it took to make that call. (jargon-a)

> Reservation conflicts and negotiation are one-way and silent. `negotiate_release` returns `pending` and that's it. "Move on and come back later" works, but every conflict costs a full extra repo-cycle's worth of bookkeeping with no signal in between. (jargon-b)

> No visibility into whether a conflict is "in progress" or "stale/abandoned." A conflict tells you who holds the lock and when it expires, but nothing about whether that agent is actively working on the file right now. (scaffold)

> Conflict reporting doesn't know whether the conflicted file is actually needed. A CONFLICT on README.md reads like a blocker, but in that case the file didn't need touching at all. (release)

## What shipped

All nine are fixed in interlock 0.2.20 (#3 to #9), intermute 0.1.1 (#7, #8), and intermux 0.1.15 (#7, #8), each with a test written from the failing case above. Two more defects fell out of writing those tests rather than out of the run: the registration script's identity-adoption loop miscounted on macOS, where `grep -c` prints `0` and exits 1, and intermux dropped a mapping file loaded before the session's first scan.

## What this means for the stack

- **Reservations work.** Twelve hundred edits by five agents on shared files, no overwrite, no merge conflict, no lost work. Pathspec commits on one branch per repo held up.
- **Identity is the next fix, not a nicety.** Three of the nine defects and most of the unanswered requests trace to one session being two agents.
- **Messaging needs an audience model.** Commit notifications to everyone, an inbox that never advances, and release requests that arrive after the holder has moved on all point the same way: messages should go to agents whose reservations overlap, inboxes should page, and a request should carry the reservation id it is about.
- **Liveness beats TTL.** Every agent asked for the same thing: is the holder still working on this file. intermux exists to answer that and could not see the sessions.
- **The hook is not the enforcement layer for autonomous agents.** The MCP tools and the pre-commit hook are. The docs should say so.

## Reproduce

`2026-09-02-five-agents-one-checkout/scripts/launch.sh <lane>` starts one executor with the stranger environment; `scripts/observe.py` logs the evidence; `briefs/` are what the agents read; `scripts/validate.sh` is the acceptance. The client demos are in interlock under `examples/two-agent-demo/clients/`, and the two-agent demo runs from a clean clone.
