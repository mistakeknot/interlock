# PRD: Interlock — Multi-Agent File Coordination Companion

**Version:** 0.1.0
**Last updated:** 2026-02-15
**Vision:** [`docs/vision.md`](vision.md)
**Roadmap:** [`docs/roadmap.md`](roadmap.md)

## 1. Product Definition

Interlock is a companion plugin for Clavain that enables safe multi-agent editing by exposing `intermute` coordination APIs through MCP and shell hooks.
It provides explicit reservation workflows, conflict visibility, and agent messaging so parallel agents can coordinate with minimal ceremony.

## 2. Users

- **Inner users:** Claude Code agents running in shared repos who need conflict-free parallel editing.
- **Middle users:** plugin maintainers who need a repeatable, low-overhead coordination layer.
- **Outer users:** teams exploring multi-agent development patterns.

## 3. Problem

Without coordination, agents can edit the same files concurrently, resulting in:

- silent overwrite risk before commit,
- context drift across handoffs,
- and late-stage conflict discovery that consumes review time.

## 4. Solution

Interlock adds a thin coordination layer:

- one Go MCP server with 9 tools,
- bash hooks wired into Claude Code lifecycle events,
- shell commands for explicit join/leave/status/setup,
- and a git pre-commit safety gate.

## 5. Component Architecture

| Type | Count | Primary Role |
|---|---|---|
| Go MCP server | 1 | `interlock-mcp` process exposing tools and delegating to `intermute` |
| MCP tools | 9 | `reserve_files`, `release_files`, `release_all`, `check_conflicts`, `my_reservations`, `send_message`, `fetch_inbox`, `request_release`, `list_agents` |
| Commands | 4 | `join`, `leave`, `status`, `setup` |
| Skills | 2 | `coordination-protocol`, `conflict-recovery` |
| Hooks | 4 | `SessionStart`, `PreToolUse:Edit` advisory, `Stop`, and git pre-commit enforcement |

### 5.1 Go binary (`cmd/interlock-mcp`)

`cmd/interlock-mcp/main.go` runs a `mark3labs/mcp-go` stdio server and composes:

- `internal/client` for HTTP communication,
- `internal/tools` for tool handlers and payload shaping,
- environment-driven identity defaults (`INTERMUTE_SOCKET`, `INTERMUTE_URL`, `INTERLOCK_AGENT_ID`, `CLAUDE_SESSION_ID`, etc.).

Transport strategy is socket-first with TCP fallback:
- use `INTERMUTE_SOCKET` when available (Unix socket),
- otherwise call `INTERMUTE_URL` (default `http://127.0.0.1:7338`).

### 5.2 HTTP client (`internal/client`)

Client methods map directly to `intermute` endpoints for:

- reservation create/list/delete,
- conflict checks (with fallback when `check` endpoint is unavailable),
- agent registry calls,
- inbox/message exchange,
- structured error mapping for conflict and service failures.

### 5.3 Hooks and scripts (`hooks/`, `scripts/`)

`hooks/hooks.json` wires host events to:

- `session-start.sh`: auto-registration and environment export when joined,
- `pre-edit.sh`: non-blocking warning on edits to reserved files,
- `stop.sh`: release-and-cleanup.

Script helpers include `interlock-check.sh`, `interlock-register.sh`, `interlock-cleanup.sh`, and `interlock-signal.sh`, with `interlock-install-hooks` and `interlock-precommit-hook` for repository enforcement.

## 6. Key Workflows

### 6.1 Reserve/Release and Ownership Hygiene

1. Agent joins coordination (`/interlock:join`) and creates local join state under `~/.config/clavain`.
2. `SessionStart` uses helper scripts to register agent identity and write session artifacts when join is enabled.
3. Agents reserve file patterns using MCP `reserve_files` with reason/TTL.
4. Agents release specific reservations with `release_files` or all with `release_all`.
5. `Stop` hook triggers best-effort cleanup via `interlock-cleanup.sh`.

### 6.2 Conflict Detection

1. `check_conflicts` MCP tool runs read-only conflict checks.
2. `PreToolUse:Edit` hook checks the target file path on each edit and injects advisory context.
3. git pre-commit hook performs mandatory enforcement against current staged files.
4. A clear block message requires users to resolve conflicts, wait, or use `--no-verify` knowingly.

### 6.3 Agent Messaging

Agents coordinate when conflicts arise through:

- `send_message` (direct message),
- `fetch_inbox` (polling),
- `request_release` (protocolized release ask),
- `list_agents` (context before escalation).

### 6.4 Signals and status visibility

Tool actions emit lightweight signal events through `interlock-signal.sh`:

- event types: reserve/release/message,
- normalized JSONL schema with version, layer, icon, text, priority, timestamp,
- file naming by project slug and agent identity for consumers like interline.

## 7. Non-Goals

- No automatic reservation on every edit.
- No replacement for agent judgement.
- No in-repo conflict policy that blocks creative progress before commit gate.

## 8. Risks

- Coordination depends on `intermute` availability; hooks degrade gracefully if the service is temporarily unreachable.
- Hook-based behavior is intentionally advisory until commit-time enforcement, so misuse is possible if users force-through options are used.
- Unix socket path and service health assumptions require healthy local environment setup for full capability.

