---
problem_type: data_loss
severity: resolved
modules:
  - .git
  - hooks/session-start.sh (pre-f1c79a2)
symptoms:
  - bead marked closed but symbol from its acceptance criteria is missing from HEAD
  - commit message describes docs change but stat shows large source file deletions
  - canonical .git/index shows phantom staged-deletions of files that exist on HEAD
  - per-session git wrapper and `git status` without that wrapper return wildly different change sets
lastConfirmed: 2026-05-21
provenance: incident
review_count: 1
related:
  - elf-revel commits 48396b0 (destructive variant), b782107 (non-destructive variant), d8032a5 (forward-fix), d92d62c (per-session-worktree mitigation in elf-revel scripts)
  - interlock commit f1c79a2 (architectural fix — replaced session index isolation with per-session worktrees)
  - sylveste-4pth (tracking bead, closed)
---

# Stealth-revert via multi-agent per-session-index pollution

> **Upstream context.** This post-mortem originated in the elf-revel project (where the bug was first diagnosed and recovered from) and is preserved here as the historical record. Project-specific references (`Revel-t93`, `crates/er-sim`, etc.) are retained as evidence of the real-world incident; the bug pattern applies to any project using interlock multi-agent coordination on a shared worktree.
>
> **Resolution (2026-05-21).** Interlock `f1c79a2` ("Replace session index isolation with worktrees") removed the `GIT_INDEX_FILE` wrapper entirely. Each session now gets its own linked worktree via `git worktree add`, with its own working directory and index. The shared-canonical-index failure mode is structurally eliminated. This document remains as the rationale for the architectural change — anyone reading `f1c79a2` can find the original incident here.
>
> Pre-f1c79a2 mitigations (canonical-refresh on commit; project-local `session-spawn.sh` scripts) are superseded by interlock's built-in worktree isolation and can be removed once consumers upgrade past 0.2.14.

## Symptom

Three different signals fire together when this pattern hits. Any one alone is debuggable; all three is a recovery scenario.

1. **Bead status disagrees with code.** `bd show <feature>` reports `closed` with an acceptance citation like `cargo test passes (405 tests)`. Then `grep -rn "<symbol from feature>" crates/` returns zero hits. The bead believes the feature shipped; the tree disagrees.
2. **Commit messages lie about scope.** A commit titled `docs(brainstorm): ...` carries a `--stat` that includes 500+ lines of deletions in `crates/<sim>/`. The reviewer who landed it probably never saw the full stat — agent commit flows favor message-first review and stat scrolls past quickly.
3. **`git status` returns different sets depending on `GIT_INDEX_FILE`.** With the harness's per-session index env var set, you see ~20 modifications focused on the feature you're working on. With `env -u GIT_INDEX_FILE`, you see ~140 entries including phantom staged-deletions of `.beads/`, `.claude/agents/*`, etc. — files that are still committed on HEAD.

## Root cause

The multi-agent harness sets a per-session `GIT_INDEX_FILE=.git/index-<session-uuid>` so two agents editing different files in the same worktree don't race on `.git/index`. The pre-commit hook is supposed to merge sibling-session indexes back into the canonical index when a session commits.

The actually-broken case is when **one session does a tree-affecting operation outside the index** — e.g. `git stash` (without `-u` or with), `git reset --hard`, `git checkout <ref>` — while another session has uncommitted work in its per-session index. The post-tree-change commit captures the *intersection* of "what the canonical index thinks is staged" and "what's actually in the worktree," which silently looks like deletions of files the other session was holding modifications for.

In the elf-revel case (commit `48396b0`, 2026-04-30 22:19): a brainstorm session that started before commits `5fcf66f` (F1) and `a7a2160` (F2) landed earlier the same day did a tree-affecting operation, then committed what it had. The result is a commit titled "docs(brainstorm): interforge — wiki/code drift monitor" that *also*:

- Deleted `crates/er-sim/src/sim/lexicon.rs` (entire 509-line F2 module)
- Deleted F1 schema additions across `components.rs` (-178), `events.rs` (-187)
- Deleted F1 wiring across `world.rs` (-33), `arrival.rs` (-14), 7 systems files
- Deleted the F1 + F2 handoff docs and the epithets PRD
- Added the legitimate brainstorm artifacts the commit message describes

None of the deletions are mentioned in the commit message. There is no indication anything went wrong.

## Detection rule

Add this check to any session-start ritual that touches features tracked by beads:

```bash
# For each "closed" bead whose acceptance mentions a code symbol:
bd show <bead-id> | grep -oE '\b[A-Z][A-Za-z0-9_]+\b' | head -10  # candidate symbols
git grep -l '<symbol>' -- 'crates/' || echo "MISSING — bead may have been reverted"
```

A more general signal: `git log --stat <bead-close-date>..HEAD -- crates/` should show *additions*, not net deletions, for any feature beads that are marked closed. If a docs/research commit shows `crates/` deletions, treat it as suspect.

## Recovery procedure

The polluted worktree's index cannot be trusted for staging. Even `git diff HEAD > recovery.patch` captures the index's lies (it includes spurious `+++` and `---` entries for files that exist on HEAD but the canonical index thinks don't).

**Always recover from a fresh clone, not from the polluted worktree:**

```bash
# 1. Backup polluted worktree state out of band before anything else
mkdir -p /tmp/<bead-id>-survivors
tar -czf /tmp/<bead-id>-survivors/snapshot.tgz crates/ docs/  # whatever's at risk
cp <critical files> /tmp/<bead-id>-survivors/critical/

# 2. Fresh clone in a new directory
git clone <origin> /tmp/<project>-recovery
cd /tmp/<project>-recovery
git checkout -b recover/<bead-id>-<short-desc>

# 3. Direct file copy from polluted worktree to clean clone
#    Curate the file list — do NOT use rsync of the whole worktree
for f in $(cat recovery_files.txt); do
    mkdir -p "$(dirname "$f")"
    cp "/path/to/polluted/$f" "$f"
done

# 4. Verify in clean clone
cargo build && cargo test  # or project equivalent
git status  # should show clean intent: just the recovery files

# 5. Stage explicitly, never `git add -A`
git add <specific paths>
git commit -m "feat: restore X after stealth-revert (<bead-id>)"
git push -u origin <recovery-branch>
gh pr create  # let a human eyeball the diff before merge
```

The polluted worktree stays untouched until the recovery merges. After merge, rename it aside (don't delete) and clone fresh into the original path:

```bash
mv ~/projects/<project> ~/projects/<project>-polluted-snapshot
git clone <origin> ~/projects/<project>
# Audit ~/projects/<project>-polluted-snapshot for anything else that
# should be migrated (in-progress notes, .beads/issues.jsonl edits, etc.)
```

## Why direct file copy beats `git apply`

`git diff HEAD > patch && git apply patch` (the obvious recovery) fails when the canonical index is lying because:

- The diff contains synthetic entries for files the index thinks are missing-from-HEAD (deletions on the patch) and files the index thinks are added (additions of files that already exist on HEAD).
- `git apply` would either reject those entries (best case) or silently apply them (worst case), turning the recovery into a second stealth-revert.

Plain `cp` from polluted worktree to clean clone works because the **worktree blobs are correct** — the rot is in `.git/index`, not in the file contents on disk. The clean clone's index, freshly populated by `git checkout main`, doesn't inherit the rot.

## Why this happens specifically with multi-agent harnesses

Single-agent setups don't hit this because `git stash` / `git reset --hard` / `git checkout` always operate on `.git/index` and the worktree atomically. The per-session-index wrapper severs that atomicity:

1. Session A opens, gets its own index file
2. Session A modifies files in the worktree (correct)
3. Session B opens concurrently, also gets its own index file, runs `git stash --include-untracked`
4. The stash command operates on the **canonical** `.git/index` (not Session B's per-session index — most stash code paths ignore `GIT_INDEX_FILE`), and reverts the worktree to HEAD
5. Session B's worktree is now at HEAD, but Session A's per-session index still has Session A's modifications staged for files that no longer exist in the worktree
6. When Session B commits, the canonical-index-staged content (which is now a snapshot of the post-stash state) becomes the commit tree. Session A's work is gone from the commit. Session A's per-session index thinks it's still staged.

The fix would be either: (a) the harness wraps stash/reset/checkout with the same `GIT_INDEX_FILE` discipline, or (b) sessions never run tree-affecting commands and the harness mediates them, or (c) per-session worktrees instead of per-session indexes (using `git worktree add`).

## How cass-as-bridge made recovery tractable

Detection of this disaster in the elf-revel session would have taken hours of forensic git archaeology. Instead, `cass search` indexed prior session transcripts surfaced *the previous F3 session* (`69f69ee7-8c9c-4ece-83ab-5e0e530a18f1` line 342) which had reached the same diagnosis three days earlier and stopped before pushing. Plus a meta-session (`dfc512ca-6f8a-498d-8867-2756ddb307b3`) that had traced the harness root cause across multiple projects.

Inheriting those diagnoses cut the investigation from "many hours" to "~20 minutes." Worth keeping cass indexed and searchable for any project where multi-agent sessions write to a shared worktree.

## Variant: stale-canonical-index fork (non-destructive, recurs on fresh clones)

The 2026-05-05 incident above is the destructive form — a session's per-session index loses work because another session re-set the tree out from under it. A second, **non-destructive** variant of the same root cause showed up on a *fresh clone* (commit `b782107`, 2026-05-14, in the post-Revel-t93 reclone). No data is lost in this variant, but commits silently revert prior in-tree work that the polluter's session can't see.

### Symptom

A new session opens, runs `git status`, and sees pre-staged changes it did not make:

```
D  docs/handoffs/2026-05-14-revel-t93-snapshot-audit-and-cleanup.md   # staged-deleted
MM docs/handoffs/latest.md                                            # staged + worktree modified
?? docs/handoffs/2026-05-14-revel-t93-snapshot-audit-and-cleanup.md   # untracked, byte-identical to deleted version
```

The file is **on disk**, byte-identical to its `git show HEAD:<path>` content, but the session's per-session `GIT_INDEX_FILE` thinks it's untracked-pending-deletion. If the session runs `git add <unrelated file>` and commits, the staged deletion + staged symlink change ride along into the commit, silently reverting the prior work.

### Why this is worse than the destructive variant

- No `git status` smoke alarm — output looks like "someone left work in flight, you can extend it." A polite agent might think "let me not disturb this" and proceed; a careless one bundles it into their own commit.
- `cargo test` passes (the worktree content is fine).
- The reverted commit shows up cleanly in `git log --stat`: the bad commit's stat will include `<unrelated-thing> | +N` AND `D path/to/file | -M`. Code review that focuses on the message + headline file misses the `D` line entirely.
- Pushes succeed. The bad state lands on origin. The next agent inherits it as the new "normal."

### Root cause

The destructive variant requires two concurrent sessions racing on a tree-affecting operation. The non-destructive variant only requires **one** prior session that committed via a per-session index without resyncing the canonical `.git/index` to HEAD afterward.

Sequence in the b782107 case:

1. Session A starts at 14:38, forks `GIT_INDEX_FILE=.git/index-839340ce-...` from canonical `.git/index` (which is at the pre-handoff state).
2. Session A creates `docs/handoffs/2026-05-14-revel-t93-snapshot-audit-and-cleanup.md`, repoints `latest.md`, and commits `827cb8e` at 14:39 using its session index `839340ce`.
3. Session A never writes its post-commit index state back to canonical `.git/index`. Canonical `.git/index` stays at the pre-handoff state.
4. Session B starts at ~15:03, forks `GIT_INDEX_FILE=.git/index-bfd6f3c2-...` from canonical `.git/index` (still stale — file `2026-05-14-...` doesn't exist in canonical's tracked set).
5. Session B's `git status` correctly reports the file as `D` (it's in HEAD but not in B's index) and the symlink as `MM` (changed in both HEAD's symlink target and B's stale index).
6. Session B runs `git add <wiki-file>` for unrelated wiki work, commits. The staged deletion + symlink revert ride along.

The destructive variant's root cause is "stash/reset bypasses per-session index discipline." The non-destructive variant's root cause is "commit doesn't write back to canonical." Different bug, same harness, same victim.

### Detection at session start

Before any `git add`, every session must verify the inherited index matches HEAD:

```bash
# Run at session start. Any output = inherited stale-canonical-index fork.
git status --short | grep -E '^(D |MM | M|A |R )' && {
  echo "WARNING: pre-existing staged changes inherited from canonical .git/index" >&2
  echo "DO NOT git add or commit until you have resolved this." >&2
  git status --short
}
```

If output appears:

1. **Do not run `git add` for any file** — even unrelated ones. `git add` writes the *full* index, not just the file you specify; the spurious staged entries persist.
2. **Verify each inherited entry against HEAD.** For each `D` entry, run `git ls-tree HEAD -- <path>`. If the file exists on HEAD but not in your index, it's the stale-canonical-fork bug. For each `MM` symlink, run `diff <(git show HEAD:<path>) <(readlink <path>)`. If the worktree symlink matches HEAD's symlink, the only diff is in *your index*, which is wrong.
3. **Resync your session index to HEAD before doing anything else.** The lowest-blast-radius operation is `git read-tree HEAD` against your `GIT_INDEX_FILE`. This rebuilds the session index from the current HEAD tree, dropping the inherited staged entries. The worktree is untouched.

```bash
# Resync your session's index to HEAD without touching the worktree.
git read-tree HEAD
git status --short  # should now be clean (or only show YOUR own edits)
```

### Recovery if you already committed

If you didn't catch it pre-commit and your `git log -1 --stat` shows surprise `D` / mode-change entries:

1. **Do not amend.** Amending preserves the bad tree; you'd commit it again.
2. **Do not `git revert <bad-commit>`.** That undoes your *intended* work too.
3. **Forward-fix commit.** Re-add the deleted files (`git add <path>`, since they're still on disk byte-identical), re-create the symlink with the correct target, commit on top:

```bash
# Resync first so add doesn't re-stage the stale entries:
git read-tree HEAD

# Re-add the dropped file(s) — content is unchanged, just re-tracking:
git add docs/handoffs/2026-05-14-revel-t93-snapshot-audit-and-cleanup.md

# Re-point the symlink:
ln -sf 2026-05-14-revel-t93-snapshot-audit-and-cleanup.md docs/handoffs/latest.md
git add docs/handoffs/latest.md

git commit -m "docs(handoff): restore <file> and latest.md symlink — fix stale-index-fork in <bad-sha>"
git push
```

This is safe because the file blob and the correct symlink target are both **already known** (you can verify with `git rev-parse HEAD~N:<path>`). No history rewrite, no force push, no coordination with other sessions needed. The forward commit makes the regression visible in `git log` (which is good — it's a self-documenting record of the bug).

### Why this variant keeps appearing on fresh clones

The Revel-t93 rename-and-reclone procedure (above) restores a clean canonical `.git/index` at clone time. But the very first commit any per-session index makes after clone leaves the canonical index stale again, and the bug arms itself for the *next* session.

The structural fix is one of:

- **Resync on commit.** Add a post-commit hook that writes the just-committed tree to canonical `.git/index` (`git read-tree HEAD > .git/index` or equivalent). This costs ~10ms per commit and closes the window.
- **Per-session worktrees instead of per-session indexes.** Each agent gets its own `git worktree add <path>` instead of sharing the canonical worktree. Canonical `.git/index` is no longer a shared resource.
- **Session-start resync.** Every session begins with `git read-tree HEAD` against its session index. Costs one extra read per session start. Cheapest mitigation; doesn't fix the root cause but reduces blast radius to "session that committed but didn't push before the next session opened."

Until one of these lands, every session must run the session-start detection script above.

## See also

- `Revel-t93` (P0 bug filed for this incident)
- PR #1 (recovery commit `f9dc7482`, squashed from `24182f6`)
- Commit `48396b0` (the original stealth-revert — destructive variant)
- Commit `b782107` (the non-destructive variant — 2026-05-14 incident)
- Commits `5fcf66f` (F1) and `a7a2160` (F2) — preserved in git object DB even though the in-tree content was reverted; recovery preserves these by re-applying the *content* rather than reverting the bad commit
