# fd-semver-coordination — Ship Plan Findings

**Reviewer:** fd-semver-coordination (release engineering lens)
**Target:** `docs/research/flux-review/interlock-ship-plan-review/2026-04-30-ship-plan.md`
**Date:** 2026-04-30
**Project:** interlock plugin v0.2.13 → v0.2.14/v0.2.15/v0.2.16
**Lens:** Version contracts, stuck-at-intermediate user states, semver truthfulness

---

## Summary

Three findings. Two P1 (stuck-at-intermediate user gets no machine-readable signal of unfixed P0 bugs; v0.2.16 design-note minor bump misleads auto-update consumers). One P2 (skip-version compatibility — v0.2.13 → v0.2.16 leapfrog has no tested composition story). One P3 (escape hatch with no removal condition).

The plan's central semver problem: each version's plugin.json bumps the minor field, but the **stuck-at-intermediate experience is not encoded into the artifact** — only into prose in this ship plan. A consumer reading the plugin manifest cannot distinguish "v0.2.14 is a complete fix" from "v0.2.14 is a partial fix with three known P0s still live." That delta is load-bearing for users who auto-update, then stop because nothing breaks.

---

## P1: v0.2.14 ships without machine-readable signal of unfixed P0 bugs

**Location:** Ship plan §"v0.2.14 — Lifecycle hotfix" (lines 32-46), specifically the "Effect on a user who upgrades to v0.2.14 only" subsection (lines 42-46), and `.claude-plugin/plugin.json` bump line.

**Failure scenario:** sma's marketplace publishes v0.2.14 with the orphan-cleanup fix. A downstream user on `interagency-marketplace` auto-updates from 0.2.13 to 0.2.14. Their cleanup gap is fixed; they observe no new orphan accumulation; they assume the plugin is healthy. Their CI runs concurrent commits in a multi-session workflow (a pattern the plugin is designed to support per AGENTS.md). CF-1 silently loses work. CF-3 corrupts staged state when a hook cd's into a nested .git dir. The user has no signal that these conditions are unsafe — the plugin advertises version 0.2.14, no prose, no warning, no `known_issues` field. They wake up at 3 AM when a colleague's commit vanishes.

The plan acknowledges (line 44-45): "Nested-repo bug NOT fixed... Concurrent-commit silent-data-loss NOT fixed." But that acknowledgment lives only in this proposal, not in any artifact the user receives.

**Concrete gap:**
- `.claude-plugin/plugin.json` has no `known_issues` field, no `compat` field, no `disable_concurrent_commits` advisory.
- No CHANGELOG entry was specified in the file list (the plan's Files Touched list at lines 33-40 omits CHANGELOG.md or RELEASE_NOTES.md).
- No runtime stderr warning is proposed in v0.2.14 (the plan's open question F at line 63 raises this but defers it as "TBD").

**Smallest viable fix:** Add to the v0.2.14 file list:
1. `CHANGELOG.md` — entry that lists CF-2 as fixed and CF-1/CF-3/CF-4/CF-5 as known unfixed, with link to GitHub issues.
2. `.claude-plugin/plugin.json` — add `"known_issues": ["#N concurrent-commit-loss", "#M nested-repo-cwd"]` (custom field; ignored by Claude Code loader but inspectable by tooling and humans who read the manifest).
3. Optional: stderr nag on hook installation when concurrent commits are detected in the session log.

**Question for sma:** Does the marketplace surface CHANGELOG.md to consumers, or only plugin.json? If only plugin.json, the `known_issues` field is the only viable signal.

---

## P1: v0.2.16 minor-version bump for design-note-only change misleads auto-update consumers

**Location:** Ship plan §"v0.2.16 — Reconciliation correctness (DESIGN-NOTE-ONLY)" (lines 57-63), specifically `.claude-plugin/plugin.json — bump to 0.2.16` (line 61).

**Failure scenario:** sma publishes v0.2.16 to the marketplace. Downstream consumers auto-update. Their plugin.json now reads 0.2.16. A consumer who follows semver conventions reads "minor bump from 0.2.15" as "new functionality, no breaking changes." They re-enable a concurrent-commit workflow they previously disabled (perhaps based on the v0.2.14 known_issues warning, if implemented per finding above). The fix has not actually shipped — only an xfail test and a design note. The data-loss bug fires.

The version number is functioning as a misinformation channel: it says "this version is newer and presumably better than v0.2.15" when the runtime behavior is identical to v0.2.15 plus a non-executing test.

**The plan's stated rationale (line 58):** "NOT a fix. Adds: design note... failing test (xfail)... bump to 0.2.16."

**Why this fails the semver contract:** Per semver.org, MINOR is for "functionality added in a backwards compatible manner." A design note is not functionality. An xfail test is not functionality (it doesn't run; it documents an expected failure). The correct version action for a docs-only or test-skeleton change is one of:
- No version bump — commit on existing v0.2.15 tag, push docs to repo.
- PATCH bump (0.2.15 → 0.2.15.1 or 0.2.16 if the project uses three-segment-only) for non-functional metadata changes.
- Pre-release tag (0.2.16-rc.0 or 0.2.16-design) signaling that this is not a runtime release.

**Smallest viable fix:**
- **Option A (preferred):** Drop v0.2.16 entirely. Commit the design note + xfail to main on top of v0.2.15 tag. No new version. Update the project README or AGENTS.md to point to the design doc.
- **Option B:** Use a pre-release suffix: `0.2.16-design.0`. semver-aware tooling will not auto-update from 0.2.15 to a pre-release.
- **Option C:** Skip the version bump in plugin.json but still cut a CHANGELOG entry under "Unreleased."

**Question for sma:** Does the interlock release pipeline (interpub:release, per available skills) require a plugin.json version bump for any commit, or does it support commits without version bumps? If the pipeline forces bumps, that's the actual constraint to work around.

---

## P2: No documented skip-version compatibility — v0.2.13 → v0.2.16 leapfrog is untested

**Location:** Ship plan §"Proposed ship sequence" (lines 30-63). Each version is described independently; no section verifies that v0.2.16 alone (without v0.2.14 or v0.2.15) is a coherent state.

**Failure scenario:** A new consumer installs interlock fresh after v0.2.16 is published. The marketplace gives them v0.2.16. They never had v0.2.13 orphans (clean install), so the orphan-sweeper from v0.2.14 is a no-op for them. They get the v0.2.15 wrapper hardening because v0.2.16 inherits it (assumption, not stated). They get the design note, which is fine. Net: this user is in a coherent state. ✓

But: a consumer pinned to v0.2.13 (perhaps because v0.2.14 was yanked, or because they pin majors) sees v0.2.16 published, decides to skip-upgrade. The marketplace installs v0.2.16. This user has accumulated v0.2.13-era orphans. The sweeper from v0.2.14 is included in v0.2.16's code — but does it run automatically on first hook invocation, or only on user-initiated `interlock-orphan-sweep`? The plan does not specify the sweeper's trigger model.

**The plan's silent assumption:** v0.2.14 → v0.2.15 → v0.2.16 are adopted as a sequence. Real marketplace consumers may skip versions.

**Concrete gaps:**
- No section "Skip-upgrade compatibility: v0.2.13 → v0.2.16" exists.
- The orphan sweeper trigger is unspecified (line 38: "new TTL sweeper" but no description of when/how it fires).
- v0.2.15's wrapper changes (line 50-54) are described as "layers on top of" v0.2.14's atomic init (line 37), but composition is asserted, not verified.

**Smallest viable fix:** Add a "Skip-upgrade matrix" subsection to the plan listing each pair (0.2.13→0.2.14, 0.2.13→0.2.15, 0.2.13→0.2.16, 0.2.14→0.2.16) and naming whether the orphan-sweeper auto-runs, whether the wrapper hardening is safe without prior cleanup, and whether the design-note-only version is reachable via skip.

---

## P3: INTERLOCK_DISABLE_INDEX_ISOLATION=1 escape hatch has no removal condition

**Location:** Ship plan §"v0.2.14 — Lifecycle hotfix", line 37: `INTERLOCK_DISABLE_INDEX_ISOLATION=1` escape hatch at top of file."

**Concern:** Escape hatches added during a stabilization push tend to calcify. By v0.2.18 nobody remembers what bug it was hatching, but removing it is now a breaking change.

**Smallest viable fix:** Add to the plan's v0.2.14 file list:
- A comment in `hooks/session-start.sh` next to the escape-hatch check that names the deprecation condition, e.g., "Remove this flag once CF-1, CF-3, CF-4 are all resolved (target: v0.3.0). If you are still using this flag in v0.3.0, file an issue."
- A test that asserts the flag exists and emits a stderr warning ("This flag is a stabilization-era escape hatch; its presence is non-load-bearing"), so the flag is observable rather than silently honored.

This is P3 because it does not affect correctness of the proposed releases; it affects long-term maintenance hygiene.

---

## What I did NOT review (per agent boundaries)

- Branch-only-push and PR-against-self mechanics — fd-irreversible-action-discipline.
- Cache-mirror behavior when marketplace ships a different v0.2.14 — fd-cache-mirror-drift.
- Whether the 6 tests cover the actual failure modes — fd-test-coverage-gaps.
- Whether the sweeper is safe to run on 638 elf-revel orphans — fd-repair-safety-protocol.

---

## Decision summary

The 3-version sequence is *correct in shape* — fixing infrastructure first (v0.2.14), then hardening (v0.2.15), then documenting unfixed correctness gaps (v0.2.16) is a defensible stabilization arc. The problems are:

1. **The intermediate versions are not annotated as intermediate** — a stuck-at-v0.2.14 user has no in-artifact signal that they are stuck.
2. **The v0.2.16 minor bump overclaims** — semver consumers will read "new functionality" where there is none.
3. **Skip-upgrade composition is asserted without verification** — the plan assumes linear adoption.

If the answer to question A ("is the sequencing right?") is "yes, the shape is right but the labeling is wrong," then the smallest set of changes is: add `known_issues` to plugin.json in v0.2.14, drop the v0.2.16 version bump (or pre-release tag it), and write the skip-upgrade matrix into the plan before execution.
