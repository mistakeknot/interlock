# fd-repair-safety-protocol — Ship Plan Findings

**Reviewer:** fd-repair-safety-protocol (data recovery / forensic-triage / chain-of-custody lens)
**Target:** `docs/research/flux-review/interlock-ship-plan-review/2026-04-30-ship-plan.md`
**Date:** 2026-04-30
**Project:** interlock orphan repair on mediumsetting (2 files), elf-revel (638 files), garden-salon (1), Sylveste (5), Lowbeer (1)
**Lens:** Classify-before-delete confidence; bake time before destructive operation; repair-vs-fix race; reversibility-first sequencing

---

## Summary

Three findings.

- One **P0**: The classify-before-delete sweeper is a *new, unvalidated* script proposed in v0.2.14, and the plan applies it to 638 real elf-revel files. If the classifier has an off-by-one, a charset bug, or an empty-detection bug (e.g., treating a partial-write index file as "empty" when it actually contains the only copy of staged work), the deletion is irreversible. The plan describes a quarantine-non-empty / delete-empty split — but the destructive arm (delete) is taken on the basis of an untested classification.

- One **P1**: The repair (delete orphans) on elf-revel is sequenced *before* the fix (CF-2 in v0.2.14) is *deployed and verified*. Until v0.2.14 is shipped and active on the user's machine, every elf-revel session continues to leak new orphans. Repairing orphans on a still-leaking system is a partial overlap — the post-repair state is not clean.

- One **P1**: `git reset --mixed HEAD` (line 84) is run as part of the repair after orphan handling. If any session has *legitimate* staged work in the canonical `.git/index` at the moment of repair, that staged work is silently discarded by the reset.

The smallest set of changes that makes the repair safe: (1) add a dry-run-only first pass with sma review of the classification output; (2) sequence repair *after* v0.2.14 is deployed and verified, not concurrent with deployment; (3) verify no live sessions are active before `git reset --mixed HEAD`; (4) test sweeper on mediumsetting (2 files) before elf-revel (638 files).

---

## P0: New, unvalidated classifier applied to 638 real files — destructive arm is irreversible if classifier is wrong

**Location:** Ship plan §"Repair existing damage" (lines 76-83), specifically:
> For each: enumerate `.git/index-<UUID>` files, check if entries differ from HEAD, quarantine non-empty as `.git/index-<UUID>.abandoned-<ts>`, delete empties.

And §"v0.2.14 — Lifecycle hotfix" (line 38):
> `scripts/interlock-orphan-sweep` — new TTL sweeper (>7 days → classify; empty → delete; non-empty → quarantine as `.git/index-<UUID>.abandoned-<timestamp>`).

**Failure scenario:** sma writes the new sweeper. The classifier uses some implementation of "is this index file empty." Plausible implementations and their failure modes:

1. **Implementation: `[ ! -s "$file" ]`** (zero file size). FAILS for valid empty git indexes — git's index file format has a 12-byte header even when empty. So `[ ! -s ]` would never match a real git index, and *everything* would be classified non-empty. Probably noticed quickly; not the dangerous case.

2. **Implementation: `git ls-files --cached --no-empty-directory` and check entry count.** Reads the index. FAILS if the index file is partially written (a session crashed mid-write) — `git` returns an error, and the sweeper's error-handling decides whether to delete or quarantine. If the sweeper treats "git error" as "empty," it deletes a corrupted index that may contain the only readable copy of the user's staged work in journaled form.

3. **Implementation: `GIT_INDEX_FILE=$file git ls-files | wc -l`.** Returns 0 for actually-empty indexes AND for indexes that only contain entries already in HEAD (because `ls-files` reports tracked files, not staged-vs-HEAD diff). If interpreted as "0 entries = safe to delete," this incorrectly deletes indexes where the user has staged a *deletion* of a tracked file (a real session state — `git rm --cached foo` produces an index with entries that match HEAD-minus-foo; not bit-identical to HEAD's index but functionally equivalent for `ls-files | wc -l`).

4. **Implementation: `git read-tree HEAD; diff $file <(git ls-files)`.** Closer to the spirit of the plan ("entries differ from HEAD"). FAILS for the case where the user staged a `chmod` (mode change) — `ls-files` doesn't show modes by default. The classifier reports "no diff = empty = delete" but the user's mode change is in the index and is now lost.

The plan describes the classifier semantically ("entries differ from HEAD") but does not specify the implementation, does not specify the test fixtures, and does not specify the validation against real orphans before the destructive run.

**The 638-file elf-revel corpus is unrecoverable if any non-trivial fraction of those files contain unique staged work that the classifier mis-classifies as "empty.**" The mortuary chain-of-custody principle (per the original Track C finding referenced at line 26-28: "orphan indexes may contain the only copy of staged-but-uncommitted work") explicitly warns against this.

**Why P0:** Irreversible data loss. The user's staged-but-uncommitted work in 638 files is exactly the data the plan acknowledges may have unique copies. Deleting based on an untested classifier is the failure mode the plan was supposed to prevent.

**Smallest viable fix:** Three concrete changes:

1. **Mandate a dry-run mode** for the sweeper (`--dry-run` flag that prints classification without acting). Run dry-run on elf-revel first. Output goes to a file. sma reviews the file. Concrete review actions: spot-check 5 randomly-sampled "empty" classifications by manually running `GIT_INDEX_FILE=$file git diff --staged HEAD` on each, confirm the classifier is right. Spot-check 5 "non-empty" classifications similarly. Only then proceed.

2. **Quarantine-only first pass.** For the v0.2.14 sweeper's *initial* deployment (and especially the elf-revel run), suppress the delete arm. Quarantine *everything* >7 days old as `.abandoned-<ts>`. This is reversible: `mv` of an `.abandoned-<ts>` file back to `.git/index-<UUID>` restores the orphan. After 30 days of uneventful operation, sma can re-evaluate whether to enable the delete arm. This costs disk space (~640 small files at ~few KB each = a few MB) for a few weeks; that's negligible.

3. **Validate classifier on synthetic golden corpus before real run.** Before running on real orphans, build a fixture set of 10-20 synthetic indexes with known-correct classification: empty (only header), single entry matching HEAD, single entry differing from HEAD, mode change only, deleted-tracked-file, partial-write (truncated to N bytes), corrupted (header bytes flipped). Confirm classifier produces correct classification for each. Document the fixture in the test suite. *Then* run on real data.

**Question for sma:** Has the orphan-sweep script been written yet, or is it still a design? If still a design, finding P0 is preventive — bake the dry-run, quarantine-only-first-pass, and validation-corpus into the design. If already written, run the validation-corpus test before applying to elf-revel.

---

## P1: Repair runs concurrent with bug — elf-revel keeps leaking orphans during the repair window

**Location:** Ship plan §"Cache mirror" (lines 72-74) and §"Repair existing damage" (lines 76-83). The plan's ordering of these two sections is implicit; the document does not state explicitly which runs first.

**Failure scenario:** The plan's two operations have a dependency ordering problem:

- **Cache mirror** patches `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/` so this machine no longer leaks new orphans.
- **Repair existing damage** quarantines/deletes 638 elf-revel orphans.

If the repair runs *before* the cache mirror is in place AND active in any future Claude Code sessions, those sessions will continue to produce new orphans during and after the repair. The post-repair state of elf-revel is not "638 → 0" but "638 → 0 → N (where N is the rate of leak per session × sessions during the window)."

If the cache mirror runs *first* but a Claude Code session is *already active* (a long-running terminal with `claude` mid-conversation), that session is using the *unpatched* hooks loaded at session start. The patch only takes effect for *new* sessions. So even after cache mirror, currently-running sessions continue leaking.

**Concrete gap in plan:** The plan does not specify:
- Whether all running Claude Code sessions must be terminated before cache mirror.
- Whether cache mirror must be verified active (e.g., by starting a new test session and confirming no orphans accumulate over a smoke-test commit) before running the repair.
- Whether the repair should be sequenced after v0.2.14 ships *to the source repo* (not just the cache mirror) so that future re-installs of the plugin from the marketplace also have the fix — relevant if sma uses elf-revel from another machine.

**Why P1:** The repair partially overlaps with ongoing accumulation. Not catastrophic — the leak rate is bounded — but the post-repair state is not clean, and a follow-up sweeper pass is needed to mop up the post-repair leak. The plan presents the repair as a one-shot operation; it isn't.

**Smallest viable fix:** Add to the plan an explicit ordering and pre-condition:

```
ORDER OF OPERATIONS (revised):

1. Apply cache mirror patches.
2. Verify cache mirror is active:
   - Quit all running `claude` sessions (`tmux kill-server` if needed, or one
     by one).
   - Start a fresh `claude` session in a scratch repo.
   - Run a commit that would have leaked under v0.2.13.
   - Confirm no `.git/index-<UUID>` orphan was created.
3. Run mediumsetting repair (2 files — smoke test of the procedure).
4. Verify mediumsetting state: `ls .git/index-* 2>/dev/null` returns nothing
   except possibly active session indexes.
5. Run elf-revel repair (638 files).
6. After repair, run a fresh smoke-test session in elf-revel, confirm no new
   orphans accumulate.
```

This explicitly serializes the dependency: fix is verified active *before* repair runs, and the smallest repo runs first as a smoke test.

---

## P1: `git reset --mixed HEAD` discards legitimate staged work if any active session holds it

**Location:** Ship plan line 84:
> Then `git reset --mixed HEAD` to rebuild canonical `.git/index` from HEAD.

**Failure scenario:** sma runs the repair on elf-revel. The repair quarantines the 23 non-empty orphans, deletes the 615 empties. Then `git reset --mixed HEAD` runs. *But*: at the moment of reset, an active Claude Code session in elf-revel had legitimate staged work in the canonical `.git/index` (the user did `git add foo.py` and was about to commit when the repair ran). `git reset --mixed HEAD` discards staged changes — moving them to working tree (not lost, but no longer staged). However, if combined with a `git checkout` or `git stash drop` later, or if the working tree changes between staging and reset (the file is modified), the user's intent is now ambiguous and recoverable only via reflog.

This is `git reset --mixed`'s documented behavior, not a bug, but the plan applies it without verifying that no legitimate staged state exists.

**Why P1:** Recoverable via reflog (within 90 days), but it's a silent destruction of staged state at the moment of reset. The Sylveste/CLAUDE.md irreversibility doctrine applies — this is a destructive git operation.

**Concrete gap in plan:** No precondition check on `git status` before `git reset`.

**Smallest viable fix:** Pre-check before reset:

```sh
if [ -n "$(git -C $repo diff --staged --name-only)" ]; then
    echo "ERROR: $repo has staged changes; refusing to git reset."
    echo "Either commit/stash the staged changes, or pass --force-reset."
    exit 1
fi
git reset --mixed HEAD
```

Also, capture the reflog hash *before* reset:

```sh
PRE_RESET=$(git -C $repo rev-parse HEAD)
git reset --mixed HEAD
echo "Reset complete. To restore, see git reflog or git reset $PRE_RESET --mixed."
```

This converts a silent reset into a checked, reversible-via-reflog operation.

---

## P2 (mention): Per-repo serialization vs. parallel batching

**Location:** Ship plan §"Repair existing damage" — the procedure is described once and applied to 5 repos: mediumsetting (2), elf-revel (638), garden-salon (1), Sylveste (5), Lowbeer (1).

**Concern:** The plan does not explicitly say "do mediumsetting first as a smoke test, then elf-revel, then the others." If interpreted as "loop over all 5 repos with the same procedure," a bug in the sweeper hits all 5 at once. If interpreted as "run each repo sequentially with verification between," a bug is caught after the first repo (the smallest, mediumsetting at 2 files).

**Smallest viable fix:** Per the P1 fix above, explicit ordering:
1. mediumsetting (2 files — smoke test).
2. Verify mediumsetting post-state.
3. garden-salon (1 file).
4. Lowbeer (1 file).
5. Sylveste (5 files).
6. Verify the procedure has worked correctly across small repos.
7. *Then* elf-revel (638 files) — the high-stakes run.

This is "biggest blast radius last" sequencing.

---

## What I did NOT review (per agent boundaries)

- Whether the test for the sweeper covers the failure modes above — fd-test-coverage-gaps.
- Whether the sweeper's introduction is correctly versioned in v0.2.14 — fd-semver-coordination.
- Whether the repair has an ask-before-irreversible gate — fd-irreversible-action-discipline.
- Whether the cache mirror is durable enough to make "verify cache mirror is active" meaningful — fd-cache-mirror-drift.

(That last one is a meaningful coupling: my P1 about repair-vs-fix race assumes the cache mirror is durable for the duration of the repair window. If fd-cache-mirror-drift's P0 is real — silent revert — my fix's "verify cache mirror is active" step needs a stronger guarantee than a one-time smoke test.)

---

## Decision summary

The repair plan is a **destructive operation on potentially valuable data** (the original review explicitly warned that orphan indexes may contain the only copy of staged work). The plan's classify-before-delete protocol is correctly motivated, but the destructive arm runs on the basis of an untested classifier, in a window where the underlying bug is still leaking, and includes a `git reset --mixed HEAD` with no preconditions.

**Order from highest risk to lowest:**

1. **Untested classifier deletes real data.** Mitigation: dry-run + spot-check + quarantine-only first pass + validation-corpus.
2. **Repair runs while bug is still leaking.** Mitigation: explicit ordering — cache mirror first, verify active, then repair.
3. **`git reset --mixed HEAD` discards live staged work.** Mitigation: pre-check `git diff --staged`, capture pre-reset hash.
4. **Wrong repo ordering — high blast-radius first.** Mitigation: smallest first, biggest last.

If sma has time to apply only one mitigation, it is the **quarantine-only first pass** (P0). Quarantine is reversible via `mv`. Even if every other safeguard fails, no data is permanently lost — only displaced into `.abandoned-<ts>` files. After a 30-day bake period with no reported corruption, the delete arm can be enabled.

**The plan's confidence in the classifier is its central risk.** A dry-run with sma's review of classification output, plus a small validated golden-fixture suite, is the minimum due diligence before running the destructive arm on 638 files.
