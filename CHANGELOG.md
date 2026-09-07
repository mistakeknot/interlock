# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

- The commit lock works from a git worktree. `interlock-precommit-hook`, `commit_lock_path` in `hooks/lib.sh` and `interlock-install-hooks` derived their paths from `$GIT_ROOT/.git`, which is the gitdir pointer *file* inside a worktree, so every worktree commit spun on `mkdir` until the 30 s timeout and failed with "Commit lock timeout. Another session is committing." They now use `git rev-parse --path-format=absolute --git-common-dir` (hooks: `--git-path hooks`), so all worktrees of a checkout share one lock and one hooks directory. `INTERLOCK_COMMIT_LOCK_TIMEOUT` overrides the timeout (tests). Found 2026-09-02 on interlens; Sylveste-asqi.

## [0.2.20] - 2026-09-02

Found by running five agents through interlock on one checkout (2026-09-02); the run is written up in `docs/case-studies/2026-09-02-five-agents-one-checkout.md`.

- The pre-edit hook now detects a conflict on the intermute path. Its jq filter indexed the path string instead of the reservation, so every check came back clear and the hook then auto-reserved over another agent's exclusive hold (#3). Nothing was ever blocked.
- A 409 from the auto-reserve is now a block naming the holder. The block message shows the holder's name rather than its id (#3).
- `INTERLOCK_AGENT_NAME` is honoured by the session-start registration ahead of any per-user name file, so the hook and the MCP server carry one name; the hook treats a hold by an agent with its own name as its own (#4, interim).
- Commit notifications are addressed to real agent ids. They used to go out to a list of empty strings (#5). The post-commit auto-release had the same jq mistake and never released anything, and its DELETE carried no agent header, which intermute answers with 403.
- The post-commit hook no longer prints intermute's reply into git's output (#6). It also sees the files of a repository's first commit.
- One identity per session (#4). The MCP server adopts an agent the session-start hook already registered under its name, the hook adopts the server's row when it got there first, and inside tmux both default to the pane title; `INTERLOCK_AGENT_NAME` names both.
- `interlock-mcp --version` prints the version and exits. Any other flag is rejected. The pre-edit hook uses intermute unless `INTERLOCK_RESERVE_BACKEND=ic` asks for intercore explicitly (#7).
- The README and SECURITY.md say which edits the pre-edit hook sees: Edit and Write tool calls, not shell writes (#9).
- `fetch_inbox` pages. It sends intermute's `since_cursor`, returns the server's position as `next_cursor`, and caps a page at 50. It used to send a parameter intermute ignores and read a field intermute never sets, so every call returned the whole history, and one lane's inbox outgrew the client's output limit (#8). The pre-edit hook's inbox poll now uses the real route and acknowledges as the agent.
- The two-agent demo clears the caller's agent identity for the agents it launches. Run from inside a Claude Code session with the plugin loaded, both demo agents used to run as the caller, which intermute 0.1.1 now refuses (#10).
- Tests: `tests/structural/test_hook_scripts.py` runs the check script, the pre-edit hook, the post-commit hook, and the registration script against a fake intermute. The cases cover identity adoption and the reserve-backend switch.

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
