# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.2.19] - 2026-09-01

### Added

- Standalone install path (`docs/install.md`): run intermute and `interlock-mcp` outside Claude Code, with a raw MCP config for any client.
- `interlock-mcp` registers itself with intermute on startup and authenticates with the token intermute issues, so a raw-MCP install (no session hooks) still shows up in `list_agents` and gets `X-Agent-Token` on every request.
- `negotiate_release` accepts the holder's display name as well as their agent ID; it resolves the name against `list_agents` before matching conflicts.
- README `## Tools` section listing all 20 MCP tools, grouped by purpose; a structural test now fails if `tools.go` and the README list ever disagree.
- `docs/install.md` "Two gates" section: the MCP tools work as soon as the server runs, independent of the advisory hooks and pre-commit enforcement that switch on with `/interlock:join`.

### Fixed

- `force_release_negotiation` now pins to the exact reservation ID a negotiation named instead of releasing by file pattern, so a holder that released and re-reserved a different sub-pattern is no longer force-released on a thread it never saw. Both `force_release_negotiation` and `respond_to_release` now refuse a caller that isn't a party to the negotiation thread.
- `bin/launch-mcp.sh` no longer probes a hardcoded personal checkout path.
- The MCP server reports its actual version (`0.2.19`, matching the plugin manifests) instead of a stale `"0.1.0"` literal.
- README's description of the pre-edit hook now matches its code: it blocks an edit to a file another agent holds exclusively, and only downgrades to a warning on a tier-2 no-conflict verdict or when intermute is unreachable.

### Changed

- Join state moves from `~/.config/clavain/{intermute-joined,intermute-agent-name}` to `~/.config/interlock/{joined,agent-name}`. The old paths are still honored as a fallback, so an agent that joined before this change stays joined.
- `scripts/interlock-semantic-check.sh` no longer defaults `INTERLOCK_INTERSEARCH_DIR` to a path on any particular machine; unset means the semantic check is disabled (it already fails open).
- Bumped `github.com/mistakeknot/interbase/go` to v0.1.2, which ships a LICENSE.
- CI now runs on an ubuntu/macos matrix, checks `gofmt`, and runs the `tests/structural` pytest suite; the interbase checkout-and-replace-directive workaround is gone now that the module resolves from the proxy.

### Removed

- Internal planning and review artifacts not meant for external readers: `PHILOSOPHY.md`, `docs/plans/`, `docs/prds/`, `docs/research/`, `docs/experiments/`, `docs/PRD.md`, `docs/roadmap.md`, `.claude/agents/`, `.claude/flux-gen-specs/`, `.clavain/quality-gates/plan-review.md`.
- Seven structural tests asserting the per-session git-worktree isolation model removed in 0.2.16 (shared-filesystem coordination replaced it).
