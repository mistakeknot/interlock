### Findings Index
- P0 | VCE-1 | "Repair existing damage" | Sweeper deployed untested against 638-file corpus — reversibility not established
- P1 | VCE-2 | "Cache mirror" | Cache mirror violates distinguishability: local fabric claims identity it no longer has
- P1 | VCE-3 | "v0.2.14 — Lifecycle hotfix" | Scope-creep from conservation into renovation — four separate interventions bundled into one
- P2 | VCE-4 | "v0.2.14 files touched" | No in-file provenance markers distinguish restored fabric from original v0.2.13 code
- P2 | VCE-5 | "v0.2.16 — design-note-only" | v0.2.16 honest-lacuna is structurally sound but relies on social visibility that xfail tests do not provide
- P3 | VCE-6 | "Repair existing damage" | Forensic reference sample of elf-revel orphan corpus not preserved before any sweep
Verdict: needs-changes

## Summary

Through the Venice Charter lens, the plan contains one authentic act of conservation (v0.2.16 honest lacuna), one renovation that has overrun its brief (v0.2.14 scope), one clear authenticity violation (cache mirror), and one irreversible act applied to an unstabilized corpus (sweeper on 638 orphan files). The most serious finding is VCE-1: a novel classify-before-delete protocol applied without dry-run validation to elf-revel's 638 files is an irreversible act, and the Venice Charter requires that all interventions on irreplaceable material be reversible or at least preceded by a complete documentary record. The cache mirror (VCE-2) is an authenticity violation in the conservation sense: the local copy claims version 0.2.13 identity while its material fabric is v0.2.14+v0.2.15.

## Issues Found

### VCE-1 — P0 — Sweeper deployed untested against 638-file corpus: reversibility not established

**File**: `scripts/interlock-orphan-sweep` (new, unimplemented at plan time) + plan section "Repair existing damage"  
**Failure scenario**: `session_index_is_empty()` in `hooks/lib.sh` has a classification boundary between "zero git-index entries" and "entries identical to HEAD." If the implementation uses `wc -l` on the raw binary index file rather than `git ls-files --others` or `git diff-index HEAD`, a non-empty index with staged files will read as empty. The sweep then deletes it. The 638-file corpus in elf-revel contains the only copies of any work staged before the stop.sh CF-2 bug manifested. Once deleted, that work is gone.

**Venice Charter test**: This fails minimum-necessary (638 files processed in one sweep, not incrementally), reversibility (deletion of empty-classified files is permanent, and `git reset --mixed HEAD` rebuilds the canonical index from HEAD, not from the orphan), and honest documentation (no dry-run manifest produced before execution).

**Smallest fix**: Before running the sweep on elf-revel, require a `--dry-run` pass that prints a manifest of intended actions (delete / quarantine) for every file, outputs to stdout, and exits without modifying anything. Human reviews the manifest. Only then run live. This is one flag, not a rewrite.

**Question**: Does `session_index_is_empty()` distinguish zero-entry indexes from indexes whose entries are identical to HEAD? The second is not archaeologically empty.

---

### VCE-2 — P1 — Cache mirror violates distinguishability: local fabric claims identity it no longer has

**File**: `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/` (all hook files) + `.claude-plugin/plugin.json` (stays at 0.2.13)  
**Failure scenario**: A support request arrives referencing a bug. The maintainer SSHes into the affected machine, runs `cat .claude-plugin/plugin.json | jq .version` — reads `0.2.13`. They check the 0.2.13 release, find the bug, and conclude the fix is not yet deployed. In fact the fix was deployed via cache mirror. Debugging proceeds on a false premise. The conservation equivalent: a restored façade with period-authentic material but a plaque that reads the original construction date, causing architectural historians to misread the stratigraphy.

**Venice Charter test**: Fails distinguishability. The repair is deliberately concealed behind the original version label.

**Smallest fix**: Write `.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/.LOCAL-PATCHES-APPLIED` with one line per patched file, the source version, and the timestamp. One `echo` command. The plugin.json need not change; the marker file documents the divergence for any future reader.

---

### VCE-3 — P1 — Scope-creep from conservation into renovation

**File**: Plan section "v0.2.14 — Lifecycle hotfix" (5 files touched + new sweeper script)  
**Failure scenario**: Not a single-failure-mode scenario, but a compounding risk: v0.2.14 bundles stop.sh path reconstruction (conservation of CF-2), atomic init (restoration), escape hatch `INTERLOCK_DISABLE_INDEX_ISOLATION=1` (renovation — adds new capability not in original), and a new orphan sweeper (renovation). If v0.2.14 fails in production, rolling back reverts all four simultaneously. A user who needed only the stop.sh fix cannot get it without the sweeper.

**Venice Charter test**: Fails minimum-necessary. A conservation intervention on CF-2 does not require bundling a new sweep script into the same version. The sweeper is a renovation — it adds capability not present in the original design.

**Smallest fix**: Split v0.2.14 into: (a) `v0.2.14a` — stop.sh path reconstruction + atomic init only; (b) `v0.2.14b` or `v0.2.15` — sweeper + escape hatch. This allows rollback of the sweeper without losing the path-reconstruction fix. If splitting versions is too much ceremony, at minimum separate the commits so the sweeper can be reverted independently.

---

### VCE-4 — P2 — No in-file provenance markers distinguish restored fabric from original code

**File**: `hooks/session-start.sh`, `hooks/stop.sh`, `hooks/lib.sh`  
**Scenario**: A maintainer in 2027 reads session-start.sh and sees the atomic-init block. There is no comment indicating when it was added, why, or what bug it addresses. They cannot tell whether this is original v0.2.13 design or a restoration. The commit history exists, but in-file provenance is required by Venice Charter for any intervention on a living artifact.

**Smallest fix**: Add a one-line comment above each new block: `# [v0.2.14] atomic init — fixes CF-5 TOCTOU race (2026-04-30)`. One line per change block.

---

### VCE-5 — P2 — v0.2.16 honest lacuna is structurally sound but socially invisible

**File**: `tests/integration/test_concurrent_commit_loss.py` (xfail) + `docs/design/2026-04-30-cross-session-reconciliation.md`  
**Scenario**: v0.2.16 ships. CI runs the test suite. The xfail test is marked `xfail` and therefore produces a green checkmark in CI output. A new contributor checking CI sees all green and concludes the plugin is correct. The lacuna is there but invisible on the public face of the artifact.

**Venice Charter test**: Honest lacuna requires that the gap be *visible* — marked in a way that a casual observer will encounter it. A test buried in `tests/integration/` with an `xfail` marker fails this. A top-level KNOWN-ISSUES.md or a README section "Known Limitations" would satisfy the doctrine.

**Smallest fix**: Add one section to README.md: `## Known Issues — CF-1: Concurrent commit silent data loss (see docs/design/2026-04-30-cross-session-reconciliation.md for status)`. One paragraph. This is the gold seam on the visible face.

---

### VCE-6 — P3 — No forensic reference sample preserved before sweeper runs

**File**: Plan section "Repair existing damage" (elf-revel, 638 orphan files)  
**Scenario**: After the sweep, the orphan corpus is gone except for quarantined non-empties. No reference set was preserved to verify the sweeper's behavior against. If the sweeper has a bug, there is no ground truth to compare against.

**Conservation recommendation**: Before any sweep, take a `tar czf /tmp/elf-revel-orphan-corpus-$(date +%s).tar.gz $(find .git -name 'index-*' -not -name '*.abandoned*')` snapshot. Store for 90 days. This is a single command, not a protocol change.

---

## Reframing

What the Venice Charter lens reveals that standard engineering review misses:

**Standard engineering** asks: does the sweeper correctly classify files? Does the fix address the bug?

**Venice Charter** asks: is the intervention reversible? Is the repair distinguishable from the original? Is the damaged section being honestly documented or speculatively completed?

The reframing surfaces three things the engineering lens obscures:

1. **The cache mirror is not a "local stopgap" — it is an authenticity fraud.** From an engineering perspective, it's pragmatic: ship the fix now, let the version string catch up later. From a conservation perspective, the object (the installed plugin) now has a false identity label. Every future diagnosis of that machine will be corrupted by the false version claim.

2. **v0.2.16 honest lacuna is actually the most defensible act in the plan.** Engineering review tends to see "failing test + no fix" as a problem. Conservation sees it as the correct approach when a speculative fix would introduce more uncertainty than the damage itself. The lacuna doctrine says: when you cannot safely repair, document the gap clearly. v0.2.16 does this — the concern is only that the documentation is not visible enough.

3. **Scope creep in a conservation context is not an efficiency problem — it is a reversibility problem.** Bundling four interventions into v0.2.14 means that if any one of them causes a regression, all must be reverted together. The conservation principle of minimum-necessary-action exists precisely to prevent this: do only what is required to stabilize, and do it in an isolatable unit.

<!-- flux-drive:complete -->
