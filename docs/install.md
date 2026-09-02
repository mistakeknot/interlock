# Standalone install

interlock works with any MCP client, not just Claude Code plugins. This is the client-agnostic path.

## 1. Run intermute

```bash
go install github.com/mistakeknot/intermute/cmd/intermute@latest
intermute serve
```

Listens on `:7338` by default.

## 2. Install the interlock server

```bash
go install github.com/mistakeknot/interlock/cmd/interlock-mcp@latest
```

## 3. Point your MCP client at it

Raw MCP config, for any client that reads one:

```json
{
  "mcpServers": {
    "interlock": {
      "command": "interlock-mcp",
      "env": {
        "INTERMUTE_URL": "http://127.0.0.1:7338",
        "INTERLOCK_AGENT_NAME": "alpha",
        "INTERLOCK_PROJECT": "/path/to/repo"
      }
    }
  }
}
```

`interlock-mcp` registers itself with intermute on startup, so the 20 tools work immediately — see § Two gates below.

## 4. Claude Code plugin path

If you're using Claude Code, install via the plugin marketplace instead (see README § Installation) — it wires the manifest, hooks, and commands for you.

## 5. Install the hooks (optional, for enforcement)

The MCP tools don't need git hooks. If you want the pre-commit block on reserved files:

```bash
bash scripts/interlock-install-hooks
```

If that script isn't present in your checkout, install the pre-commit hook by hand:

```bash
cp scripts/interlock-precommit-hook .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

## Try it with Codex, Hermes, or Kimi

interlock is an MCP server, so any client that can run a stdio server can use it. The three below were each run against a local intermute on 2026-09-02; the recordings sit in [`examples/two-agent-demo/clients/`](../examples/two-agent-demo/clients/). Every run does the same four calls: reserve `src/**/*.go`, list the reservation, check `src/main.go` for conflicts, release everything.

Start intermute first, and build the server once:

```bash
intermute serve --db ./intermute.db --port 7339 &
go build -o bin/interlock-mcp ./cmd/interlock-mcp
```

### Codex

No config file needed; `-c` passes the server on the command line:

```bash
codex exec --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox \
  -c 'mcp_servers.interlock.command="/path/to/interlock/bin/interlock-mcp"' \
  -c 'mcp_servers.interlock.env={INTERMUTE_URL="http://127.0.0.1:7339",INTERLOCK_AGENT_NAME="codex-demo",INTERLOCK_PROJECT="demo"}' \
  "Call reserve_files with patterns [\"src/**/*.go\"], then my_reservations, then check_conflicts for src/main.go, then release_all."
```

Recording: `clients/codex.cast`.

### Hermes

Register the server once (the `Enable all tools?` prompt takes `Y`), then ask:

```bash
hermes mcp add interlock --command /path/to/interlock/bin/interlock-mcp \
  --env INTERMUTE_URL=http://127.0.0.1:7339 INTERLOCK_AGENT_NAME=hermes-demo INTERLOCK_PROJECT=demo
hermes chat -Q --yolo -q "Call reserve_files with patterns [\"src/**/*.go\"], then my_reservations, then check_conflicts for src/main.go, then release_all."
```

Recording: `clients/hermes.cast`.

### Kimi Code

Add the server to `~/.kimi-code/mcp.json`:

```json
{
  "mcpServers": {
    "interlock": {
      "command": "/path/to/interlock/bin/interlock-mcp",
      "env": {
        "INTERMUTE_URL": "http://127.0.0.1:7339",
        "INTERLOCK_AGENT_NAME": "kimi-demo",
        "INTERLOCK_PROJECT": "demo"
      }
    }
  }
}
```

then:

```bash
kimi -p "Call reserve_files with patterns [\"src/**/*.go\"], then my_reservations, then check_conflicts for src/main.go, then release_all."
```

`-p` does not combine with `--yolo` or `--auto`; set `default_permission_mode = "yolo"` in `~/.kimi-code/config.toml` for an unattended run. Recording: `clients/kimi.cast`.

### What you do not get outside Claude Code

The MCP tools are the whole coordination surface. The pre-edit block and the git pre-commit hook are Claude Code plugin features; see [Two gates](#two-gates) above.

## Two gates

There are two independent gates, and it's easy to assume they're one:

- **The MCP tools** work as soon as `interlock-mcp` is running and registered — `reserve_files`, `check_conflicts`, and the rest answer immediately, raw-MCP config or not.
- **The advisory hooks and pre-commit enforcement** are a Claude Code plugin feature and switch on with `/interlock:join` (or by creating the join flag file by hand). A raw-MCP user who wants the pre-commit block installs the hooks themselves (§ 5); nothing about running `interlock-mcp` alone turns enforcement on.
