### Findings Index
- P0 | NVR-1 | "Repair existing damage" | Untested classifier applied bulk-action to 638 items of uncertain provenance — inverts the required order
- P1 | NVR-2 | "scripts/interlock-orphan-sweep" | No --dry-run / --manifest-only mode: bulk disposition without per-item review gate
- P1 | NVR-3 | "hooks/lib.sh" | Empty-classification boundary undefined: entries-identical-to-HEAD is not archaeologically empty
- P2 | NVR-4 | "Repair existing damage" | Quarantine rename lacks sidecar provenance: timestamp alone cannot reconstruct session origin
- P2 | NVR-5 | "Repair existing damage" | Unidentifiable orphans (crashed sessions from 2025) receive no heightened caution protocol
- P3 | NVR-6 | "v0.2.14 — Lifecycle hotfix" | Protocol's first real-world deployment is against the largest corpus — pilot-first order inverted
Verdict: needs-changes

## Summary

Through the NAGPRA / Vermillion Accord lens, orphan `.git/index-<UUID>` files are items of uncertain provenance: some encode irreplaceable work (staged patches that no commit ever captured), others are genuinely empty. The plan's classify-before-delete intent is sound — the Vermillion Accord endorses classification before disturbance. The failure is in execution: the classifier (`session_index_is_empty()`) is untested, the sweep has no dry-run mode, no consultation step exists before bulk disposition, and the first real-world deployment is against the largest available corpus (638 elf-revel files). NAGPRA/Vermillion protocols require that novel procedures be piloted on small controlled cohorts before being applied at scale, that a per-item manifest be reviewed by the responsible party before irreversible action, and that items of unidentifiable provenance receive heightened caution rather than standard processing.

## Issues Found

### NVR-1 — P0 — Untested classifier applied as bulk action to 638 items of uncertain provenance

**File**: `scripts/interlock-orphan-sweep` (new), `hooks/lib.sh` (`session_index_is_empty()`), plan section "Repair existing damage" (elf-revel, 638 files)  
**Failure scenario**: `session_index_is_empty()` is called 638 times. Its definition is not specified in the plan — the plan says it will be added to `hooks/lib.sh` as a "classify helper." If the implementation checks the raw binary size of the index file (rather than parsing the index to count entries), any index file that had even one staged entry will be mis-classified based on byte count rather than content. An index file for a session that staged a critical patch file reads as "non-empty" → quarantined. But if the classifier has an off-by-one in the binary header offset (git index format v2 has a 12-byte header), it could mis-classify as empty → deleted. The deletion is immediate and permanent.

**NAGPRA test**: This inverts the correct order: (1) establish classifier validity on a controlled cohort with known ground truth; (2) pilot on the smallest available corpus (mediumsetting has 2 files — the correct first target); (3) scale to elf-revel only after pilot confirms correct behavior. Running the first real-world test on the largest corpus (638 files) contradicts the case-by-case review principle.

**Smallest fix**: Before running against elf-revel, run against mediumsetting (2 files) with manual before/after verification. Document the expected classification for each of those 2 files before running. Confirm match. Only then proceed to elf-revel. This is one sentence added to the execution plan: "Pilot on mediumsetting (2 files) with manual verification before elf-revel."

---

### NVR-2 — P1 — No --dry-run mode: bulk disposition without per-item review gate

**File**: `scripts/interlock-orphan-sweep` (new), plan section "Repair existing damage"  
**Failure scenario**: The sweep runs and produces deletions and quarantine renames. There is no mode that generates a manifest of intended actions for human review before any files are modified. The user cannot know before execution which files will be deleted and which quarantined. The first indication of a mis-classification is a missing file.

**Vermillion Accord test**: The Accord requires consultation before disturbance. In the software context, "consultation" means: the responsible party (sma) reviews a per-item manifest and explicitly confirms before any irreversible action. A `--dry-run` or `--manifest-only` flag satisfies this. Without it, the consultation step is absent.

**Smallest fix**: Add to `scripts/interlock-orphan-sweep`:
```bash
DRY_RUN=false
[[ "$1" == "--dry-run" ]] && DRY_RUN=true
```
When `DRY_RUN=true`, print each intended action (WOULD DELETE / WOULD QUARANTINE / SKIP) with the file path and the classification reason, then exit 0 without modifying any files. This is ~5 lines of bash, not a rewrite.

---

### NVR-3 — P1 — Empty-classification boundary undefined: entries-identical-to-HEAD is not archaeologically empty

**File**: `hooks/lib.sh` (`session_index_is_empty()`), plan section "v0.2.14 — Lifecycle hotfix"  
**Failure scenario**: A session ran `git add -p`, staged a partial hunk, then the session stopped before committing. The orphan index contains entries. But those entries are identical to the current HEAD tree (the commit landed from another session). `session_index_is_empty()` is implemented as `git diff-index --quiet HEAD -- && echo empty`. This returns "empty" — and the file is deleted. The staged-hunk information is gone, even though the function name said "empty."

**Archaeological classification**: For repatriation purposes, an index whose entries are identical to HEAD is not the same as a zero-entry index. The former is a witness to a session's final state; it tells us the session was clean at stop. The latter tells us the session was never used. Both are safe to delete, but the classification must be explicit, not collapsed.

**Smallest fix**: Define the classification in the plan's specification of `session_index_is_empty()`: "Returns true if and only if the index contains zero entries (as reported by `git ls-files --cached | wc -l`). An index with entries identical to HEAD is classified as 'clean' not 'empty' — both can be deleted, but they must be logged separately." One sentence in the spec.

---

### NVR-4 — P2 — Quarantine rename lacks sidecar provenance: timestamp cannot reconstruct session origin

**File**: Plan section "Repair existing damage" (quarantine rename pattern `.git/index-<UUID>.abandoned-<timestamp>`)  
**Scenario**: A quarantined file `.git/index-b0df1a87-1385-4560-a867-d15cb1f97b6b.abandoned-1746153600` is examined six months later. The timestamp says when it was quarantined, not when it was created. The UUID identifies the session, but the session metadata (which repo, which user, which branch, which files were staged) is not recorded anywhere. Future archaeology cannot reconstruct provenance.

**Chain-of-custody standard**: NAGPRA chain-of-custody documentation requires: what was found, where it was found, when it was found, and what its associated context was. The rename alone captures "what" and "when quarantined" but not "session context" or "associated git state."

**Smallest fix**: When quarantining a file, write a sidecar: `.git/index-<UUID>.abandoned-<timestamp>.provenance.txt` containing:
```
SESSION_ID: <UUID>
QUARANTINED_AT: <timestamp>
GIT_ROOT: <path>
BRANCH: $(git rev-parse --abbrev-ref HEAD 2>/dev/null)
LAST_MODIFIED: $(stat -f %Sm <orphan_path>)
ENTRY_COUNT: $(git ls-files --cached --index-output=<orphan_path> | wc -l 2>/dev/null || echo unknown)
```
Five lines of metadata. Permanent record.

---

### NVR-5 — P2 — Unidentifiable orphans receive no heightened caution

**File**: Plan section "Repair existing damage" (elf-revel, all 638 files)  
**Scenario**: Some elf-revel orphans were created by sessions that crashed in 2025. The UUID matches no current interlock session record. The session that created them is unidentifiable. NAGPRA's culturally-unidentifiable-remains protocol applies: items whose origin cannot be determined deserve extra caution (longer retention before disposition, separate classification category). The plan applies the same two-category (empty → delete, non-empty → quarantine) logic to all 638 files regardless of whether the session that created them is identifiable.

**Smallest fix**: Add a third classification to the sweeper: `UNIDENTIFIABLE` — orphan is non-empty AND was created more than 30 days before the session registry's earliest known session date. These files are quarantined with a separate retention period (90 days instead of 7 days) and flagged in the manifest for manual review.

---

### NVR-6 — P3 — First-deployment inverts pilot-first order

**File**: Plan section "Repair existing damage" — elf-revel (638 files) listed before mediumsetting (2 files) in the description  
**Note**: The plan lists repair targets in the order elf-revel (638) → mediumsetting (2) → others. The execution order should be inverted: mediumsetting first (2 files, easily verified by hand), then garden-salon (1), then Sylveste (5), then Lowbeer (1), then elf-revel (638). This is not a technical constraint — the plan could list them in any order. But the current ordering implies largest-first, which inverts the pilot-first principle.

**Recommendation**: Reorder the repair section to list targets from smallest to largest: `Lowbeer (1) → garden-salon (1) → mediumsetting (2) → Sylveste (5) → elf-revel (638)`. Each step validates the classifier on a corpus smaller than the next.

---

## Reframing

What the NAGPRA / Vermillion Accord lens reveals that standard engineering review misses:

**Standard engineering** asks: is the classifier correct? Does it handle edge cases? Are there tests?

**NAGPRA** asks: what is the disposition authority for each item? Has consultation occurred? Has provenance been documented? Has the protocol been validated on a controlled cohort before being applied at scale?

Three reframings:

1. **The 638-file corpus is not a test case — it is the primary archaeological site.** Engineering treats elf-revel's orphan files as a cleanup task: delete the debris. NAGPRA treats them as a collection of items with uncertain provenance, some of which may be culturally significant (contain irreplaceable staged work). The classifier is the only thing standing between the user's data and permanent loss. A novel classifier's first deployment should never be its largest deployment.

2. **"Empty" is a legal classification, not just a technical one.** Engineering defines empty as "no bytes" or "no entries." NAGPRA distinguishes: empty-by-contents, empty-by-significance, and empty-by-identification. An index whose entries match HEAD is empty-by-contents but not empty-by-significance — it encodes a session's terminal state, which is evidence. The plan's classifier must define its classification boundary explicitly, not leave it implicit in the implementation.

3. **Consultation before disposition is a protocol requirement, not a UX nicety.** The `--dry-run` flag is not a debugging convenience — it is the consultation step. Without it, the sweep runs without the responsible party having reviewed the intended actions. In NAGPRA terms, this is proceeding with disposition without consultation, regardless of how good the classifier is.

<!-- flux-drive:complete -->
