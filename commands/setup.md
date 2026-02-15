---
name: setup
description: Self-installing setup — check/build intermute binary, create systemd unit, start service, verify health
---

Set up the intermute coordination service.

## Steps

1. **Check if intermute binary exists:**
   ```bash
   command -v intermute || ls ~/.local/bin/intermute 2>/dev/null || ls /usr/local/bin/intermute 2>/dev/null
   ```

2. **If not found, build from source** (requires Go):
   ```bash
   cd /root/projects/Interverse/services/intermute && go build -o ~/.local/bin/intermute ./cmd/intermute/
   ```
   If Go is not installed, tell the user to install Go first.
   Verify: `~/.local/bin/intermute --help`

3. **Create systemd unit** (if it doesn't exist). Check if running as root — use system-level `/etc/systemd/system/intermute.service` for root, or `~/.config/systemd/user/` for non-root:
   ```ini
   [Unit]
   Description=intermute agent coordination service
   After=network.target

   [Service]
   ExecStart=/path/to/intermute serve --socket /var/run/intermute.sock --port 7338 --db /var/lib/intermute/intermute.db
   Restart=on-failure

   [Install]
   WantedBy=multi-user.target
   ```

4. **Create data directory:** `mkdir -p /var/lib/intermute`

5. **Start service:**
   ```bash
   systemctl daemon-reload
   systemctl enable intermute.service
   systemctl start intermute.service
   ```

6. **Verify health:**
   ```bash
   sleep 1
   curl -sf http://127.0.0.1:7338/health || curl -sf --unix-socket /var/run/intermute.sock http://localhost/health
   ```

7. **Report result.** If health check fails, show `systemctl status intermute.service` and `journalctl -u intermute.service --no-pager -n 20`.

8. **Next step:** Tell the user to run `/interlock:join` to register this agent.
