# CI lane (sweep-ci) — final report

## Repos done: 54/56 fully complete, 2/56 partial (ci.yml done, README badge not applicable)

All 56 repos in `order-ci.txt` got a `.github/workflows/ci.yml` with an ubuntu-latest +
macos-latest matrix (new file from the appropriate template, or an existing workflow
patched in place — matrix added, every existing step kept, `if: runner.os == 'Linux'`
added to any harden-runner step, other workflow files in the same repo left untouched).

54 of 56 also got the README CI badge line inserted directly after the first `# `
title. Two negotiated conflicts (interdoc, interscout) and one negotiated-but-blocked-
until-retry conflict (intermux) all resolved successfully later in the run once the
holder released — I went back and finished those badges before writing this report.

**intercept** and **intermix** are the two genuine partials: neither repo has a
README.md at all (confirmed again at report time — still absent). There is no first
`# ` title line to insert after, so the badge step in the brief does not apply. Both
got their `ci.yml` written/committed normally. If a README.md shows up later (likely
from the scaffold lane, whose job includes README creation for repos that lack one),
the badge line still needs to be added — that's a natural follow-up for whoever
notices, or for a future run of this lane.

### Check-selection outcomes (for anyone auditing exit codes)

- **go chain never fully passed** on any go/go+py repo this lane touched (interlab,
  intermix, intermap, interlock, intermux). interlock and intermux were clean;
  interlab, intermix, and intermap all failed on `gofmt -l .` (unformatted files) even
  though `go build`/`go vet`/`go test` passed — pre-existing formatting debt, not
  something introduced by this sweep. Per the brief's fallback rule, interlab used its
  tests/ dir + pytest instead (ci-py.yml); intermix had no tests/ dir so fell all the
  way to ci-sh.yml; intermap and interlock already had committed go-based ci.yml files,
  which the brief says to keep as-is (patched for matrix only) regardless of what the
  local check does.
- **~9 of 56 py repos had a pre-existing failing/erroring pytest suite**: interstat,
  interkasten, interject, interdeep, interwatch, intercache, interspect, intersense,
  and tldr-swinton. Causes ranged from stale structural-count assertions (skills/
  commands/agents counts drifted from the fixture) to `ModuleNotFoundError` (package
  layout doesn't match what the test imports) to a hard `sys.exit(1)` at import time
  in tldr-swinton's eval harness. None of these looked related to anything this sweep
  touched; all fell back cleanly to the sh-check per the brief's algorithm and got
  ci-sh.yml.
- **6 repos already had a `.github/workflows/ci.yml`**: interphase, interchart,
  intermap, interlock, interflux, tldr-swinton, plus intermux (7 total). Six of those
  needed the matrix patch; interlock and intermux already had it (interlock is the
  coordination tool's own repo and was already fully compliant — nothing to do but
  add the README badge).

## Repos skipped: none

Every repo in the 56-line order list got at least a `ci.yml`. Nothing was skipped
outright.

## Blocked / negotiated: 4 conflicts, all resolved

1. **interdoc** README.md — held by sweep-jargon-a. Negotiated (pending), moved on,
   came back ~13 repos later once fetch_inbox showed a release-ack; badge added then.
2. **interscout** README.md — held by sweep-release. Negotiated (pending), moved on,
   retried successfully near the very end of the run (release must have happened
   without an ack ever surfacing in my inbox reads — see friction below).
3. **interflux** README.md — reserve_files reported a CONFLICT held by sweep-release,
   but by the time I called negotiate_release seconds later it errored NOT_FOUND
   because the holder had already released. Immediate retry of reserve_files
   succeeded with no wait.
4. **intermux** README.md — held by sweep-scaffold. Negotiated (pending), moved on,
   retried successfully near the end of the run, same pattern as interscout.

So: 4 negotiations opened, 0 explicit release-acks I could act on before retrying
(1 ack did eventually surface for interdoc), 4/4 eventually resolved by just retrying
`reserve_files` later rather than waiting on the negotiation thread.

## The three most annoying things about coordinating

1. **fetch_inbox has no working pagination and hard-fails at scale.** Every call
   returned the *entire* accumulated backlog of broadcast "commit" notifications from
   all 5 lanes, not just what's new since the last call. `next_cursor` was always
   empty. The payload grew from 4 messages at session start to 35 messages nine repos
   later to a **hard tool-call failure** ("exceeds maximum allowed tokens") by the end
   of the run — not a graceful degradation, a dead end that dumped the raw JSON to a
   file and told me to grep it. I had to progressively cut my own fetch_inbox cadence
   (every repo → every 5 → every 10 → basically "only when I actually need to check
   something") just to keep the sweep completable, which is a workaround the protocol
   explicitly discourages doing silently — so I wrote it down each time instead.
2. **A CONFLICT reported by reserve_files can already be stale by the time you act on
   it.** interflux: reserve_files said README.md was held by sweep-release with an
   expiry 20 minutes out; negotiate_release on that exact holder, called seconds
   later, errored NOT_FOUND because the reservation was already gone. The two calls
   aren't atomic and there's no way to know that from the first response — you just
   have to retry reserve_files and hope.
3. **Negotiation threads never resolved back to me.** Of 4 negotiate_release calls,
   only 1 (interdoc) ever produced a visible release-ack in a later fetch_inbox read;
   the other 3 (interscout, intermux, and interflux's stale case) I only recovered by
   blindly retrying reserve_files much later, with no signal telling me it was safe to
   do so. The protocol's "come back later" instruction works, but there's no
   notification to tell you *when* later actually is — you're polling by re-attempting
   the reservation, which is exactly the kind of loop the protocol tells you not to do
   with negotiate_release's `wait_seconds`.

Also worth a footnote: the MCP tool identity didn't match the shell-visible identity
(`INTERMUTE_AGENT_ID` env var named one agent_id, but every reserve_files call actually
executed as a *different* registered "sweep-ci" agent_id) — harmless here since I never
needed to reconcile the two, but confusing if you go looking for "my" reservations by
the env var's ID and find nothing.
