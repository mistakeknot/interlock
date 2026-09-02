#!/usr/bin/env python3
"""Two agents, one file, no collision.

Drives two interlock-mcp servers over stdio (plain MCP JSON-RPC, stdlib only)
against one intermute, and shows the reservation / negotiation / release loop.
Then asks intermux who it can see.

Env: INTERMUTE_URL (default http://127.0.0.1:7338), INTERLOCK_MCP, INTERMUX_MCP
(paths to the binaries; default: on PATH), DEMO_PROJECT (default: cwd).
"""
import json, os, subprocess, sys, time

INTERMUTE_URL = os.environ.get("INTERMUTE_URL", "http://127.0.0.1:7338")
PROJECT = os.environ.get("DEMO_PROJECT", os.getcwd())
PATTERN = "src/**/*.go"
LOG_DIR = os.environ.get("DEMO_LOG_DIR", os.getcwd())


class MCP:
    """Minimal MCP stdio client: newline-delimited JSON-RPC 2.0."""

    def __init__(self, name, binary, env):
        self.name = name
        self.proc = subprocess.Popen(
            [binary], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=open(os.path.join(LOG_DIR, f"{name}.stderr.log"), "w"), env={**os.environ, **env}, text=True,
        )
        self._id = 0
        self.request("initialize", {
            "protocolVersion": "2024-11-05", "capabilities": {},
            "clientInfo": {"name": f"demo-{name}", "version": "0"},
        })
        self.notify("notifications/initialized", {})

    def _send(self, msg):
        self.proc.stdin.write(json.dumps(msg) + "\n")
        self.proc.stdin.flush()

    def notify(self, method, params):
        self._send({"jsonrpc": "2.0", "method": method, "params": params})

    def request(self, method, params):
        self._id += 1
        self._send({"jsonrpc": "2.0", "id": self._id, "method": method, "params": params})
        while True:
            line = self.proc.stdout.readline()
            if not line:
                raise RuntimeError(f"{self.name}: server closed stdout")
            msg = json.loads(line)
            if msg.get("id") == self._id:
                if "error" in msg:
                    raise RuntimeError(f"{self.name}: {method} -> {msg['error']}")
                return msg["result"]

    def call(self, tool, **args):
        res = self.request("tools/call", {"name": tool, "arguments": args})
        text = "".join(c.get("text", "") for c in res.get("content", []))
        try:
            data = json.loads(text)
        except ValueError:
            data = {"text": text}
        if res.get("isError"):
            data["is_error"] = True
        return data

    def close(self):
        try:
            self.proc.stdin.close()
            self.proc.wait(timeout=5)
        except Exception:
            self.proc.kill()


def step(n, who, what):
    print(f"\n[{n}] {who}: {what}", flush=True)


def show(data):
    print(json.dumps(data, indent=2, sort_keys=True), flush=True)


def main():
    interlock = os.environ.get("INTERLOCK_MCP", "interlock-mcp")
    intermux = os.environ.get("INTERMUX_MCP", "intermux-mcp")
    base = {"INTERMUTE_URL": INTERMUTE_URL, "INTERLOCK_PROJECT": PROJECT, "INTERMUTE_SOCKET": ""}

    # Names only: intermute issues the IDs at registration, and each
    # interlock-mcp registers itself on startup.
    alpha = MCP("alpha", interlock, {**base, "INTERLOCK_AGENT_NAME": "alpha"})
    beta = MCP("beta", interlock, {**base, "INTERLOCK_AGENT_NAME": "beta"})
    try:
        step(1, "alpha", f"reserves {PATTERN}")
        show(alpha.call("reserve_files", patterns=[PATTERN], reason="refactoring the parser", ttl_minutes=30))

        step(2, "beta", f"checks {PATTERN} before editing")
        conflicts = beta.call("check_conflicts", patterns=[PATTERN])
        show(conflicts)

        step(3, "beta", "asks alpha to release, without blocking")
        neg = beta.call("negotiate_release", agent_name="alpha", file=PATTERN,
                        reason="need to fix a failing test in the parser", urgency="normal", wait_seconds=0)
        show(neg)
        thread_id = neg["thread_id"]

        step(4, "alpha", "reads its inbox")
        inbox = alpha.call("fetch_inbox")
        show(inbox)
        request = next(m for m in inbox["messages"] if m.get("thread_id") == thread_id)

        step(5, "alpha", "releases in response, addressing the requester named in the message")
        show(alpha.call("respond_to_release", thread_id=thread_id, requester=request["from"],
                        action="release", file=PATTERN))

        step(6, "beta", f"reserves {PATTERN}, which now succeeds")
        show(beta.call("reserve_files", patterns=[PATTERN], reason="fixing the parser test", ttl_minutes=30))

        step(7, "beta", "lists agents intermute knows about")
        show(beta.call("list_agents"))

        step(8, "beta", "releases everything")
        show(beta.call("release_all"))
    finally:
        alpha.close()
        beta.close()

    if os.environ.get("DEMO_SKIP_INTERMUX"):
        return
    step(9, "intermux", "reports the tmux sessions it can see")
    mux = MCP("intermux", intermux, {"INTERMUTE_URL": INTERMUTE_URL})
    try:
        time.sleep(2)  # first tmux scan
        show(mux.call("list_agents"))
        show(mux.call("who_is_editing", pattern="src/"))
    finally:
        mux.close()


if __name__ == "__main__":
    main()
