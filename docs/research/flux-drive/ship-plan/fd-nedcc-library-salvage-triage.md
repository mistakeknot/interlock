### Findings Index
- P0 | LST-1 | "v0.2.14 — Lifecycle hotfix" | CF-1 is in active deterioration: plan defers without stabilizing — deferral-without-stabilization is abandonment dressed as triage
- P1 | LST-2 | "Repair existing damage" | Non-empty orphans processed by identical disposition rule as empties — blurs triage categories in violation of salvage doctrine
- P1 | LST-3 | "v0.2.16 — DESIGN-NOTE-ONLY" | v0.2.16 deferral is acceptable only if CF-1 was stabilized first; the plan offers no stabilization step
- P2 | LST-4 | "Open questions for review" | No salvage-leader named: decision authority for irreversible actions is diffuse
- P2 | LST-5 | "v0.2.14 → v0.2.15 → v0.2.16" | Serial sequencing when parallel treatment would close the CF-1 disclosure gap faster
- P3 | LST-6 | "Repair existing damage" | Cost-benefit analysis absent for 638-file sweep: high-cost intervention on items of uncertain individual value
Verdict: needs-changes

## Summary

Through the NEDCC library salvage triage lens, the plan's most critical error is applying deferral-without-stabilization to CF-1. Salvage doctrine distinguishes two operations: stabilization (halt active deterioration) and treatment (full restoration). Freezing a wet book halts mold; treating the water damage comes later. The plan ships v0.2.14 (which fixes CF-2/3/4/5) but leaves CF-1 — concurrent-commit silent data loss — in active deterioration with no stabilization measure. A v0.2.14 user who doesn't reach v0.2.16 has been handed a partially-treated collection: four damaged items dried, one still in standing water. NEDCC triage requires that even deferred items be stabilized before the triage crew moves to the next case.

## Issues Found

### LST-1 — P0 — CF-1 deferred without stabilization: deferral-without-stabilization is abandonment

**File**: `hooks/session-start.sh` (v0.2.14 change), plan section "v0.2.14 — Lifecycle hotfix," plan section "v0.2.16 — DESIGN-NOTE-ONLY"  
**Failure scenario**: A library conservator encounters a wet book alongside four mold-damaged but dry books. The dry books are treated first (CF-2/3/4/5). The wet book is photographed, a design note is written about how to dry it, and a failing test is prepared to confirm it is still wet. The wet book remains in standing water. This is the plan's treatment of CF-1.

In the concrete failure mode: user upgrades to v0.2.14. Two Claude Code sessions are running. Session A's pre-commit fires and acquires the mkdir lockfile. Session B's pre-commit also runs (mkdir-based locking is not flock — two sessions can race the lock acquisition on some filesystems). Session B's commit is registered by git; Session A's staged state was incorporated into the index silently; both believe they committed. On next `git log`, only one commit appears with the merged state — or worse, one commit appears and the other's changes are absent entirely. There is no warning.

**NEDCC triage standard**: The wet book must be frozen before the team moves to the next case. Stabilization for CF-1 means: emit a stderr warning at session-start (or at pre-commit) when a concurrent session is detected, so the user knows to serialize commits manually. This does not require fixing CF-1; it halts the active deterioration by making the hazard visible.

**Smallest fix**: Add to `hooks/session-start.sh`, inside the wrapper-install block after v0.2.14 changes land, a concurrent-session detection check:
```bash
# [v0.2.14 CF-1 stabilization] Warn if another session is active in this repo
ACTIVE_SESSIONS=$(ls "${GIT_ROOT}/.git/index-"* 2>/dev/null | grep -v "\.abandoned" | wc -l | tr -d ' ')
if [[ "$ACTIVE_SESSIONS" -gt 1 ]]; then
    >&2 echo "interlock: WARNING — ${ACTIVE_SESSIONS} active index sessions detected. Concurrent commits may lose work (CF-1, unfixed). Serialize commits manually until v0.2.16+."
fi
```
This is ~5 lines in session-start.sh. It fires at session start, not in a hot loop. It does not fix CF-1; it freezes the wet book.

---

### LST-2 — P1 — Non-empty orphans processed by same disposition rule as empties

**File**: `scripts/interlock-orphan-sweep` (new), plan section "Repair existing damage"  
**Failure scenario**: The sweeper classifies orphans into two categories: empty → delete, non-empty → quarantine as `.abandoned-<ts>`. Both categories are processed in the same sweep run, in the same pass, with the same loop. NEDCC triage separates disposition categories into separate physical operations with different chains of authorization: "empties to destruction bin," "non-empties to specialist review shelf." When both run in the same pass, a mis-classification (a non-empty file classified as empty) results in deletion rather than quarantine, with no checkpoint between the two operations.

**Salvage doctrine**: Items of different triage classes should not be processed in the same physical operation. The wet books go to the freezer; the mold-damaged books go to a different room. Processing both in one pass with a common classifier means that classifier errors produce destruction rather than misdirection.

**Smallest fix**: Split the sweep into two sequential phases within the script:
1. **Phase 1 — Survey only**: Enumerate all orphans, classify each, write manifest to stdout. Exit. No deletions, no renames.
2. **Phase 2 — Delete empties** (requires `--delete-empties` flag): Read manifest, delete only files classified as empty.
3. **Phase 3 — Quarantine non-empties** (requires `--quarantine-nonempty` flag): Read manifest, rename classified non-empty files.

The three-phase structure means that classification and disposition are separate operations, a mis-classification produces only an incorrect manifest entry (not a deletion), and each destructive phase requires explicit opt-in. This is ~20 lines of bash restructuring.

---

### LST-3 — P1 — v0.2.16 deferral acceptable only with prior stabilization

**File**: Plan sections "v0.2.16 — DESIGN-NOTE-ONLY" and "v0.2.14 — Lifecycle hotfix"  
**Scenario**: v0.2.16 ships as design-note-only. This is defensible salvage triage — acknowledging that the treatment is not yet available while documenting the damage. NEDCC accepts deferral. But NEDCC's criterion for acceptable deferral is: the deferred item must be stabilized. The plan does not stabilize CF-1 before deferring it to v0.2.16. Without LST-1's stabilization step, v0.2.16 is not "honest deferral" — it is abandonment of a deteriorating item with documentation that its deterioration is being tracked.

**Note**: This finding is not about whether v0.2.16 is honest. It is about whether v0.2.16 is sufficient. The honest-lacuna principle (endorsed by Venice Charter, kintsugi) says: document what is unrepaired. Salvage triage adds: and stabilize what is actively deteriorating while you document.

**Smallest fix**: v0.2.16 is fine as designed — but only if LST-1 (the CF-1 warning) ships in v0.2.14. The fix is to make LST-1's fix a prerequisite for v0.2.16's ship. If LST-1 is deferred, v0.2.16 must include the stabilization step instead.

---

### LST-4 — P2 — No salvage-leader named

**File**: Plan sections "Execution mechanics," "Open questions for review"  
**Scenario**: Open questions A through H in the plan span irreversible decisions: whether to add the stderr nag, whether to run the orphan sweep, which orphan repair order to use. The plan presents these as questions "for review" — implying a committee deliberation. NEDCC triage requires a conservator-in-charge who makes final calls. Committee deliberation under time pressure (638 files of uncertain provenance, a plugin with live users) produces decision paralysis or lowest-common-denominator choices.

**Salvage doctrine**: The plan should name a decision authority for each open question. Since this is a single-maintainer project (sma), that person is sma — but the plan should explicitly close each question with a decision, not leave them open indefinitely.

**Smallest fix**: Resolve each open question in the plan with a recommended answer before execution begins. Q G especially must be resolved: "Does v0.2.14 include the CF-1 stderr warning? Answer: Yes." The other questions can remain advisory, but Q G and Q D (repair orphans) are load-bearing.

---

### LST-5 — P2 — Serial sequencing when parallel treatment would close the gap faster

**File**: Plan sections "v0.2.14," "v0.2.15," "v0.2.16" (sequential)  
**Scenario**: Salvage protocols run treatments in parallel where dependencies allow. The plan serializes all three versions. The design note and warning could be added to v0.2.14 without waiting for v0.2.16 — the design doc is a markdown file, not code. The CF-1 warning (a `>&2 echo`) could also ship in v0.2.14. The current plan exposes users to the un-warned CF-1 risk for the entire window between v0.2.14 ship and v0.2.16 ship. If that window is days, acceptable. If it is weeks or months, the deferral compounds the harm.

**Recommendation**: Move the CF-1 warning and the design doc into v0.2.14. Ship the test (xfail) in v0.2.15 alongside the wrapper hardening. v0.2.16 then becomes: "full fix." This eliminates the stabilization gap without restructuring the fix sequence. The version count stays at 3; only the content of each version changes.

---

### LST-6 — P3 — Cost-benefit absent for 638-file sweep

**File**: Plan section "Repair existing damage" (elf-revel)  
**Note**: NEDCC triage requires a cost-benefit assessment before high-cost interventions on items of uncertain value. The plan does not assess: what is the probability that any of the 638 files contains non-recoverable work? What is the cost of a false-positive deletion? What is the benefit of cleanup (disk space, index confusion reduction)? For 2 files in mediumsetting, the cost is trivially low and no assessment is needed. For 638 files in elf-revel, a brief assessment ("CF-2 was a lifecycle bug; sessions that terminated normally will have empty indexes; the likely non-empty count is N < 638") would justify the sweep cost.

**Recommendation**: Add one paragraph to the "Repair existing damage" section estimating how many of the 638 files are likely non-empty given CF-2's failure mode. This informs the decision about whether to run the sweep immediately or defer to a manual review of the non-empties.

---

## Reframing

What the NEDCC library salvage triage lens reveals that standard engineering review misses:

**Standard engineering** asks: does the fix sequence address all the bugs? Are the tests adequate?

**NEDCC salvage triage** asks: which items are in active deterioration? Which can safely wait? Has stabilization been separated from treatment? Is the deferred item sitting in standing water?

Three reframings:

1. **CF-1 is not deferred — it is abandoned in standing water.** Engineering categorizes CF-1 as "known issue, fix deferred to v0.2.16." Salvage triage categorizes it as "active deterioration with no stabilization measure." The distinction is operational: a deferred item that continues to cause active harm (every concurrent commit loses work until v0.2.16) is not properly deferred — it is inadequately triaged. Proper triage would freeze it first (warning) and treat it later (full fix).

2. **Deferral-without-stabilization is the most common form of institutional abandonment.** The plan has impeccable documentation of what is deferred. NEDCC's case files are full of items with excellent documentation of why treatment was deferred — and photographs taken 20 years later showing the item has disintegrated. Documentation is not a substitute for freezing the wet book. The warning in v0.2.14 is the freeze.

3. **The salvage-leader role is implicit in this plan but needs to be explicit.** The open questions section ends without resolution. This is not a failure of thinking — it is a failure of decision authority. NEDCC's most important lesson from major salvage operations is: appoint a conservator-in-charge on day one, and that person makes the calls. For this plan, sma is the conservator-in-charge. The plan should name that role and close the open questions, especially Q G and Q D.

<!-- flux-drive:complete -->
