# Compound Advisory-Only Pattern Analysis

## Summary

This document records the analysis performed while creating the solution document at
`/root/projects/Interverse/plugins/interlock/docs/solutions/2026-02-16-advisory-only-timeout-eliminates-toctou.md`.

## What Was Examined

### Source Files
- `internal/client/client.go` — Contains the advisory-only `CheckExpiredNegotiations` implementation (lines 377-472), the idempotent `ReleaseByPattern` with `isNotFound` guard (lines 353-367), the typed `IntermuteError` struct (lines 143-152), and the `isNotFound` helper (lines 574-580).
- `internal/tools/tools.go` — Contains the `negotiate_release` tool handler (lines 340-389) that delegates to the client layer.
- `hooks/pre-edit.sh` — Contains the feature-flagged advisory release-request display (lines 65-89) gated by `INTERLOCK_AUTO_RELEASE=1`.
- `CLAUDE.md` and `AGENTS.md` — Confirm the advisory-only design decision is documented as canonical.

### Flux-Drive Reviews Referenced
- `docs/research/correctness-review-of-implementation.md` — Identified the goroutine leak (B4) and duplicate force-release processing.
- `docs/research/quality-review-of-implementation.md` — Flagged structural concerns with the timeout pattern.
- `docs/research/correctness-review-of-diff.md` — Confirmed fix correctness.

## Key Findings

### 1. The Four Bugs Share a Single Root Cause
All four P0 bugs (TOCTOU race, non-idempotent DELETE, consent violation, goroutine leak) trace back to one design choice: a background goroutine that writes to shared mutable state based on stale reads. Removing the writes (advisory-only) eliminates B1, B3, and B4 simultaneously. Making remaining deletes idempotent via typed error detection fixes B2.

### 2. Advisory-Only Is a General Concurrency Pattern
The insight generalizes: any background process that "detects a condition then mutates state on behalf of another actor" is vulnerable to TOCTOU races. Converting it to "detect and report" eliminates the race entirely because read-only code has no interleaving hazards. The tradeoff is latency (the owning actor must respond), which is acceptable when sub-second enforcement is not required.

### 3. Plan-Stage Review Is Disproportionately Effective
All four bugs were identified by flux-drive reviewers reading the plan document before any code was written. The correctness reviewer traced the full TOCTOU race narrative from the plan's prose description alone. This confirms: reviewing plans catches architectural bugs at a fraction of the cost of reviewing (or debugging) implementation code.

### 4. Idempotent DELETE Is the Complementary Pattern
Even after removing background writes, explicit user-triggered releases can still race (two agents call `respond_to_release` concurrently). The idempotent DELETE pattern — treating 404 as "already done" via `errors.As` on typed `*IntermuteError` — handles this cleanly. These two patterns together (advisory-only + idempotent DELETE) provide complete coverage for the coordination system's concurrency hazards.

## Output

Solution document written to:
`/root/projects/Interverse/plugins/interlock/docs/solutions/2026-02-16-advisory-only-timeout-eliminates-toctou.md`

The document follows YAML-frontmatter format with sections: Problem, Root Cause, Solution, Key Insight, Reusable Pattern, and Related. It is 89 lines (under the 100-line target) and focuses on the reusable patterns rather than interlock-specific implementation details.
