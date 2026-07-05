# fd-irreversible-action-discipline — Ship Plan Findings

**Reviewer:** fd-irreversible-action-discipline (repository governance / single-maintainer post-mortem lens)
**Target:** `docs/research/flux-review/interlock-ship-plan-review/2026-04-30-ship-plan.md`
**Date:** 2026-04-30
**Project:** interlock @ ffe8129 (v0.2.13)
**Lens:** Mechanical enforcement of branch-only / no-merge / no-publish; PR-against-self ceremony; repair-step gating

---

## Summary

Four findings.
- One **P1**: 3-version commits bundled on a single branch couples review of v0.2.14 with v0.2.15/v0.2.16 — partial acceptance is not mechanically possible without amend/rebase.
- One **P1**: Cache-mirror step (a local-machine destructive write to `~/.claude/plugins/cache/`) lacks an explicit ask-before-proceed gate — Sylveste/CLAUDE.md requires it.
- One **P2**: Orphan-repair step (638 elf-revel files) lacks a mid-sequence checkpoint between dry-run and destructive operations.
- One **P3**: Repository CI workflows verified — no auto-merge or auto-publish on push (good news), but the plan does not document this verification.

I confirmed by inspection: `.github/workflows/ci.yml` runs only build/vet/test on push to main and pull-request; `.github/workflows/secret-scan.yml` runs gitleaks. Neither workflow triggers a marketplace publish, an auto-merge, or any tag-based release. **The "push branch only, do not push main" assumption is mechanically safe in this repository as of `ffe8129`.** This is good — but the plan should record the verification.

---

## P1: Single-branch 3-commit bundle couples review of v0.2.14 with v0.2.15/v0.2.16

**Location:** Ship plan §"Execution mechanics" (lines 65-69):
> Branch: `fix/git-index-isolation-completeness` off `main` (currently @ `ffe8129`).
> Commits: 3 sequential, each on its own logical area.
> Push: BRANCH ONLY. Do not push main. Do not auto-merge. Do not publish.
> PR description drafted in chat for sma to review.

**Failure scenario:** sma reviews the PR. The PR contains three commits — v0.2.14 (lifecycle hotfix), v0.2.15 (wrapper hardening), v0.2.16 (design-note + xfail test + version bump). sma is comfortable with v0.2.14 and v0.2.16 but wants to push back on v0.2.15's `env -u GIT_DIR -u GIT_WORK_TREE` strip-then-rev-parse pattern (perhaps because it disrupts a workflow where GIT_DIR is intentionally set, or because of zsh `export -f` semantics — line 53). 

What's the path forward? With a single branch:
- sma must `git rebase -i` to drop or amend the v0.2.15 commit. This means rewriting v0.2.16 on top of v0.2.14. Not impossible, but a manual ceremony in a context where the maintainer is already triaging issues.
- If sma instead merges only the first commit (v0.2.14) by squashing or cherry-picking, v0.2.15's wrapper changes are still on the branch but unmerged — and v0.2.16's design note (which references v0.2.15's wrapper changes by line number? unclear) is now disconnected.
- Worst case: sma decides "ship all or none" because the rebase is friction, and accepts a less-than-ideal v0.2.15 to avoid blocking v0.2.14.

**Why P1:** This couples three independent decisions into one mechanical commitment. The Sylveste/CLAUDE.md doctrine "irreversible actions ask before proceeding" is satisfied at the merge moment, but the merge action is overloaded — it's three decisions, not one.

**Concrete gap:** The plan's commits are described as "sequential, each on its own logical area" (line 67). That's correct in a code-organization sense but wrong in a review-decision sense. A reviewer wants three separate accept/reject points, not one accept/reject for a sequence.

**Smallest viable fix:** Three options:

1. **Three branches, three PRs.** `fix/git-index-cf2-cleanup` (v0.2.14), `fix/wrapper-hardening` (v0.2.15 off v0.2.14 once v0.2.14 lands), `docs/concurrent-commit-design-note` (v0.2.16 off v0.2.15 once v0.2.15 lands). More overhead, but each version has its own merge gate. Best fit for "stuck-at-intermediate" being a real concern.

2. **One branch, three PRs by stacking.** `fix/cf2-cleanup` first; once merged, `fix/wrapper-hardening` rebased onto main; then `docs/design-note`. Standard stacked-PR pattern.

3. **One branch, one PR, but each commit is independently revertible.** Verify that `git revert` of v0.2.15's commit alone leaves v0.2.16's design note coherent. If yes, single-PR is acceptable because revert is a viable rollback path. If no, force option 1 or 2.

**Recommendation:** Option 2 is lowest overhead; Option 1 is safest. Option 3 requires verification of revert-coherence which the plan currently does not do.

**Question for sma:** Is the v0.2.16 design note (`docs/design/2026-04-30-cross-session-reconciliation.md` per line 60) tightly coupled to v0.2.15's specific wrapper changes, or is it a standalone document about the CF-1 reconciliation issue? If standalone, it can be commit-1 (off v0.2.13) or commit-N — the ordering is interchangeable, so a single-branch bundle is fine. If coupled, ordering matters and stacking is preferable.

---

## P1: Cache-mirror step (local-machine destructive write) lacks ask-before-proceed gate

**Location:** Ship plan §"Cache mirror (local stopgap)" (lines 72-74).

**Failure scenario:** Sylveste/CLAUDE.md (which I have read in this session — see system context) states: "For irreversible actions (publish, delete, merge, bead-close), always ask before proceeding."

The cache mirror step writes files into `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/`. This is a destructive write — it overwrites the upstream-published 0.2.13 hook files with patched content. While not a "publish/delete/merge" in the most literal sense, it has irreversibility properties:

- The original upstream files are gone after overwrite (unless backed up elsewhere).
- A user who later wants to verify "what was the original 0.2.13 stop.sh?" would have to re-download from the marketplace.
- It changes the runtime behavior of all future Claude Code sessions on this machine until manually undone.

The Sylveste working-style note says: "When you have enough context to start implementing, do it." But it carves out irreversible actions as a separate category that requires explicit consent.

**Concrete gap:** The plan describes the cache mirror as a procedural step. There is no checkpoint between "draft the patch" and "apply the patch." If executed by an agent, this would proceed without sma's explicit consent at the apply moment.

**Smallest viable fix:** Add to the plan, immediately before line 72 ("Cache mirror"):

> **Ask gate:** Before writing files into `~/.claude/plugins/cache/`, present sma with: (1) the list of files to be overwritten, (2) a backup of the original content (e.g., `cp -r 0.2.13 0.2.13.upstream-backup`), (3) the diff that will be applied. Wait for explicit approval before proceeding.

This adds ~30 seconds of ceremony but converts an implicit destructive write into an explicit one.

---

## P2: Orphan repair (638 elf-revel files) lacks a mid-sequence checkpoint

**Location:** Ship plan §"Repair existing damage" (lines 76-83), specifically the procedure described at lines 82-83:

> For each: enumerate `.git/index-<UUID>` files, check if entries differ from HEAD, quarantine non-empty as `.git/index-<UUID>.abandoned-<ts>`, delete empties. Then `git reset --mixed HEAD` to rebuild canonical `.git/index` from HEAD. Verify with `git status --short --branch`.

**Failure scenario:** The procedure as written is a single shell pipeline. Once started, it runs end-to-end:
1. enumerate orphans
2. classify (empty / non-empty)
3. quarantine non-empties
4. delete empties
5. `git reset --mixed HEAD` (rebuild canonical index)
6. verify

There is no pause between step 2 (classify) and step 3-4 (destructive). For 638 files, the classify step produces a list of "640 candidates: 615 empty (will delete), 23 non-empty (will quarantine)." That's the moment sma should review the classification before destruction begins. Without a checkpoint, all 615 deletes happen automatically.

If the classifier has a bug (which fd-repair-safety-protocol can speak to in detail), the deletion is irreversible.

Sylveste/CLAUDE.md explicitly lists `delete` as an irreversible action requiring ask-before.

**Concrete gap:** The plan describes the procedure as a continuous sequence, not as a multi-stage pipeline with a human-review pause point.

**Smallest viable fix:** Re-structure the repair procedure as:

```
Stage A (read-only):
  - Enumerate orphans
  - Classify each (empty / non-empty / cannot-classify)
  - Write report to /tmp/orphan-classify-<repo>.txt
  - Stop. Present report to sma.

Stage B (after sma approval):
  - Quarantine non-empties (move, not delete — reversible)
  - Stop. Present "639 quarantined to .abandoned-<ts>" to sma.

Stage C (after sma approval):
  - Delete empties
  - Stop. Confirm "615 empties deleted."

Stage D (after sma approval):
  - git reset --mixed HEAD
  - Verify with git status

Per-repo: do mediumsetting (2 files) FIRST as a smoke test. Validate the
procedure works end-to-end on small input before running on elf-revel (638).
```

This converts the destructive part of the plan from "638 deletes happen automatically" to "638 deletes happen with three explicit consent points."

---

## P3: CI / GitHub Actions verification done by inspection — record it in the plan

**Location:** Ship plan §"Execution mechanics" (lines 65-69) — the assumption "do not auto-merge" is not currently documented.

**Verified by inspection:**
- `.github/workflows/ci.yml` triggers on `push: branches: [main]` and `pull_request:`. It runs `go build`, `go vet`, `go test -race`. **It does NOT run any merge action, publish action, or release action.**
- `.github/workflows/secret-scan.yml` triggers on `pull_request`, `push: branches: [main, master]`, and a daily cron. It runs gitleaks. **It does NOT run any merge or publish action.**

**No other workflows exist.** Specifically, there is no `release.yml`, no `publish.yml`, no `auto-merge.yml`. **The ship plan's safety assumptions are mechanically enforced by absence.**

**Why P3:** This is a positive finding. The plan's "do not auto-merge, do not publish" constraint is currently honored by the repository's actual CI configuration. But the plan doesn't say "we verified there are no auto-merge / auto-publish workflows." Future-sma may add such a workflow without realizing the ship plan implicitly depends on their absence.

**Smallest viable fix:** Add to the plan a short subsection:

> **Verified:** As of ffe8129, the repository has two GitHub Actions workflows (`ci.yml`, `secret-scan.yml`) and neither performs merge or publish. The "branch-only push" constraint is currently mechanically safe. If future workflows are added, re-verify before applying this plan as a template.

---

## What I did NOT review (per agent boundaries)

- Whether the version bumps themselves are semver-correct — fd-semver-coordination.
- Whether the cache mirror is durable against marketplace resync — fd-cache-mirror-drift.
- Whether the test suite covers the right failure modes — fd-test-coverage-gaps.
- Whether the orphan classifier is *correct* (vs. whether the procedure has a checkpoint) — fd-repair-safety-protocol.

---

## Decision summary

**Branch-only-push, no auto-merge, no publish:** mechanically safe in this repository (verified). Document the verification in the plan.

**PR-against-self in single-maintainer repo:** is *not* pure ceremony when:
- The branch contains 3 logically-independent commits (it does — v0.2.14, v0.2.15, v0.2.16).
- The diff is large enough that visual review benefits from PR-line-comment UX (the plan's file list is ~10 files plus tests).
- Future-sma may want a paper trail (the PR is a permanent artifact; chat-only review is not).

**It IS ceremony when:**
- The branch is a one-line typo fix.
- The commit is a documentation-only change with no behavioral risk.

For *this* plan, PR-against-self is justified. The bigger structural problem is that 3 commits on one branch couples the review of three independent decisions. Recommendation: either stack the PRs (option 2 above), or verify that `git revert` of any single commit leaves the others coherent (option 3 above).

**Three irreversible actions in the plan that need explicit ask-before gates:**
1. The merge of the PR (covered by Sylveste/CLAUDE.md).
2. The cache mirror write (P1 above — currently missing a gate).
3. The orphan-delete step in repair (P2 above — currently missing mid-sequence checkpoints).

Adding gates 2 and 3 brings the plan into compliance with Sylveste/CLAUDE.md's irreversibility doctrine.
