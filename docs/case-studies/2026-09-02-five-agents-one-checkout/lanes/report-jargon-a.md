# Report: lane jargon-a

## Repos done: 28/28 (all repos in order-jargon-a.txt)

No repo was skipped entirely. Every repo was reserved, grepped, and either edited+committed or confirmed clean.

Full per-repo detail is in `progress-jargon-a.tsv`. Summary:

- **19 repos fully clean** (no residual check failures, or only out-of-scope/identifier residuals): intercache, intercept, interchart, intercheck, intercraft, interdeep, interdeploy, interdev, interform, intergraph, interhelm, interknow, interlearn, interleave, interlens, interlock, interloop, intermap, intermix.
- **9 repos partial** — edited everything in scope, but left a documented, deliberate residue: interdoc, interfluence, interflux, interject, interkasten, interlab, interline, interlore, intermem.

## What "partial" means, concretely

Two distinct reasons, both explained per-repo in the progress file and never silent:

1. **`docs/plans/*.md` blocked by a local `guard-plan-needs-card.sh` PreToolUse hook** (an Autarch product-card discipline check, unrelated to interlock/intermute/the jargon task). It fires on any Edit-tool write to a plan doc in a repo with no `docs/why.md`, regardless of what the edit is. Hit in 8 repos: interdoc (5 files), interfluence (2), interflux (1), interject (5), interkasten (3), interlab (1), interline (1), intermem (2) — roughly 20 files total. I did not attempt the override env var or draft product cards; both are out of scope for a mechanical jargon sweep. See friction log.
2. **Deliberately-left "beads"/"bd" and some "PHILOSOPHY.md"/"Clavain" mentions where the term is the actual accurate subject matter, not internal jargon standing in for something generic.** Concretely: interkasten and interlab genuinely integrate with the real open-source Beads issue tracker (interlock's own docs even cite `github.com/steveyegge/beads`); interlore's entire product is generating PHILOSOPHY.md files (exact parallel to a README-generator saying "README.md"); several research docs (interlens, intermap's fd-architecture-review.md) use lowercase `clavain` consistently as a plugin-name identifier alongside `interlock`/`interflux`, or quote another file's literal text. The brief's own exception ("if a remaining hit is an identifier a program consumes, leave it and say so") covers these; I extended the same reasoning to beads/PHILOSOPHY where the check script doesn't even test for those terms.

Two of the largest repos (interflux: 39 files, intermem: 16 files, intermap: 17 files) had 15–50+ Clavain/Demarch/path instances each, mostly in historical/frozen research and review artifacts. Handled via reviewed bulk `sed` substitution (case-sensitive on capital-`Clavain` only, which cleanly separated prose from the lowercase `os/clavain/`, `.clavain/`, `clavain:` identifier patterns used throughout those same docs) rather than hundreds of individual Edit calls, then a full diff read to catch and fix grammar breaks (sentence-initial lowercase, missing possessive `'s`) before committing.

## Coordination stats

- Reservations: 28 repos × up to 4 patterns each, all succeeded (usually on the first try).
- Real conflicts: 1 (interhelm — `README.md` and `kimi.plugin.json` held by `sweep-release`).
- `negotiate_release` calls: 2, both for that one conflict — both returned `NOT_FOUND` because the holder had already released by the time the call landed. A bare retry of `reserve_files` succeeded immediately afterward.
- Release-requests received via `fetch_inbox` and acknowledged with `respond_to_release`: 2 (`interdoc/README.md` from `sweep-ci`, `interfluence/README.md` from `sweep-scaffold`) — both already released on my end by the time the request arrived, so both were a `released: 0` no-op ack.
- Commits: 26 (2 repos — intercache-family repos with zero matches — had nothing to commit).

## Three most annoying things about coordinating

1. **`fetch_inbox` has no unread/cursor semantics.** It re-delivers the *entire* accumulated message history — almost all of it other lanes' commit broadcasts fanned out to all ~18 registered agents — on every single call, for the life of the session. By repo 8 of 28 it had grown past the tool's own output-size limit and started erroring with a "saved to file" message instead of returning inline; from there on, every repo required saving-and-re-parsing a growing JSON blob just to check for the one or two message types (`release-request`) that were ever actually relevant to me. This is pure overhead that scales with session length and agent count, and it dominated the token cost of the whole sweep — more than the actual file edits did.
2. **Conflicts can be stale by the time the protocol tells you to act on them.** `reserve_files` reported a real conflict (interhelm), and I followed the protocol straight to `negotiate_release` — but the reservation had already cleared in the few hundred milliseconds it took to make that call, so `negotiate_release` just errored `NOT_FOUND`. The fix that actually worked (bare retry of `reserve_files`) isn't in the documented protocol at all; the protocol assumes conflicts are durable enough to negotiate, but short-lived reservations (someone finishing up) make negotiation a dead end.
3. **A completely unrelated local hook silently ate ~20 files of scope with no coordination-layer visibility.** `guard-plan-needs-card.sh` has nothing to do with interlock/intermute — it's a separate Autarch product-card policy — but it fires on the same Edit tool call and blocks purely on file path (`docs/plans/*`), independent of what the edit does or how small it is. There's no way to see this coming from the coordination tools' side (reservations succeed fine; the block only surfaces mid-edit), and no way to satisfy it without either drafting a full product card or invoking an override meant for real planning decisions — both disproportionate to a one-line jargon fix.

Full verbatim entries for all of the above are in `friction-jargon-a.md`.
