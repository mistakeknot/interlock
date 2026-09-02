---
name: interlock-setup
description: Install and validate intermute plus the interlock server (standalone path)
---

Install and validate the intermute coordination server and interlock-mcp.

## Steps

1. **Install intermute:**
   ```bash
   go install github.com/mistakeknot/intermute/cmd/intermute@latest
   intermute serve
   ```
   Listens on `:7338` by default.

2. **Install the interlock server:**
   ```bash
   go install github.com/mistakeknot/interlock/cmd/interlock-mcp@latest
   ```

3. **Check health:**
   ```bash
   curl -sf http://127.0.0.1:7338/health
   ```

4. **Point your MCP client at it.** See `docs/install.md` for the raw MCP config, the Claude Code plugin path, and how to install the enforcement hooks. Full reference: [`docs/install.md`](../docs/install.md).

If you're using Clavain, `/clavain:setup --scope interlock` runs these same steps for you.
