#!/usr/bin/env python3
"""Evidence logger for the dogfood run.

Every INTERVAL seconds append one JSON line per source to $D/log/:
  reservations.jsonl  rows of file_reservations (sqlite, read-only)
  messages.jsonl      rows of messages (sqlite, read-only)
  agents.jsonl        GET /api/agents from intermute
  intermux.jsonl      list_agents + activity_feed + who_is_editing from intermux-mcp over stdio
Rows are only appended when they are new (by primary key) so the files stay small.
"""
import json, os, sqlite3, subprocess, sys, time, urllib.request

D = os.path.dirname(os.path.abspath(__file__))
LOG = os.path.join(D, "log")
os.makedirs(LOG, exist_ok=True)
DB = os.path.join(D, "intermute.db")
INTERVAL = int(os.environ.get("OBS_INTERVAL", "60"))
INTERMUX = os.environ.get("INTERMUX_BIN", "<checkout>/interverse/intermux/bin/intermux-mcp")
BASE = "http://127.0.0.1:7338"

seen = {"reservations": set(), "messages": set()}


def now():
    return time.strftime("%Y-%m-%dT%H:%M:%S%z")


def append(name, obj):
    with open(os.path.join(LOG, name + ".jsonl"), "a") as f:
        f.write(json.dumps(obj, default=str) + "\n")


def snap_sqlite():
    con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
    con.row_factory = sqlite3.Row
    for row in con.execute("select * from file_reservations order by created_at"):
        d = dict(row)
        key = (d["id"], d["released_at"])
        if key in seen["reservations"]:
            continue
        seen["reservations"].add(key)
        d["_seen"] = now()
        append("reservations", d)
    for row in con.execute("select * from messages order by created_at"):
        d = dict(row)
        key = (d["project"], d["message_id"])
        if key in seen["messages"]:
            continue
        seen["messages"].add(key)
        d["_seen"] = now()
        append("messages", d)
    con.close()


def snap_agents():
    try:
        with urllib.request.urlopen(BASE + "/api/agents", timeout=5) as r:
            body = json.load(r)
    except Exception as e:  # noqa: BLE001
        body = {"error": str(e)}
    append("agents", {"_seen": now(), "agents": body})


class MCP:
    def __init__(self, binary, env):
        self.p = subprocess.Popen([binary], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  stderr=subprocess.DEVNULL, text=True, env=env)
        assert self.p.stdin is not None and self.p.stdout is not None
        self.stdin, self.stdout = self.p.stdin, self.p.stdout
        self.n = 0
        self.request("initialize", {"protocolVersion": "2024-11-05", "capabilities": {},
                                    "clientInfo": {"name": "observe", "version": "0"}})
        self.notify("notifications/initialized")

    def request(self, method, params):
        self.n += 1
        self.stdin.write(json.dumps({"jsonrpc": "2.0", "id": self.n, "method": method, "params": params}) + "\n")
        self.stdin.flush()
        line = self.stdout.readline()
        return json.loads(line) if line else {"error": "eof"}

    def notify(self, method):
        self.stdin.write(json.dumps({"jsonrpc": "2.0", "method": method}) + "\n")
        self.stdin.flush()

    def call(self, name, args=None):
        r = self.request("tools/call", {"name": name, "arguments": args or {}})
        try:
            return json.loads(r["result"]["content"][0]["text"])  # type: ignore[index]
        except Exception:  # noqa: BLE001
            return r

    def close(self):
        try:
            self.stdin.close()
            self.p.wait(timeout=3)
        except Exception:  # noqa: BLE001
            self.p.kill()


def snap_intermux():
    env = dict(os.environ)
    env["INTERMUTE_URL"] = BASE
    env["INTERMUX_MAPPING_DIR"] = os.path.join(D, "intermux-mappings")
    try:
        m = MCP(INTERMUX, env)
        out = {"_seen": now(),
               "list_agents": m.call("list_agents"),
               "activity_feed": m.call("activity_feed", {"limit": 20}),
               "who_is_editing": m.call("who_is_editing", {"pattern": "interverse/"})}
        m.close()
    except Exception as e:  # noqa: BLE001
        out = {"_seen": now(), "error": str(e)}
    append("intermux", out)


def main():
    once = "--once" in sys.argv
    while True:
        try:
            snap_sqlite()
        except Exception as e:  # noqa: BLE001
            append("errors", {"_seen": now(), "sqlite": str(e)})
        snap_agents()
        snap_intermux()
        if once:
            break
        time.sleep(INTERVAL)


if __name__ == "__main__":
    main()
