---
artifact_type: review-synthesis
method: flux-review
target: "ship plan for interlock git index isolation completeness fixes"
target_path: "/Users/sma/projects/Sylveste/interverse/interlock/docs/research/flux-review/interlock-ship-plan-review/2026-04-30-ship-plan.md"
tracks: 2
track_a_agents: [fd-semver-coordination, fd-cache-mirror-drift, fd-irreversible-action-discipline, fd-test-coverage-gaps, fd-repair-safety-protocol]
track_c_agents: [fd-venice-charter-restoration-ethics, fd-informed-consent-therapeutic-privilege, fd-nagpra-vermillion-repatriation, fd-nedcc-library-salvage-triage, fd-kintsugi-mended-vessel-honesty]
date: 2026-04-30
---

# Ship Plan Review — Synthesis

## Verdict on the Plan as Written

**Ship-with-modifications.** The plan's three-version arc is structurally sound — fix infrastructure (v0.2.14), harden wrappers (v0.2.15), document the unfixed-correctness gap (v0.2.16) — but it ships v0.2.14 with three load-bearing omissions that both tracks converged on independently: (1) no runtime stderr warning to disclose CF-1 to users who only reach v0.2.14, (2) no `--dry-run` / pilot-first protocol for the untested classifier that will be applied to 638 elf-revel files, and (3) no honest marker for the cache-mirror divergence between file content and version label. Each of these maps to a P0 in at least one track and to ethical/professional doctrine in the other. The plan is roughly 80% there; the missing 20% is structural, not cosmetic, and must land before execution.

## Critical Findings (P0/P1) — Must Fix Before Execution

### CF-1 disclosure failure in v0.2.14 — P0
**Surfaced by:** fd-informed-consent (P0 ICP-1), fd-nedcc-library-salvage-triage (P0 LST-1), fd-test-coverage-gaps (P1), fd-semver-coordination (P1, "stuck-at-intermediate").
**Plan change:** Add to `hooks/session-start.sh` in v0.2.14 a one-time-per-session stderr warning when concurrent index files are detected: `interlock: WARNING — N active index sessions detected. Concurrent commits may lose work (CF-1, unfixed). Serialize commits manually until v0.2.16+.` Plus a CHANGELOG.md "Known Issues" entry naming CF-1 with a link. Open question G is resolved: yes, ship the warning in v0.2.14.
**Relation to plan:** Plan currently defers this to v0.2.16's xfail test ("TBD" at line 63). Both bioethics (Canterbury/Montgomery materiality) and library-salvage doctrine (stabilize-before-defer) say this is non-negotiable. xfail is invisible to runtime users.

### Untested classifier on 638-file corpus — P0
**Surfaced by:** fd-repair-safety-protocol (P0), fd-venice-charter (P0 VCE-1), fd-nagpra-vermillion (P0 NVR-1), fd-nedcc (P1 LST-2), fd-kintsugi (referenced via P3 KIN-5), fd-test-coverage-gaps (P1 corpus-test gap). **6 of 10 agents converged.**
**Plan change:** (a) Mandate `--dry-run` flag on `scripts/interlock-orphan-sweep` that produces a per-item manifest without modifying anything; (b) split sweep into three explicit phases (survey → delete-empties → quarantine-non-empties) each requiring opt-in flag; (c) quarantine-only first pass on the elf-revel run — suppress the delete arm for 30 days of bake-in; (d) reorder repair targets smallest-first: Lowbeer (1) → garden-salon (1) → mediumsetting (2) → Sylveste (5) → elf-revel (638); (e) capture pre-sweep `tar.gz` snapshot of the elf-revel orphan corpus for 90-day forensic reference.
**Relation to plan:** Plan currently lists elf-revel first (638 files) and describes the procedure as a single shell pipeline with no checkpoint between classify and destroy. The destructive arm runs on an untested classifier — exactly the shape of failure that the original Track-C mortuary chain-of-custody warning at line 26-28 was meant to prevent.

### Cache mirror is identity-fraud-shaped — P0
**Surfaced by:** fd-cache-mirror-drift (P0 silent revert + P1 namespace collision), fd-venice-charter (P1 VCE-2 distinguishability violation), fd-kintsugi (P1 KIN-1 invisible weld), fd-informed-consent (P2 ICP-4 labeling integrity), fd-irreversible-action-discipline (P1 ask-gate). **5 agents converged.**
**Plan change:** (a) Drop `LOCAL-PATCHES-APPLIED.md` marker into `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/` listing patched files, source version, applied date, and removal condition; (b) capture upstream content hashes before patching and patched hashes after, store both, add a session-start integrity check that compares current cache against patched-hashes baseline and warns on revert; (c) add explicit ask-before-proceed gate for the cache write; (d) before patching, investigate whether Claude Code has a precedence-ordered override directory mechanism that would avoid mutating the cache at all.
**Relation to plan:** Plan line 73 claims "this machine is safe immediately"; that claim is unverified against marketplace re-sync, content-hash validation, or namespace collision when upstream eventually publishes its own v0.2.14. The label-content mismatch corrupts every future incident diagnosis on this machine.

### No concurrent-process test — P0
**Surfaced by:** fd-test-coverage-gaps (P0).
**Plan change:** Replace single-process `test_concurrent_commit_loss_xfail` with two real multi-process tests using `multiprocessing.Barrier` or `subprocess.Popen` pairs: `test_concurrent_pre_commit_loss` (CF-1) and `test_session_init_toctou` (CF-5). Mark xfail until fix lands. CF-1 and CF-5 are concurrency bugs by definition; a single-process test cannot exercise them.
**Relation to plan:** Plan's 6 tests are all single-session. The bug class that motivated v0.2.14 is not under test — only the implementation surface. v0.2.15's wrapper-hardening could introduce new races and CI would not catch them.

### Single-branch 3-commit bundle couples 3 review decisions — P1
**Surfaced by:** fd-irreversible-action-discipline (P1).
**Plan change:** Use stacked PRs (option 2): `fix/cf2-cleanup` first, once merged `fix/wrapper-hardening` rebased onto main, then `docs/design-note`. Each version gets its own merge gate. Alternative: one branch with verified-revert-coherence between commits.
**Relation to plan:** Plan line 67 packs three logically-independent decisions (v0.2.14, v0.2.15, v0.2.16) into one PR. sma cannot accept v0.2.14 and reject v0.2.15 without a rebase ceremony.

### v0.2.16 minor-version bump overclaims — P1
**Surfaced by:** fd-semver-coordination (P1).
**Plan change:** Pre-release tag the v0.2.16 bump (`0.2.16-design.0`) so semver-aware tooling does not auto-update; OR drop the version bump entirely and commit the design note + xfail to main on top of v0.2.15; OR ensure release notes' first paragraph explicitly says "no runtime code changes."
**Relation to plan:** Per semver.org, MINOR is functionality added in a backwards-compatible manner. A design note is not functionality. Auto-update consumers will read "newer = better" and may relax workarounds.

### Skip-upgrade compatibility unverified — P2
**Surfaced by:** fd-semver-coordination (P2).
**Plan change:** Add a "Skip-upgrade matrix" subsection to the plan covering 0.2.13→0.2.14, 0.2.13→0.2.15, 0.2.13→0.2.16, 0.2.14→0.2.16. Specify orphan-sweeper trigger model (auto on first hook invocation? user-initiated only?).

### Test 4 (env-var bypass) too narrow — P2
**Surfaced by:** fd-test-coverage-gaps (P2).
**Plan change:** Expand test 4 from one env-var to four: GIT_DIR, GIT_WORK_TREE, GIT_INDEX_FILE, and combinations. v0.2.15's strip-then-rev-parse pattern (line 51) covers all three; the test must match the structural change.

### `git reset --mixed HEAD` lacks pre-check — P1
**Surfaced by:** fd-repair-safety-protocol (P1).
**Plan change:** Pre-check `git diff --staged --name-only` before reset; refuse if non-empty; capture `PRE_RESET=$(git rev-parse HEAD)` for reflog-restore documentation.

### Scope-creep is renovation bundled with conservation — P1
**Surfaced by:** fd-venice-charter (P1 VCE-3).
**Plan change:** Either split v0.2.14 into v0.2.14a (stop.sh path reconstruction + atomic init) and v0.2.14b (sweeper + escape hatch), or at minimum keep them on separate commits so the sweeper can be reverted independently if it regresses.

## Cross-Track Convergence

Ranked by convergence score (track count × agent count). All 5 noted convergences verified; one additional convergence (#6) emerged from the agent files.

### Convergence #1 — Ship a CF-1 runtime stderr warning in v0.2.14 (highest confidence)
**Score:** 2/2 tracks, 4 agents. Adjacent: fd-test-coverage-gaps + fd-semver-coordination. Distant: fd-informed-consent + fd-nedcc.
**Track A framing:** "xfail test is invisible to runtime users; warning closes the failure-mode coverage gap until v0.2.16+. Stuck-at-intermediate users have no in-artifact signal of unfixed P0s."
**Track C framing:** Bioethics (Canterbury/Montgomery) — material risk a reasonable patient would want disclosed; therapeutic privilege does not apply because user can make an informed workaround choice. Library salvage triage — "deferral without stabilization is abandonment dressed as triage; freeze the wet book before moving to the next case."
**Synthesis:** Both tracks arrive at the same operational conclusion via different ethical frames. Engineering says "the test is invisible." Bioethics says "the consent is missing." Salvage doctrine says "the wet book is in standing water." All three are the same recommendation: a one-line `>&2 echo` in `hooks/session-start.sh` that fires when concurrent sessions are detected. **This is the single highest-leverage modification to the plan.**

### Convergence #2 — Untested classifier on 638 files needs --dry-run and pilot-first
**Score:** 2/2 tracks, 6 agents (Adjacent: fd-repair-safety-protocol. Distant: fd-venice-charter, fd-nagpra, fd-nedcc, fd-kintsugi referenced via quarantine-naming, fd-informed-consent.).
**Track A framing:** "The classifier is new and unvalidated. The destructive arm is irreversible if it has an off-by-one or empty-detection bug."
**Track C framing:** NAGPRA — items of uncertain provenance require consultation before disposition; novel protocols are piloted on small controlled cohorts before scale. NEDCC — separate disposition categories into separate physical operations with different chains of authorization. Venice Charter — interventions on irreplaceable material must be reversible or preceded by a complete documentary record. Bioethics — consent gate before intervention.
**Synthesis:** The distant track gives the strongest framing — these 638 files are not "stale debris," they are items of uncertain individual significance, some of which may contain the only copies of staged work. Mandate (1) `--dry-run` mode, (2) quarantine-only first pass for 30 days, (3) smallest-corpus-first ordering (mediumsetting before elf-revel), (4) sidecar provenance metadata (NVR-4) on quarantined files: SESSION_ID, GIT_ROOT, BRANCH, ENTRY_COUNT, (5) `tar.gz` snapshot of the corpus before any sweeper run.

### Convergence #3 — Cache mirror is identity-fraud-shaped, needs honest labeling
**Score:** 2/2 tracks, 5 agents (Adjacent: fd-cache-mirror-drift, fd-irreversible-action-discipline. Distant: fd-venice-charter, fd-kintsugi, fd-informed-consent.).
**Track A framing:** Operational risk — silent revert on resync, namespace collision when upstream publishes 0.2.14, no inspection mechanism so future-sma can't tell the patch is there.
**Track C framing:** Venice Charter — distinguishability is the central conservation principle; restored fabric must be visually distinguishable from original. Kintsugi — concealment is aesthetically and ethically inferior to conspicuous repair; the cache mirror is the plan's most invisible weld. Informed consent — FDA labeling doctrine: contents must match the label or the divergence must be conspicuously documented.
**Synthesis:** The distant track elevates this from "operational risk" to "ethical disclosure failure." Engineering would accept "fix it now, label it later." Conservation/labeling-integrity says: every future diagnosis on this machine is corrupted by the false version claim. The minimum acceptable seam is a `LOCAL-PATCHES-APPLIED.md` marker in the cache directory naming patched files, source version, date, and removal condition.

### Convergence #4 — Quarantine-only first pass + pilot-first ordering
**Score:** 2/2 tracks, 4 agents (Adjacent: fd-repair-safety-protocol, fd-irreversible-action-discipline. Distant: fd-nagpra, fd-nedcc.).
**Track A framing:** Make destruction reversible during bake-in; explicit checkpoints between classify and destroy; smallest-blast-radius first.
**Track C framing:** NAGPRA — classify-before-disturb; consultation-before-disposition. NEDCC — stabilize-vs-treat distinction; salvage ordering follows item significance, not item count.
**Synthesis:** This is the strongest answer to question D. Order of operations is now explicit:
1. Apply v0.2.14 fix.
2. Verify fix is active (kill all running claude sessions, smoke test in scratch repo).
3. Tar-snapshot the elf-revel orphan corpus for 90-day forensic retention.
4. Run `interlock-orphan-sweep --dry-run` on Lowbeer (1 file). sma reviews manifest. If correct, proceed to quarantine-only run.
5. Repeat for garden-salon (1), mediumsetting (2), Sylveste (5).
6. Only after the small-corpus runs pass clean: dry-run on elf-revel. sma reviews manifest. Then quarantine-only run on elf-revel — no deletes for 30 days.
7. After 30-day bake with no corruption reports: enable delete arm.

### Convergence #5 — Scope expansion is acceptable IF visible in changelog
**Score:** 1/2 tracks (distant only) — fd-kintsugi (P3 KIN-6), partial agreement from fd-venice-charter (P1 VCE-3 framed as conservation-vs-renovation).
**Distant framing:** kintsugi reframes question F from "are we over-engineering" to "are we narrating the discovery of deeper damage honestly." The scope expansion (one-line ask → 3-version plan) is the artifact's biography — the surface fix revealed CF-2, which revealed the orphan corpus, which revealed the need for the sweeper. The kintsugi answer: the expansion is dignified IF the changelog narrates it. The Venice Charter answer: minimum-necessary-action is a reversibility constraint, so split v0.2.14 into 14a/14b for independent rollback.
**Synthesis:** Both perspectives are honoured by adding a "History" section to v0.2.14's changelog: "What began as a one-line wrapper fix revealed four additional bugs (CF-1 through CF-5) and 647 orphan files across 5 repositories. This release fixes the lifecycle bugs (CF-2/3/4/5); CF-1 is documented as a known issue and tracked for v0.2.16." Plus the Venice Charter recommendation: keep sweeper as a separable commit so it can be reverted independently.

### Convergence #6 (newly surfaced) — v0.2.16 honest-lacuna is structurally sound but socially invisible
**Score:** 2/2 tracks, 4 agents (Adjacent: fd-test-coverage-gaps, fd-semver-coordination. Distant: fd-venice-charter VCE-5, fd-kintsugi KIN-3.).
**Track A framing:** xfail in test suite is invisible to users who don't run pytest; release-notes silence on CF-1 misleads auto-update consumers.
**Track C framing:** Venice Charter — honest lacuna requires the gap to be visible on the artifact's visible face, not buried in `tests/integration/`. Kintsugi — distinguishes technical visibility (the seam exists in the repo) from social visibility (the seam runs across the surface a normal reader encounters).
**Synthesis:** v0.2.16 is the plan's most defensible act — but only if the seam moves to the visible face. Add to README.md: a top-level "## Known Issues" section linking to the design doc, so a casual visitor sees the lacuna without reading the test suite.

## Domain-Expert Insights (Track A)

These required release-engineering specialist knowledge:

- **Marketplace cache invalidation triggers** (fd-cache-mirror-drift): manifest checksum verification, `claude plugin update` re-sync, cache GC, repair-on-load. The plan assumes immutable per-version cache; that assumption is unverified.
- **Semver protocol on design-note-only releases** (fd-semver-coordination): minor-bump-without-functionality is a misinformation channel for auto-update consumers; pre-release tags or no-bump are the correct tools.
- **TOCTOU + multiprocessing test infrastructure** (fd-test-coverage-gaps): single-process `pytest` cannot exercise CF-1 or CF-5 race conditions; `multiprocessing.Barrier` + `subprocess.Popen` pairs are the right primitive.
- **`git reset --mixed HEAD` semantics around concurrent staged state** (fd-repair-safety-protocol): silently moves staged-but-uncommitted work to working tree; recoverable via reflog within 90 days, but the plan applied without precondition check.
- **Stacked PR vs single-branch coupling** (fd-irreversible-action-discipline): three logically-independent decisions on one branch couple three accept/reject points into one mechanical commitment.
- **GitHub Actions verification by absence** (fd-irreversible-action-discipline): mechanically confirmed `.github/workflows/` contains only ci.yml + secret-scan.yml — no auto-merge, no auto-publish — and recommended documenting that verification.
- **Real-corpus orphan classifier failure modes** (fd-test-coverage-gaps + fd-repair-safety-protocol): macOS APFS case-folding, partial-write truncation, mode-only changes, deleted-tracked-file staging, indexes-identical-to-HEAD — all edge cases the synthetic-fixture test cannot cover.

## Structural Insights (Track C)

The reframings the distant lenses produced. Each describes what the lens specifically lets you SEE that engineering does not.

### Bioethics / informed consent (fd-informed-consent)
**What the lens sees:** disclosure timing relative to harm-onset. The xfail test is technically present but does not satisfy the materiality standard because the harm (data loss) manifests at point-of-use, not at point-of-test-suite-execution. **Mechanism:** Canterbury v. Spence + Montgomery v. Lanarkshire establish that disclosures must be in a form a reasonable person would encounter at the moment they have a meaningful choice. The xfail test is not in that form for users; it is in that form only for maintainers. Therapeutic privilege (the implicit argument behind question G) is also rejected: it requires immediate serious psychological harm, which a stderr warning does not produce.
**What engineering doesn't see:** that the warning is not a UX-noise tradeoff — it is the structural requirement that converts shipping CF-1 from "non-disclosure of a material risk" to "disclosed risk the user can choose to accept or work around."

### Library salvage triage (fd-nedcc)
**What the lens sees:** stabilization-vs-treatment as distinct operations. **Mechanism:** salvage doctrine: when you cannot fully treat a damaged item immediately, you must stabilize it first (freeze the wet book) before moving on. Otherwise the deferral is not deferral, it is abandonment. CF-1 in this plan is being moved to v0.2.16 (treatment) but never stabilized in v0.2.14 (no warning, no concurrent-session detection). The wet book stays in standing water for the entire window between v0.2.14 ship and v0.2.16 ship.
**What engineering doesn't see:** that "deferred to v0.2.16" and "stabilized for v0.2.14" are not the same thing. Engineering tends to see a known issue with a tracking ticket as adequately handled. Salvage triage says: if active deterioration continues during the deferral window, the item is not properly triaged.

### NAGPRA / Vermillion Accord (fd-nagpra)
**What the lens sees:** items-of-uncertain-provenance requiring case-by-case review before bulk action. **Mechanism:** the Vermillion Accord requires consultation before disturbance of remains of uncertain origin. In the software analogue, the 638 elf-revel orphans are not "files" in the sense engineering uses; they are items whose individual significance cannot be determined without inspection. NAGPRA's culturally-unidentifiable-remains protocol applies extra caution to items whose origin cannot be determined. The plan applies the same two-category disposition to all 638 regardless of identifiability.
**What engineering doesn't see:** that "delete on the basis of automated classification" is a category-level decision that requires the classifier itself to be validated, not just the destination of each classified item. Pilot-first is not best-practice — it is the specific protocol that prevents a bad classifier from destroying the only copy of irreplaceable material at scale.

### Venice Charter restoration ethics (fd-venice-charter)
**What the lens sees:** distinguishability of restored fabric from original. **Mechanism:** the 1964 Venice Charter requires that any restoration be visually distinguishable from the original material so that future scholars can read the intervention history. The cache mirror violates this directly: patched files claim 0.2.13 identity. **What this lens lets you see:** the cache mirror is not a pragmatic shortcut, it is an authenticity violation. Every future incident diagnosis on this machine that consults `plugin.json` for version will be corrupted by the false claim. The bug is not an operational risk that might bite later — it is a permanent corruption of the artifact's documentary record from the moment the patch is applied.
**What engineering doesn't see:** that "this machine is safe immediately" is true only in the runtime-behavior sense and false in the diagnosability sense. Engineering optimizes runtime behavior; conservation optimizes the legibility of the artifact's history.

### Kintsugi (fd-kintsugi)
**What the lens sees:** technical visibility vs social visibility of seams. **Mechanism:** kintsugi distinguishes between a repair mark that exists in the artifact (technical visibility) and a repair mark that runs across the surface a normal reader encounters (social visibility). The xfail test is technically visible (it lives at `tests/integration/test_concurrent_commit_loss.py`) and socially invisible (CI shows green; no normal user-path of reading the artifact passes through that file). Similarly the cache mirror is technically detectable (file diff against upstream) and socially invisible (the directory listing shows no marker).
**What engineering doesn't see:** that the version-bump-for-design-note-only is actually kintsugi-eligible — naming the break and making it part of the artifact's official history is a dignified act IF the seam runs across the visible face. Engineering tends to see "failing test + no fix" as a liability; kintsugi sees it as the most defensible act in the plan, provided the disclosure surfaces in README/CHANGELOG, not just the test directory.

## Direct Answers to Questions A–H

**A. Sequencing risk — is v0.2.14 → v0.2.15 → v0.2.16 right?**
The shape is right; the labeling is wrong. v0.2.14 must include the CF-1 stabilization warning AND a "Known Issues" CHANGELOG section. v0.2.15 may proceed as designed. v0.2.16 should pre-release-tag (`0.2.16-design.0`) or skip the version bump entirely; the README must add a top-level "Known Issues" section. With these changes, sequencing is sound.

**B. Cache-mirror correctness when marketplace ships an actual 0.2.14?**
Drop a `LOCAL-PATCHES-APPLIED.md` marker file. Capture upstream content hashes before patching and patched hashes after. Add a session-start integrity check that warns if the cache reverts. Document an explicit decommission step: once upstream 0.2.14 is verified to ship the same fix, `rm -rf` the patched 0.2.13 directory to avoid namespace collision. Best of all: investigate whether Claude Code has a precedence-ordered override mechanism that avoids cache mutation entirely.

**C. v0.2.16 design-note-only honesty — appropriate?**
Yes — IF the seam runs across the visible face. Plan as written buries the disclosure in xfail and changelog silence. With the README "Known Issues" section, the v0.2.14 stderr warning, and release-notes language that explicitly says "no runtime code changes," v0.2.16 becomes the most defensible act in the plan (kintsugi-eligible: naming the break as part of the artifact's official history).

**D. Repair-orphans-while-fixing-cause coupling — using untested classify-before-delete on 638 files?**
No. Sequence:
1. Ship v0.2.14 (with CF-1 warning).
2. Quit all running claude sessions; verify cache mirror active in fresh session.
3. Tar-snapshot the elf-revel orphan corpus.
4. Validate classifier on a synthetic golden-fixture corpus (10–20 indexes with known-correct classification).
5. Dry-run on Lowbeer (1) → garden-salon (1) → mediumsetting (2) → Sylveste (5). sma reviews each manifest before quarantine-only execution. NO deletes on these runs.
6. Dry-run on elf-revel. sma reviews manifest. Quarantine-only execution. NO deletes.
7. After 30-day bake with no corruption reports: enable delete arm.
This is convergence #4 fully unpacked.

**E. Test-strategy adequacy — what's missing?**
Three new tests minimum: `test_concurrent_pre_commit_loss` (multiprocessing.Barrier, CF-1), `test_session_init_toctou` (multiprocessing.Barrier, CF-5), `test_orphan_sweeper_on_real_corpus_sample` (20-orphan sample copied to tmp). Plus expansion of test 4 from one env-var to four. Plus a structural-test assertion that `scripts/interlock-orphan-sweep` exists, sources lib.sh, and uses `session_index_is_empty()`. Total: 6 → 11 tests.

**F. Scope-creep risk — is this over-engineering?**
The kintsugi reframing is the right answer: the expansion is the discovery of deeper damage during repair, and the question is whether the changelog narrates that honestly. Yes ship the full scope, AND name the expansion in v0.2.14's changelog History section. Plus the Venice Charter recommendation: keep the sweeper on a separable commit so it can be reverted independently of the path-reconstruction fix.

**G. Branch-don't-push-main — is PR-against-self ceremony?**
Not pure ceremony for *this* plan. The branch contains 3 logically-independent commits with non-trivial diff. Stacked PRs (fix/cf2-cleanup → fix/wrapper-hardening → docs/design-note) are the right structure. Per fd-irreversible-action-discipline: this couples three independent accept/reject decisions into one if you single-branch it. Also add an explicit ask-gate for the cache-mirror write and the orphan-delete steps — Sylveste/CLAUDE.md irreversibility doctrine applies to both.

**H. Failure-mode breadth — what does the plan miss?**
- CI verification (no auto-merge / no auto-publish workflows) confirmed by inspection but not documented in the plan.
- Cache-resync triggers: unverified.
- Skip-upgrade compatibility (0.2.13→0.2.16 leapfrog): not analyzed.
- Concurrent-process test infrastructure: missing entirely.
- Real-corpus orphan filename quirks (Unicode, case-folding, partial-write truncation, mode changes): not in the synthetic-fixture test.
- Pre-condition check on `git status` before `git reset --mixed HEAD`: missing.
- Decommission step for the cache-mirror once upstream catches up: missing.

## Revised Ship Plan

Numbered, ready to execute:

### Phase 0 — Stabilization (before any commits)

1. **Verify GitHub Actions safety:** confirm `.github/workflows/` contains only `ci.yml` and `secret-scan.yml` (no auto-merge/publish). Document the verification in the plan.
2. **Investigate cache-override mechanism:** check whether Claude Code supports a precedence-ordered override directory (e.g., `~/.claude/plugins.local/`) that would obviate cache mutation. If yes, prefer this; the rest of the cache-mirror plan changes if so.

### Phase 1 — v0.2.14 (lifecycle hotfix + CF-1 stabilization)

3. **Branch:** `fix/cf2-cleanup` off `main` @ `ffe8129`.
4. **Files (revised):**
   - `hooks/stop.sh` — reconstruct SESSION_INDEX from `$SESSION_ID + $GIT_ROOT`.
   - `hooks/lib.sh` — add `session_index_is_empty()` with explicit semantic: returns true iff `git ls-files --cached` reports zero entries. Define "clean" (entries == HEAD) as a separate classification, not collapsed into "empty."
   - `hooks/session-start.sh`:
     - Atomic init (`$SESSION_INDEX.tmp` + fsync + rename).
     - `INTERLOCK_DISABLE_INDEX_ISOLATION=1` escape hatch with deprecation comment naming removal target (v0.3.0) and a stderr warning at use.
     - **NEW: CF-1 stabilization warning.** When `$ACTIVE_SESSIONS > 1`, `>&2 echo "interlock: WARNING — N active index sessions detected. Concurrent commits may lose work (CF-1, unfixed). See docs/design/2026-04-30-cross-session-reconciliation.md for status. Set INTERLOCK_SUPPRESS_CF1_WARN=1 to silence."`
   - `scripts/interlock-orphan-sweep` — new sweeper with **mandatory `--dry-run` flag** that produces per-item manifest. Three explicit phases (survey / delete-empties / quarantine-non-empties), each opt-in. Sidecar provenance metadata file written when quarantining.
   - `.claude-plugin/plugin.json` — bump to 0.2.14.
   - **NEW: `CHANGELOG.md`** — entry with "Fixed" (CF-2/3/4/5), "Known Issues" (CF-1 with workaround and link), and "History" (one paragraph narrating the scope expansion from one-line ask).
   - **NEW: `README.md`** — top-level "## Known Issues" section linking to design doc.
5. **Tests (revised, 11 total):**
   - `test_stop_reconstructs_session_index_path`
   - `test_session_init_is_atomic`
   - `test_orphan_sweeper_classifies_before_delete`
   - `test_env_var_bypass_GIT_DIR`
   - `test_env_var_bypass_GIT_WORK_TREE` (NEW)
   - `test_env_var_bypass_GIT_INDEX_FILE` (NEW)
   - `test_env_var_combinations` (NEW)
   - `test_disable_index_isolation_escape_hatch`
   - `test_concurrent_pre_commit_loss` (NEW, multiprocessing, xfail until CF-1 fixed)
   - `test_session_init_toctou` (NEW, multiprocessing)
   - `test_orphan_sweeper_on_real_corpus_sample` (NEW, 20 anonymized real orphans)
6. **Commit + push branch.** Do not merge. **Ask sma before merge.**

### Phase 2 — v0.2.15 (wrapper hardening)

7. **Branch:** `fix/wrapper-hardening` stacked off `fix/cf2-cleanup` once #6 lands.
8. Wrapper-hardening changes per original plan (env-strip, --git-dir, zsh detection).
9. `plugin.json` bump to 0.2.15.
10. CHANGELOG.md update (still includes CF-1 known-issue section).
11. Commit + push branch. Ask before merge.

### Phase 3 — v0.2.16 (honest lacuna)

12. **Branch:** `docs/design-note` stacked off `fix/wrapper-hardening` once #11 lands.
13. Files: `docs/design/2026-04-30-cross-session-reconciliation.md`, `tests/integration/test_concurrent_commit_loss.py` (xfail with issue-link annotation).
14. **Version decision:** pre-release tag `0.2.16-design.0` (preferred) OR no-bump (commit on top of 0.2.15 tag).
15. Release notes open with: "This release contains no runtime code changes. It publishes a design document and a failing test for CF-1. CF-1 remains unfixed."
16. Commit + push branch. Ask before merge.

### Phase 4 — Cache mirror (with honest labeling)

17. **Capture upstream baseline:** `cd ~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13 && find . -type f -exec sha256sum {} \; > /tmp/0.2.13-upstream.sums && cp -r . ../0.2.13.upstream-backup`.
18. **Ask sma before proceeding.** Show diff to be applied.
19. Apply v0.2.14 + v0.2.15 patches.
20. **Capture patched baseline:** `find . -type f -exec sha256sum {} \; > /tmp/0.2.13-patched.sums`.
21. **Drop marker:** `LOCAL-PATCHES-APPLIED.md` listing patched files, source version, applied date, removal condition.
22. **Add integrity check** to `hooks/session-start.sh` (or a separate startup script): compare current cache hashes against `/tmp/0.2.13-patched.sums`; warn on revert.
23. Skip mirroring v0.2.16 (design-note-only).

### Phase 5 — Repair (with safety protocol)

24. **Verify fix is active:** kill all running claude sessions; start a fresh session in a scratch repo; perform a commit; confirm no `.git/index-<UUID>` orphan accumulates.
25. **Tar-snapshot the elf-revel orphan corpus** for 90-day forensic retention: `tar czf /tmp/elf-revel-orphan-corpus-$(date +%s).tar.gz $(find /path/to/elf-revel/.git -name 'index-*' -not -name '*.abandoned*')`.
26. **Validate classifier on synthetic golden corpus** (10–20 fixture indexes with known-correct classification).
27. **Dry-run + quarantine-only on small repos in order:** Lowbeer (1) → garden-salon (1) → mediumsetting (2) → Sylveste (5). For each: dry-run, sma reviews manifest, quarantine-only execution. NO deletes.
28. **Pre-check for `git reset --mixed HEAD`:** refuse if `git diff --staged --name-only` is non-empty. Capture `PRE_RESET=$(git rev-parse HEAD)`.
29. **Dry-run on elf-revel.** sma reviews manifest. **Ask before proceeding.**
30. **Quarantine-only on elf-revel.** No deletes for 30 days.
31. After 30-day bake with no corruption reports: re-evaluate enabling delete arm.

### Confirmations encoded in this revised plan

- **v0.2.14 includes a runtime stderr warning for CF-1?** Yes (step 4, hooks/session-start.sh new block).
- **Sweeper has --dry-run mode and pilot-first execution?** Yes (step 4 mandatory flag; steps 27/29/30 pilot-first ordering).
- **Cache mirror gets a LOCAL-PATCHES-APPLIED.md marker?** Yes (step 21).
- **Tests now include concurrent-session, upgrade-path, chaos coverage?** Concurrent-session yes (steps 5: test_concurrent_pre_commit_loss + test_session_init_toctou). Upgrade-path: skip-upgrade matrix added to plan as documentation. Real-corpus sample test added.
- **Changelog narrative for v0.2.14 / v0.2.15 / v0.2.16?** v0.2.14 includes "Fixed" + "Known Issues (CF-1)" + "History"; v0.2.15 maintains the "Known Issues" section; v0.2.16 release notes open with "no runtime code changes" disclosure.
- **Other modifications:** stacked-PR structure replaces single branch; ask-gates added for cache write, sweeper execution, and `git reset`; pre-check on staged state before reset; sidecar provenance metadata for quarantined orphans; tar-snapshot for forensic retention; integrity check against cache reverts.

## Synthesis Assessment

**Did the distant track produce qualitatively different insights this time, or restate the same issues?**

Qualitatively different. Three of the five distant agents produced reframings that the adjacent track could not have produced from its position:

1. **fd-nedcc** introduced the stabilization-vs-treatment distinction. Adjacent agents identified the missing warning (fd-test-coverage-gaps, fd-semver-coordination) but framed it as "test invisibility." The salvage-triage frame — "deferral without stabilization is abandonment dressed as triage" — converts the same operational gap into a doctrinal failure with a name. That's not restatement; it's a different category of error.

2. **fd-venice-charter** elevated the cache mirror from "operational risk" to "permanent corruption of the documentary record." Adjacent fd-cache-mirror-drift identified silent-revert and namespace-collision as runtime failures. The Venice frame says: even if the runtime is fine forever, every future incident diagnosis is corrupted by the false version label. That's an additional load-bearing concern, not a duplicate.

3. **fd-kintsugi** distinguished technical from social visibility of seams. Adjacent agents could see "xfail is invisible to runtime users." Kintsugi names *why* — that the seam runs along the back of the vessel. This let the synthesis recommend specifically *where* the disclosure must move (README, CHANGELOG, release notes' first paragraph) rather than just *that* it must move.

The remaining two distant agents (fd-informed-consent, fd-nagpra) restated similar concerns to the adjacent track but with sharper framing — informed consent gave the precise legal doctrine (Canterbury/Montgomery materiality + therapeutic-privilege rebuttal) that closes question G; NAGPRA gave the pilot-first-on-smallest-corpus protocol that converts "good practice" into "doctrinal requirement."

**Was 2 tracks the right call?**

Yes. A single-track adjacent review would have produced the operational fixes (stderr warning, dry-run flag, marker file) but not the framing that makes them non-negotiable. The framing matters because the original ship plan's open questions C, F, and G all read as "is this acceptable?" — and the adjacent track answers "this is missing test coverage / has a coverage gap." The distant track answers "this fails informed-consent doctrine / library-salvage triage / Venice Charter distinguishability." The second framing is harder to talk yourself out of when reviewing your own plan at 2 AM. The 2-track structure earned its cost.

**The single highest-leverage modification — the one change without which the plan stays broken even if everything else gets done:**

**Ship the CF-1 stabilization warning in v0.2.14.** Without it, every other fix is downstream of a plan that knowingly ships silent data loss with no disclosure to users. The warning is one `>&2 echo` line in `hooks/session-start.sh`. It costs ~5 lines of bash. It is the freeze that converts "deferred to v0.2.16" from abandonment into honest triage. It is the disclosure that converts "non-disclosure of material risk" into "consented-to risk the user can work around." It is the seam that converts the cache-mirror situation from invisible-weld to acknowledged-repair. Without this single change, the plan is correct in shape but fails the load-bearing ethical test that all 4 of the most independent reviewers (across both tracks) converged on.
