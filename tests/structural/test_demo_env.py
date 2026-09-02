"""The two-agent demo must not inherit the caller's agent identity (issue #10)."""
import json
import os
import subprocess
import sys
from pathlib import Path

DEMO_DIR = Path(__file__).resolve().parents[2] / "examples" / "two-agent-demo"


def test_demo_agents_do_not_inherit_the_callers_identity():
    probe = (
        "import json, os, sys; sys.path.insert(0, %r); import demo; "
        "env = {**os.environ, **demo.agent_env({'INTERMUTE_URL': 'http://127.0.0.1:1'}, 'alpha')}; "
        "print(json.dumps({k: env.get(k, '<unset>') for k in "
        "['INTERMUTE_AGENT_ID', 'INTERLOCK_AGENT_ID', 'INTERMUTE_AGENT_NAME', 'CLAUDE_SESSION_ID', 'INTERLOCK_AGENT_NAME']}))"
    ) % str(DEMO_DIR)
    leaked = {
        **os.environ,
        "INTERMUTE_AGENT_ID": "d523f879-leaked",
        "INTERLOCK_AGENT_ID": "leaked-too",
        "INTERMUTE_AGENT_NAME": "the caller",
        "CLAUDE_SESSION_ID": "0123456789abcdef",
    }
    proc = subprocess.run([sys.executable, "-c", probe], env=leaked, capture_output=True, text=True, timeout=30)
    assert proc.returncode == 0, proc.stderr
    env = json.loads(proc.stdout)
    # An empty value is "unset" to interlock-mcp's os.Getenv checks.
    assert env["INTERMUTE_AGENT_ID"] == ""
    assert env["INTERLOCK_AGENT_ID"] == ""
    assert env["INTERMUTE_AGENT_NAME"] == ""
    assert env["CLAUDE_SESSION_ID"] == ""
    assert env["INTERLOCK_AGENT_NAME"] == "alpha"
