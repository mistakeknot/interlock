# Shared-Filesystem Coordination

Status: **0.2.16 ships tier 1 (this doc's "stop the bleed" + shared-FS pivot).**
Tiers 2–3 are roadmap, gated on experiments (see §4).

## 1. Why the model changed

Interlock ≤0.2.15 gave every joined session its own `git worktree` under
`~/.cache/interlock/worktrees/`. This was added (commit `f1c79a2`) to fix a real
stealth-revert / index-pollution bug where a stale per-session `GIT_INDEX_FILE`
could commit a peer's changes as phantom deletes
(`docs/solutions/git/stealth-revert-via-multi-agent-index-pollution-20260505.md`).

Two problems made it a net liability:

1. **It leaked unboundedly.** Creation was unconditional per session; the only
   cleanup (`stop.sh`) *retained* worktrees by design, and the planned TTL
   sweeper was never built. `git gc` auto-prunes the worktree admin entries
   during normal operation, severing tracking and leaving orphaned directories
   on disk. Observed: ~9.7 GB / 164 dirs accumulated on one machine.
2. **It was self-contradictory.** Agents isolated in private worktrees *cannot
   collide*, which makes the file-reservation layer — interlock's actual value —
   coordinate nothing. We paid the full cost of isolation *and* coordination
   while getting the benefit of neither. The isolation was also redundant with
   consuming projects' own discipline (e.g. elf-revel's `session-spawn.sh`).

**Decision:** drop per-session worktrees entirely; make interlock a genuine
shared-filesystem coordinator. All agents work in the one real checkout;
collisions are *prevented* (not merge-resolved) by reservations.

## 2. The shared-FS safety model (tier 1, shipped 0.2.16)

All sessions edit the same working tree. Three existing mechanisms, now actually
load-bearing because there is shared state to contend over:

- **Edit-time reservation** (`hooks/pre-edit.sh`): before an edit lands, the hook
  reserves the target file (atomic `ic coordination reserve`, intermute HTTP
  fallback). If a peer holds an exclusive reservation, the edit is blocked
  (`decision:block`); otherwise the file is auto-reserved for this agent
  (15-min auto-renewing TTL).
- **Commit serialization** (`commit_lock_path` → `.git/commit.lock`): the
  pre-commit git hook acquires an `mkdir`-based lock so two agents can't race a
  commit.
- **Commit-time reservation check** (pre-commit hook): blocks committing a file
  another agent has reserved — a second line of defense behind the edit-time gate.

Failure posture: **fail-open**, consistent with interlock's existing ethos
(`trap 'exit 0' ERR`, `--max-time 2` circuit breakers, silent degradation when
intermute/`ic`/jq are unavailable). A dead coordination server must never freeze
an agent's edits. The cost is that during an outage the clobber-prevention is
advisory only.

### Orphan self-healing

`scripts/interlock-orphan-sweep.sh` reclaims worktrees leaked by ≤0.2.15. It is
invoked best-effort, backgrounded, once-per-day-per-machine from
`session-start.sh`. Safety contract (verified by test):

- clean + HEAD reachable from a ref → `git worktree remove` (admin entry too)
- **dirty** (uncommitted changes) → **quarantined** to `<base>/.quarantine`, never deleted
- clean but holding **unreachable/detached commits** → **quarantined**
- empty orphan dir (admin already gc-pruned) → removed
- non-empty orphan dir → quarantined

## 3. Why filename-level reservation isn't enough (the tier-2/3 motivation)

Glob/filename reservation answers "are these two edits to the same *path*?" It
does **not** answer "do these two edits *actually conflict*?" Two agents editing
opposite ends of a 2000-line file — one in the auth logic up top, one in a test
at the bottom — collide on the filename but not in content. Filename locking
either over-blocks (serializes work that didn't need it) or, with non-exclusive
reservations, under-protects. The thing that makes shared-FS genuinely *better*
than worktree-per-agent is **content-aware** conflict detection.

## 4. Roadmap: tiered semantic conflict detection

Cheapest-first; each tier only runs when the prior tier says "maybe." This keeps
the expensive tiers off the hot path for ~99% of edits.

**Tier 1 — glob overlap (shipped).** intermute/`ic` reservation check. Free.
Only escalate when a filename overlap with a *live peer reservation* exists.

**Tier 2 — embedding similarity (roadmap).** On overlap, embed the two edit
regions (or the reserving agent's stated objective vs. this edit's target) with
the already-local `intersearch` model (`nomic-embed-text-v1.5`, 768d) and compare
cosine similarity. A warm encode of two short spans is single-to-low-tens of ms —
within a per-edit budget. Generative inference is **not** viable here: the local
MLX server (`interfer`) is ~300ms TTFT warm on its smallest useful model,
serializes all requests, and the hook timeout is 5s.

**Tier 3 — distilled classifier (roadmap).** For overlap cases where embeddings
are ambiguous, log every `(reservation_context, edit_A, edit_B) → conflict?`
decision (the `intercept` pattern: `decisions.jsonl`), optionally consult a
fast cloud model async/advisory. Once a few hundred labels accumulate, distill to
a tiny local classifier (xgboost, or `interfer`'s purpose-built 262K-param
`ReservoirReadout` head) for a sub-ms forward pass.

Promotion follows interlock's existing shadow→enforce pattern: any model verdict
starts as `additionalContext` (advisory) before graduating to `decision:block`.

### Experiments gating tier 2 (E1–E3)

- **E1 — embedding latency.** Warm-encode real code-span pairs via intersearch's
  `EmbeddingClient`; measure p50/p99 on target hardware. Confirms tier 2 fits the
  per-edit budget (the estimate is tens-of-ms; measure it).
- **E2 — discrimination.** Cosine similarity over known conflict vs non-conflict
  edit pairs. If similarity doesn't separate the classes, tier 2 is theater and
  tier 3 is needed sooner. (Corpus: same-file edit pairs from real multi-agent
  and reconcile runs.)
- **E3 — glob hit rate.** Instrument how often edits actually overlap a live
  reservation in real runs. If <1%, tier 1 carries everything and tiers 2–3 are
  rarely-exercised insurance.

Results from E1–E3 feed back into this doc before tier 2 is implemented.

## 5. Config knobs

- `INTERLOCK_WORKTREE_ROOT` — legacy worktree base the sweeper scans
  (default `~/.cache/interlock/worktrees`).
- `INTERLOCK_SWEEP_AGE_DAYS` — only sweep dirs older than N days (default 1).
- `INTERLOCK_SWEEP_DRY_RUN=1` — report only, change nothing.
- Join flag `~/.config/clavain/intermute-joined` — master on/off for all
  coordination.
