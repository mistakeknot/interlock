### Findings Index
- P0 | ICP-1 | "v0.2.14 — Lifecycle hotfix" | v0.2.14 ships with no warning for CF-1: users upgrade into a known silent-data-loss bug with no disclosure
- P1 | ICP-2 | "Repair existing damage" | Untested sweeper applied to 638 elf-revel orphans without documented consent gate
- P1 | ICP-3 | "v0.2.16 — design-note-only" | xfail test is buried disclosure: technically present, practically invisible — fails materiality standard
- P2 | ICP-4 | "Cache mirror" | Cache mirror is a labeling-integrity violation: contents do not match the declared version
- P2 | ICP-5 | "v0.2.16 — design-note-only" | Version bump for design-note-only sends a misleading signal of forward progress on CF-1
- P3 | ICP-6 | "Open questions for review" | Therapeutic-privilege argument (Q G: "users might panic") is named but not resolved; plan should pre-empt it explicitly
Verdict: needs-changes

## Summary

Through the informed-consent lens, the plan has a clear P0 failure: v0.2.14 ships a plugin that continues to silently lose work on concurrent commits (CF-1 unfixed) with no warning to users who upgrade. Under Canterbury v. Spence and Montgomery v. Lanarkshire, material information is what a reasonable patient — here, a user who relies on interlock to protect their git state — would want before consenting to the treatment. A user who installs v0.2.14 does not know they are still exposed to CF-1. The plan acknowledges this as an open question (Q G: stderr nag) but does not resolve it. The bioethics framework requires that the warning be present at the point of upgrade — not in a failing test that ships months later in v0.2.16.

## Issues Found

### ICP-1 — P0 — v0.2.14 ships with no CF-1 warning: informed-consent failure with irreversible harm

**File**: `hooks/session-start.sh` (v0.2.14 version), `.claude-plugin/plugin.json` (0.2.14 release), plan section "v0.2.14 — Lifecycle hotfix"  
**Failure scenario**: User has two Claude Code sessions open in the same repo. They upgrade to v0.2.14 which fixes CF-2, CF-3, CF-4, CF-5. They trust the plugin is now correct. Session A and Session B both commit. Session B's pre-commit hook silently loses Session A's staged work (CF-1, the mkdir-based lockfile race). The user discovers missing commits hours or days later. Because v0.2.14 has no stderr warning about CF-1, the user had no information at the point of upgrade that would have led them to work around the bug (e.g., never have two sessions open simultaneously).

**Consent analysis**: This is non-disclosure of a material risk. Under Canterbury, materiality is what a reasonable person would want to know — and "your git history may silently lose work if you use two sessions at once" is clearly material for a git-coordination plugin. The plan defers this disclosure to v0.2.16 (months later), which itself buries the disclosure in an xfail test. There is no consent.

**Is therapeutic privilege available?** The plan implicitly raises this in Q G ("should v0.2.14 add a stderr nag"). The therapeutic-privilege argument would be: "warning users about CF-1 will cause them to disable the plugin, and they'll lose even more protection." This argument fails under modern bioethics (Montgomery v. Lanarkshire explicitly rejected it) because: (a) the user has a right to make an informed choice even if the clinician thinks they'll choose wrong; (b) "disable the plugin" is a valid response to an unfixed data-loss bug; (c) the privilege is only available when disclosure would cause serious, immediate psychological harm — a stderr warning does not meet this threshold.

**Smallest fix**: Add to `hooks/session-start.sh` (top of the wrapper install block, after CF-2/CF-3/CF-4 fixes land):
```bash
# [v0.2.14] CF-1 known issue: concurrent commits from multiple sessions may lose
# staged work. See docs/design/2026-04-30-cross-session-reconciliation.md.
# Workaround: avoid concurrent commits from separate sessions until v0.2.16+.
>&2 echo "interlock: WARNING — concurrent commit data-loss (CF-1) is known and unfixed. See interlock docs for status."
```
One `>&2 echo` line. Session start is the right injection point — the warning fires when the isolation is installed, before the user can be harmed.

---

### ICP-2 — P1 — Untested sweeper applied to elf-revel without documented consent gate

**File**: Plan section "Repair existing damage" (elf-revel, 638 files), `scripts/interlock-orphan-sweep` (new)  
**Failure scenario**: The repair plan says "Apply classify-before-delete to existing orphans" in elf-revel. The sweeper is new and untested at this scale. The plan does not describe a consent step where the user reviews the manifest of intended deletions before execution. An untested classifier deletes an orphan that contained the only copy of a staged patch. The user had no opportunity to inspect the manifest before deletion.

**Consent analysis**: In clinical terms, this is performing an intervention without first explaining the material risks and obtaining consent. The user (sma) is both the clinician and the patient here — but the protocol should still require an explicit review step before irreversible action. Material information includes: "this protocol is untested at this scale," "some files may contain staged work you cannot recover," and "the classifier has not been validated against a controlled corpus." None of this is documented in the plan as a pre-action disclosure.

**Smallest fix**: Before running the sweep on elf-revel, add an explicit gate to the execution plan: "Run `interlock-orphan-sweep --dry-run` on elf-revel. Review the output manifest. Explicitly confirm before running live." This is documentation, not code — one bullet point added to the "Repair existing damage" section.

---

### ICP-3 — P1 — Buried disclosure: xfail test fails materiality standard

**File**: `tests/integration/test_concurrent_commit_loss.py` (v0.2.16), plan section "v0.2.16 — DESIGN-NOTE-ONLY"  
**Failure scenario**: v0.2.16 ships. A user who reads changelogs (rather than test suites) sees "bump to 0.2.16, add design doc." They do not notice the xfail test. They continue using the plugin assuming the issue is being tracked but not actively harming them. In fact CF-1 is still live and the only disclosure is a test marked `xfail` in a CI output that shows all-green.

**Consent analysis**: Buried disclosure is distinct from non-disclosure but fails the same materiality standard. Under Canterbury, disclosure must be in a form a reasonable person will encounter. Most users read changelogs, READMEs, and release notes. Almost none read `xfail` test files. The xfail test is technically-visible but practically-invisible disclosure — a consent form hidden on page 47 of a 50-page insurance contract.

**Smallest fix**: v0.2.16 changelog entry must explicitly state: "CF-1 (concurrent commit silent data loss) is known, unfixed, and documented. See docs/design/2026-04-30-cross-session-reconciliation.md. The full fix is deferred." One paragraph. The design doc exists; the changelog must point to it prominently.

---

### ICP-4 — P2 — Cache mirror is a labeling-integrity violation

**File**: `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/` (all patched files), `plugin.json` (version stays 0.2.13)  
**Scenario**: During an incident, someone checks `plugin.json` version, gets `0.2.13`, cross-references the 0.2.13 release for the stop.sh behavior, draws false conclusions. The label-content mismatch produces diagnostic errors.

**Labeling-integrity analysis**: FDA labeling doctrine holds that contents must match the label — either the label changes or the divergence must be conspicuously documented on the packaging. Here, "conspicuously documented" means a marker file that any reader will find when examining the cache directory.

**Smallest fix**: `echo "v0.2.14+v0.2.15 patches applied $(date -u +%Y-%m-%dT%H:%M:%SZ)" > ~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/.LOCAL-PATCHES-APPLIED`

---

### ICP-5 — P2 — Version bump for design-note sends misleading signal

**File**: `.claude-plugin/plugin.json` (0.2.16 bump), plan section "v0.2.16 — DESIGN-NOTE-ONLY"  
**Scenario**: A user with auto-update enabled sees the 0.2.16 bump. They assume a fix landed. They relax their workaround (avoiding concurrent commits). CF-1 damages their work.

**Labeling analysis**: A version bump is a signal of progress. "New formula" on a drug package implies the formula was improved. If the new version contains only a design note and a failing test — and the failing test produces green CI output via xfail — the version signal is misleading. This is not a reason to not bump; it is a reason to make the release notes unambiguous that 0.2.16 contains no runtime fix.

**Smallest fix**: 0.2.16 release notes must include in the first paragraph: "This release contains a design document and a failing test for CF-1. No runtime code changes. CF-1 remains unfixed."

---

### ICP-6 — P3 — Therapeutic-privilege temptation should be explicitly pre-empted in the plan

**File**: Plan section "Open questions for review," question G  
**Note**: Q G asks whether v0.2.14 should add a stderr nag. The plan does not answer it. The implicit risk is that someone reviewing the plan makes the therapeutic-privilege argument ("the warning will scare users") and it goes uncontested.

**Recommendation**: Resolve Q G in the plan with an explicit answer: "Yes, the CF-1 stderr warning is required. Therapeutic privilege does not apply because: (a) concurrent-commit data loss is a material risk any reasonable user would want disclosed; (b) the user can make an informed choice to work around CF-1 or accept the risk; (c) no immediate serious harm results from reading the warning. The warning ships in v0.2.14."

---

## Reframing

What the informed-consent lens reveals that standard engineering review misses:

**Standard engineering** asks: does the fix work? Are the tests sufficient? Is the scope appropriate?

**Informed consent** asks: does the user have the information they need to make a meaningful choice at every point where their interests are at risk?

Three reframings:

1. **The stderr nag is not optional — it is an ethical requirement.** Engineering review treats Q G as a UX tradeoff: does the warning help, is it too noisy? Bioethics treats it as non-negotiable: you cannot ship a plugin that silently loses user work without disclosing the risk at the point of use. "The information is in the test suite" does not satisfy the materiality standard any more than "the warning is in the package insert" satisfies it when the harm manifests at point of use.

2. **The xfail test is not a disclosure — it is a record.** A test exists for the codebase's benefit. A disclosure exists for the user's benefit. The plan conflates these. Disclosures must be in a form the user will encounter: release notes, changelogs, READMEs, or in-process warnings. An xfail test in an integration test directory is not a form the user will encounter.

3. **The 638-file sweep is an intervention that requires consent, not just competence.** Engineering focuses on whether the sweep is correct. Bioethics focuses on whether the user was informed of the risks before the intervention proceeded. "The protocol is untested at this scale" is material information that the user must receive before the sweep runs — not as documentation after the fact, but as a pre-action gate.

<!-- flux-drive:complete -->
