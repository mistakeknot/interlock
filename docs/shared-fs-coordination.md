# Shared-Filesystem Coordination

Status: **0.2.16** ships tier 1 (shared-FS pivot + leak fix). **0.2.17 ships
tier 2 (embedding similarity) in SHADOW mode** — the verdict is computed and
annotated onto block messages, but does not change behavior until
`INTERLOCK_SEMANTIC_ENABLE=1`. Validated by experiment (E1 latency PASS, E2
discrimination PASS — see §4). Tier 3 (LLM/distilled adjudication) is roadmap.

**Implementation note / honest limitation:** at the reservation layer the peer's
actual in-flight edit is not available — only their reservation *reason* string.
So tier 2 compares *this edit's new content* against *the peer's reason*, which
is a weaker signal than the region-vs-region setup E2 measured (it lands in the
0.70–0.90 "escalate" band more often). The full-strength upgrade is to store an
embedding of the reserved region at reserve-time (touches intermute/ic schema) so
a later checker does true region-vs-region; tracked as a follow-up. Shadow mode
exists precisely to gather real-trace data on this weaker signal before enforcing.

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

**Tier 2 — embedding similarity (validated, ready to implement).** On overlap,
embed the two edit regions with the already-local `intersearch` model
(`nomic-embed-text-v1.5`, 768d, via `EmbeddingClient.embed_batch`) and compare
cosine similarity. **Empirically confirmed (E1/E2 below):** a pair-encode runs
p50 13.5 ms / p99 ~20 ms warm — well inside the budget — and cosine cleanly
separates same-region edits from independent-region edits. Generative inference
is **not** viable here: the local MLX server (`interfer`) is ~300 ms TTFT warm on
its smallest useful model, serializes all requests, and the hook timeout is 5 s.

What tier 2 measures is **region/topic similarity, not logical contradiction.**
High cosine = "these edits touch the same code/logic" — the correct *trigger* for
a closer look, not a verdict. It exists to kill the "same file, different
function" false positives that tier 1 (filename) over-reports, narrowing the
candidate set for tier 3. Use a **hysteresis band**, not a knife-edge threshold:

- cosine **< 0.70** → treat as no-conflict (allow; auto-reserve)
- cosine **> 0.90** → treat as conflict (block / warn per fail-posture)
- **0.70–0.90** → escalate to tier 3

(The measured empty band was [0.69, 0.93]; 0.70/0.90 are the conservative edges.
Re-validate on real interlock edit traces before hard-coding — the corpus was 16
constructed pairs.)

**Tier 3 — distilled classifier (roadmap).** For the 0.70–0.90 escalation band,
log every `(reservation_context, edit_A, edit_B) → conflict?` decision (the
`intercept` pattern: `decisions.jsonl`), optionally consult a fast cloud model
async/advisory. Once a few hundred labels accumulate, distill to a tiny local
classifier (xgboost, or `interfer`'s purpose-built 262K-param `ReservoirReadout`
head) for a sub-ms forward pass. Tier 3 is the only tier that adjudicates actual
*contradiction* (vs. tier 2's "same region").

Promotion follows interlock's existing shadow→enforce pattern: any model verdict
starts as `additionalContext` (advisory) before graduating to `decision:block`.

### Experiment results (E1–E2 run 2026-06-23; artifacts in job tmp `embed-exp/`)

- **E1 — embedding latency: PASS.** Warm steady-state on M-series: single span
  encode p50 7.3 / p99 12.0 ms; **pair (2 encodes + cosine) p50 13.5 / p99 19.9
  ms.** Cold model load ~4.2 s one-time (amortized by a warm process). Fits the
  tens-of-ms budget with margin; batching both regions in one call would shave more.
- **E2 — discrimination: PASS.** 16 hand-labeled pairs (8 conflict / 8 not).
  NO-CONFLICT cosine ∈ [0.45, 0.69]; CONFLICT ∈ [0.93, 1.00] — a **0.234-wide
  empty band, zero overlap**, 100% separable. The hardest realistic case (two
  different functions in the *same* file) sat at 0.45–0.69, i.e. embeddings
  correctly demote it below the conflict band. Caveat: small constructed corpus;
  treat the band as provisional pending real-trace validation.
- **E3 — glob hit rate (not yet run).** Needs instrumentation on real
  multi-agent runs: how often does an edit actually overlap a live peer
  reservation? If <1%, tier 1 carries the load and tiers 2–3 are rare insurance.
  This is the next data point before investing in tier-2 implementation.

## 5. Config knobs

- `INTERLOCK_WORKTREE_ROOT` — legacy worktree base the sweeper scans
  (default `~/.cache/interlock/worktrees`).
- `INTERLOCK_SWEEP_AGE_DAYS` — only sweep dirs older than N days (default 1).
- `INTERLOCK_SWEEP_DRY_RUN=1` — report only, change nothing.
- `INTERLOCK_SEMANTIC_ENABLE` — tier-2 enforcement (default `0` = shadow: compute
  + annotate only; `1` = enforce: a "no-conflict" verdict downgrades a filename
  block to an advisory allow).
- `INTERLOCK_SEMANTIC_LOW` / `INTERLOCK_SEMANTIC_HIGH` — hysteresis band edges
  (defaults `0.70` / `0.90`).
- `INTERLOCK_INTERSEARCH_DIR` — path to the intersearch repo providing the
  embedding model; unset disables the semantic check (fails open to `unknown`).
- `INTERLOCK_SEMANTIC_TIMEOUT` — seconds before the semantic check gives up and
  fails open (default `3`).
- Join flag `~/.config/interlock/joined` — master on/off for all coordination.

## Relationship to worktree isolation

Interlock is the **coordination** layer for agents that deliberately *share* a
working tree; native Claude Code worktrees are the **isolation** layer for agents
that must not touch each other's files. They are complementary, not alternatives:
use worktrees to isolate parallel edits, use interlock reservations when a shared
tree is intentional. Interlock stopped creating its own worktrees at 0.2.16 (§1).
See the canonical [worktree-first coordination contract](../../../docs/guide-worktree-first-coordination.md).
