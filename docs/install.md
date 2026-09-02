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

## Two gates

There are two independent gates, and it's easy to assume they're one:

- **The MCP tools** work as soon as `interlock-mcp` is running and registered — `reserve_files`, `check_conflicts`, and the rest answer immediately, raw-MCP config or not.
- **The advisory hooks and pre-commit enforcement** are a Claude Code plugin feature and switch on with `/interlock:join` (or by creating the join flag file by hand). A raw-MCP user who wants the pre-commit block installs the hooks themselves (§ 5); nothing about running `interlock-mcp` alone turns enforcement on.
