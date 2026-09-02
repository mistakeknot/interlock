# Bead-aware reservations

Interlock v0 represents Beads issue IDs ([beads](https://github.com/steveyegge/beads)) correlation in reservation **reason** metadata because the current Intermute reservation schema has no first-class `metadata` column.

This is intentionally a compact compatibility convention, not a new task-state source of truth. Beads remains canonical for task state; Interlock reservations are live coordination hints for file/path intent.

## Reservation metadata convention

`reserve_files` accepts optional correlation fields:

- `active_bead_id` — preferred current work item ID, e.g. `sylveste-kgfi.3`.
- `bead_id` — canonical Beads issue ID if it differs from the active child/increment.
- `thread_id` — optional message/thread correlation handle; use the bead ID when possible.

Until Intermute stores reservation metadata natively, Interlock appends these to the reservation reason as key/value metadata:

```text
<operator reason> [active_bead_id=sylveste-kgfi.3 bead_id=sylveste-kgfi thread_id=sylveste-kgfi.3]
```

Readers must treat this as a v0 wire convention. If multiple distinct bead values are found in one reason, Interlock marks the reservation correlation as `ambiguous` instead of guessing.

## Collision cards

`check_conflicts` returns an agent-readable collision summary for the configured repo/project and requested path patterns:

```json
{
  "project": "Sylveste",
  "patterns": ["interverse/interlock/internal/*"],
  "requested_bead_id": "sylveste-kgfi.3",
  "cards": [
    {
      "reservation_id": "res-123",
      "agent_id": "agent-a",
      "held_by": "agent-a",
      "project": "Sylveste",
      "requested_path": "interverse/interlock/internal/*",
      "pattern": "interverse/interlock/internal/client/*",
      "path_pattern": "interverse/interlock/internal/client/*",
      "state": "active",
      "confidence": "reported",
      "active_bead_id": "sylveste-kgfi.3",
      "suggested_action": "coordinate_same_bead",
      "hard_blocker": false
    }
  ],
  "clear": ["docs/*"]
}
```

Card fields are intentionally small:

- holder: `agent_id` / `held_by`
- scope: `project`, `requested_path`, `pattern` / `path_pattern`
- task correlation: `active_bead_id`, `bead_id`, `thread_id`, `confidence`
- reservation state: `state`, `expires_at`
- next move: `suggested_action`, `hard_blocker`

`conflicts` is a compatibility alias containing the subset of cards where `hard_blocker == true`.
Hard blockers come from Intermute's reservation conflict endpoint; list-derived overlap cards are advisory context only.
`clear` means “no hard blockers for this requested pattern”; a pattern can appear in `clear` while still having non-blocking same-bead, stale, ambiguous, or shared cards in `cards`.

## Confidence and state

Correlation confidence:

- `reported` — exactly one bead/thread correlation was found in the reason metadata.
- `unknown` — no bead/thread correlation was found.
- `ambiguous` — multiple distinct candidate values were found; do not guess.

Reservation state:

- `active` — live reservation evidence.
- `stale` — inactive/expired/released evidence surfaced for context; never a hard blocker.

## Suggested actions

- `coordinate_same_bead` — same bead is already holding the path; coordinate/reuse context rather than treating it as a collision.
- `negotiate_release` — active exclusive reservation for another or unknown bead; use Interlock negotiation.
- `coordinate_with_holder` — non-exclusive active overlap; coordinate with the holder.
- `clarify_bead_before_negotiation` — ambiguous bead evidence; clarify before treating it as a hard conflict.
- `wait_for_expiry_or_release` — stale/expired evidence; wait for cleanup or request release only if it is still operationally visible.

## Prior-art boundary

This intentionally reuses the useful MCP Agent Mail pattern of file reservations plus release negotiation, but does not adopt its installer, authority model, or environment mutations wholesale. Interlock stays a thin adapter over intermute reservations.
