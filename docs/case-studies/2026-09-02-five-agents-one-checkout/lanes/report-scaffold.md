# Lane report: scaffold

## Repos done

All 56 repos in `order-scaffold.txt` reached the passing state defined by the brief's check command (LICENSE, CONTRIBUTING.md, SECURITY.md, CODE_OF_CONDUCT.md present; README.md references CONTRIBUTING.md where a README exists). Verified with a full re-run of the check command across all 56 at the end of the sweep: `PASS=56 FAIL=0`.

Two repos (`intercept`, `intermix`) have no README.md at all, so step 5 (append a Contributing section) legitimately does not apply to them — the other four files are present and correct. Every other repo has the full five-file set plus the README append.

`interflux` (list position 11) was skipped over during the initial top-to-bottom pass by my own error and picked up immediately after finishing through `interlore` (position 25), before continuing on to `intermap` (position 26). End state is identical to strict top-to-bottom order; only the sequencing differed.

`intercraft` was fully completed (commit `0642369`) but its progress-log line was accidentally omitted during the run; caught and added retroactively during the final audit, verified against the actual git history first.

## Repos skipped (partial/permanent)

- **intermix** — no README.md exists in the repo (confirmed at scan time and again at the end of the run). LICENSE/CONTRIBUTING.md/SECURITY.md/CODE_OF_CONDUCT.md are all present and correct; the README step has nothing to append to. Not a coordination failure — a real gap in the repo that's outside this lane's brief to fix (creating a README from scratch isn't in scope).
- **intercept** — same situation: no README.md, other four files present and correct.

No repo was skipped due to unresolved coordination conflicts — the three repos that hit a genuine reservation conflict on README.md (`interdoc`, `interfluence`, `interlock`) were all revisited after the conflicting lane finished and the README append landed successfully in all three (commits `997e1d6`, `255675b`, `094a0be`).

## Blocked / negotiated count

- **5 conflicts** on `reserve_files` calls, all on a README.md pattern, all against two other lanes (`sweep-ci`, `sweep-jargon-a`): `intercept`, `interdoc`, `interfluence`, `intermix`, `interlock`.
- **5 `negotiate_release` calls** across 4 of those repos (`interdoc` needed two attempts — see friction log). One came back `NOT_FOUND` against a reservation the conflict error had just reported to me seconds earlier; the rest returned `pending` and eventually resolved.
- Per protocol, every conflict was handled by skip-and-continue rather than blocking or polling; the three repos with real README content pending were revisited once at the end of the run.

## Three most annoying things about coordinating

1. **`fetch_inbox` has no way to see only what matters.** It returns the *entire* message history every single call — no read cursor, no "unread since last call," no filter for "release requests addressed to me." By repo 17 (~20 minutes in) a single call already exceeded the harness's own tool-result size limit and started dumping to a file I had to `python3 -c "json.loads(...)"` through by hand. The signal I actually needed (a release request naming me) was drowned in dozens of other lanes' commit broadcasts I have zero use for. I ended up deliberately throttling my own polling frequency to cope — a workaround the protocol didn't anticipate, though I logged it rather than doing it silently.
2. **`negotiate_release` doesn't reliably find the reservation `reserve_files` just told me about.** Same holder, same file, reported seconds apart — one call returns `pending`, the next returns `NOT_FOUND` under either the display name or the raw agent UUID. There's no third identifier to fall back to, so when it fails you're just stuck; I ended up parking the file and coming back later rather than looping retries, which the protocol at least explicitly sanctions.
3. **No visibility into whether a conflict is "in progress" or "stale/abandoned."** A `reserve_files` conflict error tells you who holds the lock and when it expires, but nothing about whether that agent is actively working on the file right now or stalled/dead. For a 20-minute TTL, guessing wrong either way costs real time — negotiating too eagerly annoys an agent mid-edit, waiting it out risks sitting on a lock that will never actually release early. (I always resolved this by just moving to the next repo per protocol, so it never blocked me, but the ambiguity itself is friction worth naming.)

## Note on LICENSE handling

Early in the run I nearly overwrote `intercache/LICENSE` (which already existed with a different copyright line, "2025 MK" vs. the template's "2026 mistakeknot") because I wrote the template unconditionally before checking presence. Caught via `git diff` before committing and reverted; every subsequent repo checked file existence first. Logged here rather than in the friction file since it was my own process error, not a tool surprise — but worth flagging in case other lanes hit the same trap on shared files like LICENSE that are easy to assume are "always missing."
