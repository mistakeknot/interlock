# Flux Drive Review — 2026-04-30-ship-plan

**Reviewed**: 2026-04-29 | **Agents**: 5 launched, 5 completed | **Verdict**: needs-changes

---

## Verdict Summary

| Agent | Verdict | Summary |
|-------|---------|---------|
| fd-venice-charter-restoration-ethics | needs-changes | Untested sweeper on 638 files is irreversible; cache mirror is an authenticity violation; scope over-bundled |
| fd-informed-consent-therapeutic-privilege | needs-changes | v0.2.14 ships CF-1 with no warning; xfail test is buried disclosure; 638-file sweep lacks consent gate |
| fd-nagpra-vermillion-repatriation | needs-changes | No dry-run mode; empty-classification boundary undefined; bulk action on largest corpus first |
| fd-nedcc-library-salvage-triage | needs-changes | CF-1 deferred without stabilization; non-empties and empties processed identically |
| fd-kintsugi-mended-vessel-honesty | needs-changes | Cache mirror is invisible weld; changelogs risk hiding CF-1; xfail seam is socially invisible |

---

## Critical Findings (P0)

### P0-1 — Untested sweeper on 638-file corpus (4/5 agents)

`scripts/interlock-orphan-sweep` + `hooks/lib.sh:session_index_is_empty()` + plan "Repair existing damage"

Novel classify-before-delete protocol first deployed against the largest corpus (638 files). Classification boundary error silently deletes staged work that was never committed. No dry-run mode; no pilot on small corpus (mediumsetting 2 files) required first.

**Fix**: `--dry-run` flag + pilot on mediumsetting before elf-revel.

---

### P0-2 — CF-1 deferred without stabilization (2/5 agents)

`hooks/session-start.sh` (v0.2.14) + "v0.2.14 — Lifecycle hotfix" + "v0.2.16 — DESIGN-NOTE-ONLY"

CF-1 (concurrent-commit silent data loss) is in active deterioration. v0.2.14 fixes CF-2/3/4/5 but ships no warning about CF-1. User who upgrades to v0.2.14 is fully exposed with no disclosure. Design doc deferred to v0.2.16 (months later).

**Fix**: 5 lines in session-start.sh detecting concurrent sessions and emitting `>&2 echo "interlock: WARNING — CF-1..."`.

---

## Important Findings (P1) — 8 total

1. **No `--dry-run` mode in sweeper** (fd-nagpra, fd-nedcc): 638-file sweep with no per-item manifest review gate
2. **Empty-classification boundary undefined** (fd-nagpra): entries-identical-to-HEAD is not archaeologically empty
3. **xfail test is buried disclosure** (fd-informed-consent, fd-kintsugi): CI shows green; fails materiality standard
4. **Cache mirror is identity/labeling violation** (fd-venice, fd-informed-consent, fd-kintsugi): `LOCAL-PATCHES-APPLIED.md` marker required
5. **v0.2.14 scope over-bundled** (fd-venice): 4 interventions = 1 rollback unit; sweeper regression reverts CF-2 fix
6. **v0.2.14/v0.2.15 changelogs not constrained to name CF-1** (fd-kintsugi, fd-venice): "Known Issues" section required
7. **Non-empties and empties in same sweep pass** (fd-nedcc, fd-nagpra): three-phase structure needed
8. **v0.2.16 is not honest deferral without prior CF-1 stabilization** (fd-nedcc): design-note-only without freeze = abandonment

---

## Improvements (P2/P3) — 9 total

- Pilot order: Lowbeer(1) → garden-salon(1) → mediumsetting(2) → Sylveste(5) → elf-revel(638)
- Quarantine sidecar provenance file: SESSION_ID, GIT_ROOT, branch, entry count
- Name decision-maker (sma) and close Q G and Q D before execution
- Serial-to-parallel: design doc + CF-1 warning could ship in v0.2.14
- README/KNOWN-ISSUES.md for CF-1 (social visibility for the seam)
- In-file provenance comments on v0.2.14 changes in session-start.sh
- v0.2.16 release notes must explicitly state no runtime fix was shipped
- Classification reason encoded in quarantine filename (`.abandoned-cf2-empty-<ts>`)
- Forensic snapshot of elf-revel corpus before any sweep runs

---

## Cross-Agent Reframings (Unique to Distant Track)

1. **"Deferral without stabilization is abandonment dressed as triage"** (fd-nedcc): The CF-1 warning is not UX — it is the freeze step that legitimizes v0.2.16 deferral.
2. **"Cache mirror is an authenticity fraud"** (fd-venice, fd-kintsugi): Engineering sees pragmatic shortcut; conservation sees false identity label corrupting all future diagnosis.
3. **"638 files are items of uncertain provenance, not debris"** (fd-nagpra): Some may be the only copy of irreplaceable work. The consultation step (dry-run + review) is not optional polish.
4. **"Scope creep is the artifact's biography"** (fd-kintsugi): The expansion from one-line fix to 3 versions is the discovery of deeper damage during repair. Q F's framing is wrong — the question is not whether the scope is appropriate but whether the expansion is visible and dignified in the changelog.
5. **"An xfail test is a record, not a disclosure"** (fd-informed-consent): The materiality standard requires encountering the disclosure in a normal path (changelog, README) — not in a test file CI marks green.

---

## Files
- Summary: `docs/research/flux-drive/ship-plan/summary.md`
- Findings: `docs/research/flux-drive/ship-plan/findings.json`
- fd-venice-charter-restoration-ethics.md
- fd-informed-consent-therapeutic-privilege.md
- fd-nagpra-vermillion-repatriation.md
- fd-nedcc-library-salvage-triage.md
- fd-kintsugi-mended-vessel-honesty.md
