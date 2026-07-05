# fd-cache-mirror-drift — Ship Plan Findings

**Reviewer:** fd-cache-mirror-drift (distribution systems / package mirror lens)
**Target:** `docs/research/flux-review/interlock-ship-plan-review/2026-04-30-ship-plan.md`
**Date:** 2026-04-30
**Project:** interlock plugin cache patching at `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/`
**Lens:** Cache invariants, version-label accuracy, sync triggers, namespace collisions

---

## Summary

Three findings. One P0 (silent revert when marketplace publishes its own 0.2.13 hash refresh, or when a `claude` CLI plugin sync triggers cache replacement). One P1 (namespace collision when upstream publishes its own v0.2.14 alongside the locally-patched 0.2.13). One P2 (no inspection mechanism — user cannot verify "what version am I actually running" against the in-place patch).

The cache directory layout I observed at `~/.claude/plugins/cache/interagency-marketplace/interlock/` contains separate per-version subdirectories: `0.2.11/`, `0.2.12/`, `0.2.13/`. This confirms that Claude Code installs each marketplace version into its own directory rather than overwriting in place — so an upstream v0.2.14 publish would create `0.2.14/` and leave the patched `0.2.13/` intact. **However**, that observation does NOT confirm what happens when the marketplace re-publishes the same `0.2.13` tag (e.g., a re-tag, a manifest hash refresh, or a forced cache invalidation). The plan assumes immutable per-version cache; that assumption is unverified.

---

## P0: Marketplace re-sync of 0.2.13 silently reverts the in-place patch

**Location:** Ship plan §"Cache mirror (local stopgap)" (lines 72-74), specifically:
> Mirror v0.2.14 + v0.2.15 changes into `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/` so this machine is safe immediately. Skip mirroring v0.2.16 because design-note-only. plugin.json on the cache stays at 0.2.13 (don't bump local version, just patch files in place).

**Failure scenario:** sma patches the local `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/hooks/stop.sh` and `hooks/session-start.sh` and `scripts/interlock-orphan-sweep` with v0.2.14 content. The patched files have content hashes that differ from the upstream-published 0.2.13. Several plausible triggers can re-sync the cache:

1. **Manifest checksum verification.** If Claude Code's plugin loader periodically re-validates cache entries against the marketplace manifest's content-hash, the patched files will fail validation and be redownloaded — silently reverting the fix.
2. **`claude plugin update` or marketplace pull.** Any user-initiated or scheduled refresh of the marketplace catalog could re-fetch 0.2.13 and overwrite the patch.
3. **Cache cleanup / GC.** If Claude Code has a cache GC that deletes and re-fetches old versions, the patched 0.2.13 entry is in the firing line.
4. **Repair-on-load.** If the loader detects "modified files in cache" and treats it as corruption, it could heal back to the published manifest.

After the silent revert, the user runs sessions that produce orphan files again. They observe — perhaps weeks later — that elf-revel has another 100 orphans. They re-investigate, only to discover the cache patch has been overwritten.

**Why P0:** This is silent data-loss reintroduction. The user explicitly took an action to fix the bug on this machine; the cache subsystem invisibly undid that action.

**What the plan does not specify:**
- Whether Claude Code uses content hashing or version-label-only matching (line 74 implies the plan assumes version-label-only).
- Whether the patch survives a `claude` restart, a system reboot, a plugin marketplace refresh.
- Any verification step ("re-check that the patch is still in place after N hours / after restart").

**Smallest viable fix:**
1. Before patching, capture upstream 0.2.13 content hashes (e.g., `sha256sum hooks/*.sh > /tmp/0.2.13.upstream.sums`).
2. After patching, capture local hashes (`sha256sum hooks/*.sh > /tmp/0.2.13.patched.sums`).
3. Add a startup hook or a periodic check (in this user's shell rc, or a launchd / systemd timer) that compares current cache file hashes against `/tmp/0.2.13.patched.sums`. If they revert to upstream, alert.
4. Better: patch in `~/.claude/plugins/<some-override-dir>/` if Claude Code supports a precedence-ordered override directory, rather than mutating the cache. The plan should investigate whether Claude Code has such a mechanism (`~/.claude/plugins.local/` or similar) before mutating the cache.

**Question to verify before execution:**
- Does `claude` have any background marketplace-sync behavior?
- Is the cache directory write-protected after install, or freely writable?
- Does Claude Code's plugin loader validate cache content against a content hash, a version label, or neither?

---

## P1: Upstream v0.2.14 publish + local 0.2.13 patch creates namespace collision with no resolution policy

**Location:** Ship plan §"Cache mirror" (lines 72-74) and Open question B (line 107):
> Cache-mirror correctness — local code diverges from claimed plugin.json version. What happens when marketplace ships an actual 0.2.14?

**Failure scenario:** sma patches local `0.2.13/` with v0.2.14 file content. Time passes. Upstream publishes its own v0.2.14 (perhaps a different person, perhaps a future sma after losing context on the patch). Claude Code on this machine pulls the marketplace catalog and installs upstream v0.2.14 to `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.14/`. Now the directory tree contains:

```
~/.claude/plugins/cache/interagency-marketplace/interlock/
├── 0.2.11/        (upstream)
├── 0.2.12/        (upstream)
├── 0.2.13/        (LOCAL PATCH — files match v0.2.14 source, plugin.json says 0.2.13)
├── 0.2.14/        (upstream — files match upstream's v0.2.14 source)
```

**The collision:**
- `0.2.13/hooks/stop.sh` and `0.2.14/hooks/stop.sh` both contain v0.2.14 logic. If they are bit-identical, no problem; if upstream's v0.2.14 differs from sma's local patch (different formatting, different escape-hatch behavior, different orphan-sweeper path), the user has two divergent "v0.2.14" implementations.
- Whichever version Claude Code activates depends on the marketplace.json or cache-resolver logic. If it activates 0.2.14, the local patch was wasted. If it activates 0.2.13, the user gets the local patch but never benefits from upstream's actual v0.2.14 (which may include unrelated fixes).
- The local patch's plugin.json claims 0.2.13. Tooling that selects the "highest version available" picks 0.2.14 from upstream. Tooling that pins to 0.2.13 picks the local patch. This is operator-confusing.

**Why P1:** The state is recoverable (delete `0.2.13/` to fall through to upstream `0.2.14/`), but it's a foot-gun the plan does not address. It is also a future-tense ambiguity: the bug accumulates over time as the patch ages.

**What the plan does not specify:**
- Whether `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/` should be deleted manually as a precondition once an upstream v0.2.14 is published.
- Whether plugin.json in the local 0.2.13 patch should mention "this is a local override of upstream 0.2.13."
- Whether sma should leave a marker file (e.g., `.LOCAL_PATCH_README` in 0.2.13/) so future-sma can identify the patch.

**Smallest viable fix:**
1. Add to the cache mirror step: drop a marker file `~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/.LOCAL_PATCH_INFO` containing the patch date, the bug being patched (CF-2 etc.), and the removal condition ("delete this directory once upstream 0.2.14 is verified to ship the same fix").
2. Add a section to the plan: "Decommissioning the local patch" — an explicit instruction that, once upstream 0.2.14 is published and verified, the user must `rm -rf ~/.claude/plugins/cache/interagency-marketplace/interlock/0.2.13/` to avoid namespace collision.

---

## P2: No inspection mechanism — user cannot verify "what version am I actually running"

**Location:** Ship plan §"Cache mirror", lines 72-74. The phrase "this machine is safe immediately" (line 73) is unverifiable from the user's perspective.

**Failure scenario:** sma applies the patch on Tuesday. On Friday, sma is debugging an orphan-related issue in elf-revel and wants to confirm the patch is still active. The plugin.json reports 0.2.13. Running `claude /clavain doctor` or any plugin diagnostic shows 0.2.13. There is no command that says "the runtime hooks differ from the upstream 0.2.13 manifest by N files; you are running a local patch."

This is not a critical bug — just an observability gap that becomes a real problem six months later when sma has forgotten the patch exists and observes "weird" behavior on this machine compared to other machines.

**Smallest viable fix:**
1. Drop a `.LOCAL_PATCH_INFO` file (per P1 fix) — also serves as inspection.
2. Optional: stderr nag from `hooks/session-start.sh` on every session start: "[interlock] Running local patch dated YYYY-MM-DD; upstream version 0.2.13 reported by plugin.json." One line. Honest.
3. Optional: extend `scripts/interlock-check.sh` (which exists at `scripts/interlock-check.sh`) to compare hook file hashes against a known baseline and report drift.

---

## P3 (mention): Skip mirroring v0.2.16 is correct *if and only if* v0.2.16 truly contains no runtime changes

**Location:** Ship plan line 74: "Skip mirroring v0.2.16 because design-note-only."

**Concern (low-severity):** The xfail test in v0.2.16 is described as living at `tests/integration/test_concurrent_commit_loss.py`. If the local cache test runner picks up tests from the cache directory (rather than the source tree), or if any tooling iterates over cached plugin tests, the cache will not have the xfail and will produce a different test set than the source. This is unlikely to matter in practice because plugin caches typically don't run plugin tests, but the plan should verify.

**Why P3:** The tests are not the runtime path of the plugin. Plugin loaders activate hooks/MCP servers, not test files. The decision to skip mirroring v0.2.16 is correct.

---

## What I did NOT review (per agent boundaries)

- Whether bumping plugin.json from 0.2.13 to 0.2.14 is semver-correct — fd-semver-coordination.
- Branch / push / PR mechanics for source repo — fd-irreversible-action-discipline.
- Test coverage adequacy of the proposed v0.2.14 test suite — fd-test-coverage-gaps.
- Sweeper safety on real orphan files — fd-repair-safety-protocol.

---

## Decision summary

The cache mirror is a reasonable stopgap *if and only if* three things are true:
1. The cache directory is durable across `claude` restarts and marketplace catalog refreshes (P0 risk: unverified).
2. The patch is recoverable / re-appliable if it is silently overwritten (P0 mitigation).
3. The patch is observable so future-sma can find it (P2 mitigation).

The plan currently has zero of those three guarantees. Before executing the cache mirror step, sma should either:
- (a) Investigate Claude Code's cache invalidation triggers and document them, then add an integrity-check mechanism, OR
- (b) Use a different override mechanism (override directory, environment variable that points to a custom plugin path, or a wrapper shell that intercepts hook invocations) that does not depend on cache durability.

If (a) and (b) are both too much work, the smallest acceptable change is to add the `.LOCAL_PATCH_INFO` marker file and a hash-baseline file the user can re-check, plus a runtime stderr nag so the patch is visible at session start.

The "this machine is safe immediately" claim should be downgraded to "this machine is safer until the next cache resync, whose timing is unverified."
