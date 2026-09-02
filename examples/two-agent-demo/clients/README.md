# interlock from Codex, Hermes, and Kimi Code

Three recordings of the same four calls (reserve `src/**/*.go`, list the reservation, check `src/main.go`, release everything) made from clients other than Claude Code on 2026-09-02, each against a local intermute on port 7339:

- `codex.cast`: Codex CLI 0.146, server passed with `-c` on the command line.
- `hermes.cast`: Hermes Agent 0.20, server registered with `hermes mcp add`.
- `kimi.cast`: Kimi Code 0.36, server listed in `~/.kimi-code/mcp.json`.

Play one with `asciinema play <file>`. The prompt each client received is `prompt.txt` with `CLIENT` replaced by the client's name. The exact commands are in [`docs/install.md`](../../../docs/install.md) under "Try it with Codex, Hermes, or Kimi".
