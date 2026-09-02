# Lane report: release (agent sweep-release)

## Summary

56 repos in scope. 53 done (CHANGELOG.md created or updated, kimi.plugin.json version reconciled, README Install section present or intentionally left alone). 2 partial. 1 skipped. 0 repos skipped for reservation-conflict reasons, so the mandated single retry pass had nothing to retry.

## Repos done (53)

intermonk, intermux, intername, intermem, internext, intermap, interpath, interlore, interpeer, interloop, interphase, interlock (no-op -- already fully compliant), interplug, interline, interpub, interlens, interpulse, interleave, interrank, interlearn, interscout, interlab, interscribe, interknow, interkasten, intership, interject, intersight, interhelm, interskill, intergraph, interslack, interform, interspect, interflux, interstat, interfluence, intersynth, interdoc, intertest, interdev, intertrace, interdeploy, intertree, interdeep, intertrust, intercraft, interwatch, intercheck, tldr-swinton, interchart, tuivision, intercache.

## Repos partial (2)

- **intermix** -- CHANGELOG.md drafted but not committed. Repo has no README.md at all (only AGENTS.md/CLAUDE.md), so the mandated check-before-commit one-liner hard-fails on `grep ... README.md` (No such file or directory) and never prints PASS. kimi.plugin.json version already matched. Locked interpretations don't authorize creating a README from scratch, so nothing to do there.
- **intercept** -- same shape as intermix: no README.md exists, check-before-commit fails the same way, CHANGELOG.md drafted but left uncommitted.

## Repo skipped (1)

- **intersense** -- archived 2026-03-26 (see its ARCHIVED.md: superseded by interflux's LLM classification). No `.claude-plugin/` directory at all, so there is no plugin.json version to drive the CHANGELOG's `[<version>] - <date>` section or to compare kimi.plugin.json against. README already had an Installation heading, so nothing needed there. Did not draft a CHANGELOG.md since there was no version to anchor it to.

## Blocked / negotiated count

- Reservation conflicts encountered: 1 (interphase's README.md, held by sweep-ci). No `negotiate_release` call was needed or made, because the conflicted file (README.md) already satisfied the task's requirements and needed no edit from this lane -- the conflict was moot for this repo's actual work.
- Repos skipped and requeued for the mandated retry pass: 0. Nothing to retry.
- `fetch_inbox` was called once per repo (56 times) per protocol step 1; it never surfaced a real release request addressed to this agent in the entire run -- every message was a "commit" broadcast FYI from another lane. From roughly the 21st repo onward, the tool began erroring out ("exceeds maximum allowed tokens") instead of returning content, because the shared inbox has no read/ack mechanism and grows unbounded across all 5 agents' commits for the whole session.

## Three most annoying things about coordinating

1. **`fetch_inbox` has no acknowledgment/consumption model.** Every call returns the full accumulated history of every commit broadcast from every agent since session start -- there is no "mark as read" or delta-since-last-fetch. By repo ~20 the payload exceeded the tool's own token ceiling and started hard-erroring on every subsequent call for the rest of the run. A protocol step meant to be a cheap "any requests for me?" check turned into an ever-growing, eventually-broken liability that had to be called 56 times regardless.
2. **Conflict reporting doesn't know whether the conflicted file is actually needed.** `reserve_files` returning a CONFLICT entry for README.md (interphase) reads like a blocker, but in that case the file didn't need touching at all -- the protocol's "negotiate or move on" branch assumes every reserved pattern is load-bearing for the task, which isn't always true when the task itself is conditional (e.g., "only edit README if it lacks an Install heading").
3. **The task's own check script silently assumes files exist that sometimes don't.** `.claude-plugin/plugin.json` and `README.md` are both treated as always-present by release.md's one-liner and by the locked interpretations, but three repos in this list violated that assumption in two different ways (no README at all; no plugin.json at all because the repo is archived). Each case required judgment calls not spelled out anywhere in the brief, and in a 56-repo sweep that's not a one-off -- it's a real fraction of the fleet.

## Commits

56 lines in `progress-release.tsv`; each `done` row carries the commit SHA for that repo. `partial` and `skipped` rows carry `-` (no commit made).
