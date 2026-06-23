#!/usr/bin/env bash
# interlock-orphan-sweep.sh — reclaim leaked per-session worktrees from
# interlock <=0.2.15, which created one `git worktree add` per session under
# ~/.cache/interlock/worktrees/ with no cleanup path. 0.2.16 stopped creating
# them; this sweeper cleans up the accumulated backlog (and any left by a crash
# on an older version still in flight).
#
# Safety: a worktree is removed ONLY if it has no uncommitted changes AND its
# HEAD commit is reachable from some ref (i.e. nothing would be lost). Anything
# dirty or holding unreachable commits is QUARANTINED (moved aside), never
# deleted — honoring the original "don't destroy uncommitted work" intent.
#
# Invocation: best-effort from session-start.sh (throttled), or manually.
# Env:
#   INTERLOCK_WORKTREE_ROOT  override the base dir (default ~/.cache/interlock/worktrees)
#   INTERLOCK_SWEEP_AGE_DAYS only sweep dirs older than N days (default 1)
#   INTERLOCK_SWEEP_DRY_RUN  if set to 1, report only, change nothing
set -uo pipefail
trap 'exit 0' ERR

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
source "${SCRIPT_DIR}/../hooks/lib.sh" 2>/dev/null || true

BASE="$(legacy_worktree_base 2>/dev/null)"
[[ -n "$BASE" && -d "$BASE" ]] || exit 0

AGE_DAYS="${INTERLOCK_SWEEP_AGE_DAYS:-1}"
DRY="${INTERLOCK_SWEEP_DRY_RUN:-0}"
QUARANTINE="${BASE%/}/.quarantine"

removed=0; quarantined=0; skipped=0

# Each leaked worktree is BASE/<project-key>/<session-id>. Find leaf dirs that
# look like worktrees (contain a .git file pointing at an admin gitdir).
while IFS= read -r wt; do
    [[ -n "$wt" ]] || continue
    # Age gate: skip recently-touched dirs (a session may still be live on an
    # older interlock version).
    if [[ -n "$AGE_DAYS" && "$AGE_DAYS" != "0" ]]; then
        if [[ -n "$(find "$wt" -maxdepth 0 -mtime -"$AGE_DAYS" 2>/dev/null)" ]]; then
            skipped=$((skipped+1)); continue
        fi
    fi

    # Is it a real git worktree we can reason about?
    if ! command git -C "$wt" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        # Not a usable worktree (admin entry already pruned by git gc); it's an
        # orphan dir. Safe to remove only if it has no untracked content worth
        # keeping — but we can't diff it against a repo, so quarantine to be safe
        # unless it is empty.
        if [[ -z "$(ls -A "$wt" 2>/dev/null)" ]]; then
            [[ "$DRY" == "1" ]] || rm -rf "$wt" 2>/dev/null && removed=$((removed+1))
        else
            if [[ "$DRY" != "1" ]]; then
                mkdir -p "$QUARANTINE" 2>/dev/null || true
                mv "$wt" "$QUARANTINE/" 2>/dev/null && quarantined=$((quarantined+1))
            else
                quarantined=$((quarantined+1))
            fi
        fi
        continue
    fi

    # Dirty? (uncommitted changes) -> quarantine, never delete.
    if [[ -n "$(command git -C "$wt" status --porcelain 2>/dev/null)" ]]; then
        if [[ "$DRY" != "1" ]]; then
            mkdir -p "$QUARANTINE" 2>/dev/null || true
            mv "$wt" "$QUARANTINE/" 2>/dev/null && quarantined=$((quarantined+1))
        else
            quarantined=$((quarantined+1))
        fi
        continue
    fi

    # Clean. Confirm HEAD is reachable from a ref (no stranded detached commits).
    head_sha=$(command git -C "$wt" rev-parse HEAD 2>/dev/null) || head_sha=""
    reachable=1
    if [[ -n "$head_sha" ]]; then
        # main project root for this worktree (resolve the linked gitdir's parent repo)
        main_root=$(command git -C "$wt" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)
        main_root="${main_root%/.git}"
        if [[ -n "$main_root" ]]; then
            if ! command git -C "$main_root" branch -a --contains "$head_sha" 2>/dev/null | grep -q .; then
                reachable=0
            fi
        fi
    fi

    if [[ "$reachable" == "0" ]]; then
        # Detached commits not on any branch -> quarantine.
        if [[ "$DRY" != "1" ]]; then
            mkdir -p "$QUARANTINE" 2>/dev/null || true
            mv "$wt" "$QUARANTINE/" 2>/dev/null && quarantined=$((quarantined+1))
        else
            quarantined=$((quarantined+1))
        fi
        continue
    fi

    # Clean + reachable -> safe to remove via git so the admin entry goes too.
    if [[ "$DRY" != "1" ]]; then
        main_root=$(command git -C "$wt" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)
        main_root="${main_root%/.git}"
        if [[ -n "$main_root" ]]; then
            command git -C "$main_root" worktree remove --force "$wt" >/dev/null 2>&1 || rm -rf "$wt" 2>/dev/null
            command git -C "$main_root" worktree prune >/dev/null 2>&1 || true
        else
            rm -rf "$wt" 2>/dev/null || true
        fi
        removed=$((removed+1))
    else
        removed=$((removed+1))
    fi
done < <(find "$BASE" -mindepth 2 -maxdepth 2 -type d 2>/dev/null | grep -v "/.quarantine")

# Tidy now-empty project-key dirs.
[[ "$DRY" == "1" ]] || find "$BASE" -mindepth 1 -maxdepth 1 -type d -empty -delete 2>/dev/null || true

if [[ $((removed+quarantined)) -gt 0 ]]; then
    echo "INTERLOCK: orphan sweep — removed ${removed}, quarantined ${quarantined}, skipped ${skipped} (base ${BASE})" >&2
fi
exit 0
