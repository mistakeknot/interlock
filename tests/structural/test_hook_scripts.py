"""End-to-end tests for the shell hooks against a fake intermute.

Found by the 2026-09-02 dogfood run: the conflict check never detected a
conflict (issue #3), the post-commit notification had empty recipients
(issue #5) and printed intermute's reply into git output (issue #6). These
tests run the real scripts with curl and jq against a small HTTP server.
"""

import json
import os
import shutil
import subprocess
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from urllib.parse import urlparse, parse_qs

import pytest

REPO = Path(__file__).resolve().parents[2]
CHECK = REPO / "scripts" / "interlock-check.sh"
POSTCOMMIT = REPO / "scripts" / "interlock-postcommit-hook"
PRE_EDIT = REPO / "hooks" / "pre-edit.sh"

pytestmark = pytest.mark.skipif(
    shutil.which("jq") is None or shutil.which("curl") is None,
    reason="jq and curl are required to run the hook scripts",
)


class FakeIntermute:
    """Serves canned reservations and agents; records POSTs and DELETEs."""

    def __init__(self, reservations, agents, agent_reservations=None):
        self.reservations = reservations
        self.agents = agents
        self.agent_reservations = agent_reservations or []
        self.post_only_reservations = []  # holds the POST sees but the list does not (race)
        self.posts = []
        self.deletes = []
        fake = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, format, *args):  # noqa: A002 - silence the server log
                del format, args

            def _send(self, code, body):
                data = json.dumps(body).encode()
                self.send_response(code)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_GET(self):
                url = urlparse(self.path)
                qs = parse_qs(url.query)
                if url.path == "/api/reservations" and "agent" in qs:
                    self._send(200, {"reservations": fake.agent_reservations})
                elif url.path == "/api/reservations":
                    self._send(200, {"reservations": fake.reservations})
                elif url.path == "/api/agents":
                    self._send(200, {"agents": fake.agents})
                elif url.path.startswith("/api/messages/inbox"):
                    self._send(200, {"messages": []})
                else:
                    self._send(404, {"error": "not found"})

            def do_POST(self):
                length = int(self.headers.get("Content-Length", "0"))
                body = self.rfile.read(length).decode()
                payload = json.loads(body) if body else None
                fake.posts.append((self.path, payload))
                if self.path == "/api/reservations" and payload:
                    for r in fake.reservations + fake.post_only_reservations:
                        same_path = r["path_pattern"] == payload["path_pattern"]
                        if r["is_active"] and r["exclusive"] and same_path and r["agent_id"] != payload["agent_id"]:
                            name = next((a["name"] for a in fake.agents if a["agent_id"] == r["agent_id"]), "")
                            self._send(409, {"error": "reservation_conflict", "conflicts": [
                                {"reservation_id": r["id"], "agent_id": r["agent_id"], "held_by": name,
                                 "pattern": r["path_pattern"], "reason": r["reason"], "expires_at": r["expires_at"]}]})
                            return
                    self._send(200, dict(payload, id="res-new"))
                    return
                self._send(200, {"message_id": "m-1", "cursor": 1, "delivery": "async"})

            def do_DELETE(self):
                if not self.headers.get("X-Agent-ID"):
                    self._send(403, {"error": "agent identity required"})
                    return
                fake.deletes.append(self.path)
                self._send(200, {"released": True})

        self.server = HTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self.server.server_port}"
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def stop(self):
        self.server.shutdown()
        self.server.server_close()


def _git_repo(tmp_path: Path) -> Path:
    repo = tmp_path / "proj"
    repo.mkdir()
    subprocess.run(["git", "init", "-q", "-b", "main", str(repo)], check=True)
    subprocess.run(["git", "-C", str(repo), "config", "user.email", "t@example.com"], check=True)
    subprocess.run(["git", "-C", str(repo), "config", "user.name", "t"], check=True)
    return repo


def _stranger_path():
    """PATH without any directory that holds an `ic` binary: the pre-edit hook
    prefers intercore when it finds one, and a stranger has none."""
    return ":".join(d for d in os.environ["PATH"].split(":") if not (Path(d) / "ic").exists())


def _env(fake_url, **extra):
    env = {
        "PATH": _stranger_path(),
        "HOME": os.environ.get("HOME", "/tmp"),
        "INTERMUTE_URL": fake_url,
        "INTERMUTE_SOCKET": "/nonexistent/intermute.sock",
    }
    env.update(extra)
    return env


def _reservation(agent_id, pattern, **kw):
    r = {
        "id": f"res-{agent_id}-{pattern}",
        "agent_id": agent_id,
        "project": "proj",
        "path_pattern": pattern,
        "exclusive": True,
        "reason": "editing",
        "created_at": "2026-09-02T00:00:00Z",
        "expires_at": "2099-01-01T00:00:00Z",
        "is_active": True,
    }
    r.update(kw)
    return r


AGENTS = [
    {"agent_id": "us", "name": "me", "project": "proj"},
    {"agent_id": "twin", "name": "me", "project": "proj"},
    {"agent_id": "other", "name": "peer", "project": "proj"},
]


def run_check(repo, fake, path, agent_id="us", name="me"):
    proc = subprocess.run(
        ["bash", str(CHECK), str(repo / path), agent_id],
        cwd=repo,
        env=_env(fake.url, INTERMUTE_AGENT_NAME=name),
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert proc.returncode == 0, proc.stderr
    out = proc.stdout.strip()
    return json.loads(out) if out else None


class TestConflictCheck:
    def test_detects_foreign_exclusive_hold(self, tmp_path):
        repo = _git_repo(tmp_path)
        fake = FakeIntermute([_reservation("other", "README.md")], AGENTS)
        try:
            conflict = run_check(repo, fake, "README.md")
        finally:
            fake.stop()
        assert conflict is not None, "a foreign exclusive hold must be reported"
        assert conflict["held_by"] == "other"
        assert conflict["held_by_name"] == "peer"
        assert conflict["pattern"] == "README.md"

    def test_glob_prefix_matches(self, tmp_path):
        repo = _git_repo(tmp_path)
        fake = FakeIntermute([_reservation("other", "src/*")], AGENTS)
        try:
            conflict = run_check(repo, fake, "src/a.go")
        finally:
            fake.stop()
        assert conflict is not None and conflict["held_by"] == "other"

    def test_own_and_same_name_holds_are_not_conflicts(self, tmp_path):
        repo = _git_repo(tmp_path)
        fake = FakeIntermute(
            [_reservation("us", "README.md"), _reservation("twin", "README.md")], AGENTS
        )
        try:
            conflict = run_check(repo, fake, "README.md")
        finally:
            fake.stop()
        assert conflict is None

    def test_released_and_shared_holds_are_not_conflicts(self, tmp_path):
        repo = _git_repo(tmp_path)
        fake = FakeIntermute(
            [
                _reservation("other", "README.md", is_active=False),
                _reservation("other", "docs/*", exclusive=False),
            ],
            AGENTS,
        )
        try:
            assert run_check(repo, fake, "README.md") is None
            assert run_check(repo, fake, "docs/x.md") is None
        finally:
            fake.stop()


class TestPostCommitHook:
    def _commit(self, repo, name="README.md"):
        (repo / name).write_text("hello\n")
        subprocess.run(["git", "-C", str(repo), "add", name], check=True)
        subprocess.run(["git", "-C", str(repo), "commit", "-q", "-m", "test"], check=True)

    def _run(self, repo, fake):
        return subprocess.run(
            ["bash", str(POSTCOMMIT)],
            cwd=repo,
            env=_env(
                fake.url,
                INTERMUTE_AGENT_ID="us",
                INTERMUTE_AGENT_NAME="me",
                INTERMUTE_PROJECT="proj",
            ),
            capture_output=True,
            text=True,
            timeout=30,
        )

    def test_notification_goes_to_real_agent_ids_only(self, tmp_path):
        repo = _git_repo(tmp_path)
        self._commit(repo)
        fake = FakeIntermute([], AGENTS)
        try:
            proc = self._run(repo, fake)
        finally:
            fake.stop()
        assert proc.returncode == 0, proc.stderr
        posts = [p for p in fake.posts if p[0] == "/api/messages"]
        assert len(posts) == 1, fake.posts
        payload = posts[0][1]
        assert payload["to"] == ["other"], payload
        assert payload["from"] == "us"
        assert payload["subject"].startswith("commit:")
        assert json.loads(payload["body"])["files"] == ["README.md"]

    def test_nothing_is_printed_to_the_terminal(self, tmp_path):
        repo = _git_repo(tmp_path)
        self._commit(repo)
        fake = FakeIntermute([], AGENTS)
        try:
            proc = self._run(repo, fake)
        finally:
            fake.stop()
        assert proc.stdout == "", proc.stdout

    def test_committed_file_reservation_is_released(self, tmp_path):
        repo = _git_repo(tmp_path)
        (repo / "src").mkdir()
        self._commit(repo, "src/a.go")
        fake = FakeIntermute(
            [], AGENTS, agent_reservations=[_reservation("us", "src/*", id="res-glob")]
        )
        try:
            proc = self._run(repo, fake)
        finally:
            fake.stop()
        assert proc.returncode == 0, proc.stderr
        assert "/api/reservations/res-glob" in fake.deletes, fake.deletes


class TestPreEditHook:
    """The PreToolUse hook end to end: stdin JSON in, block decision or nothing out."""

    def _run(self, repo, fake, path):
        hook_input = json.dumps({
            "session_id": "sess-1",
            "cwd": str(repo),
            "tool_name": "Edit",
            "tool_input": {"file_path": str(repo / path), "old_string": "a", "new_string": "b"},
        })
        return subprocess.run(
            ["bash", str(PRE_EDIT)],
            cwd=repo,
            input=hook_input,
            env=_env(
                fake.url,
                INTERMUTE_AGENT_ID="us",
                INTERMUTE_AGENT_NAME="me",
                INTERMUTE_PROJECT="proj",
                INTERLOCK_PROJECT_ROOT=str(repo),
                CLAUDE_SESSION_ID="sess-1",
            ),
            capture_output=True,
            text=True,
            timeout=30,
        )

    def test_foreign_hold_blocks_and_names_the_holder(self, tmp_path):
        repo = _git_repo(tmp_path)
        fake = FakeIntermute([_reservation("other", "README.md", reason="rewriting")], AGENTS)
        try:
            proc = self._run(repo, fake, "README.md")
        finally:
            fake.stop()
        assert proc.returncode == 0, proc.stderr
        out = json.loads(proc.stdout.strip().splitlines()[-1])
        assert out["decision"] == "block"
        assert "peer" in out["reason"] and "rewriting" in out["reason"]
        assert 'request_release(agent_name="peer")' in out["reason"]

    def test_same_name_hold_is_allowed(self, tmp_path):
        repo = _git_repo(tmp_path)
        fake = FakeIntermute([_reservation("twin", "README.md")], AGENTS)
        try:
            proc = self._run(repo, fake, "README.md")
        finally:
            fake.stop()
        assert proc.returncode == 0, proc.stderr
        assert proc.stdout.strip() == "", proc.stdout

    def test_free_file_is_auto_reserved(self, tmp_path):
        repo = _git_repo(tmp_path)
        fake = FakeIntermute([], AGENTS)
        try:
            proc = self._run(repo, fake, "README.md")
        finally:
            fake.stop()
        assert proc.returncode == 0, proc.stderr
        assert proc.stdout.strip() == "", proc.stdout
        reserves = [p for p in fake.posts if p[0] == "/api/reservations"]
        assert len(reserves) == 1, fake.posts
        assert reserves[0][1]["path_pattern"] == "README.md"
        assert reserves[0][1]["agent_id"] == "us"

    def test_race_lost_at_reserve_time_blocks(self, tmp_path):
        """The check sees nothing, but by the time the hook reserves, someone
        else holds the file: intermute's 409 must become a block, not silence."""
        repo = _git_repo(tmp_path)
        fake = FakeIntermute([], AGENTS)
        fake.post_only_reservations = [_reservation("other", "README.md", reason="late")]
        try:
            proc = self._run(repo, fake, "README.md")
        finally:
            fake.stop()
        assert proc.returncode == 0, proc.stderr
        out = json.loads(proc.stdout.strip().splitlines()[-1])
        assert out["decision"] == "block"
        assert "peer" in out["reason"] and "late" in out["reason"]
