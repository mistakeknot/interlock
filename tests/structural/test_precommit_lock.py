"""The commit lock works from a git worktree (Sylveste-asqi).

The lock directory used to be made under $GIT_ROOT/.git, which inside a
worktree is the gitdir pointer file, so every worktree commit spun until the
timeout. These tests run the real hook in a worktree of a throwaway repo with
intermute unreachable (the hook is fail-open there) and a two-second lock
timeout; the last test runs the pre-fix hook from history to show the fixture
bites.
"""

import os
import shutil
import subprocess
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[2]
HOOK = REPO / "scripts" / "interlock-precommit-hook"
PRE_FIX_COMMIT = "d635136"  # origin/main before the fix (0.2.19)

pytestmark = pytest.mark.skipif(
    shutil.which("jq") is None or shutil.which("curl") is None,
    reason="jq and curl are required to run the hook scripts",
)


def _git(cwd: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(cwd), *args], check=True, capture_output=True, text=True,
    ).stdout.strip()


@pytest.fixture()
def worktree(tmp_path: Path) -> tuple[Path, Path]:
    main = tmp_path / "main"
    main.mkdir()
    subprocess.run(["git", "init", "-q", "-b", "main", str(main)], check=True)
    _git(main, "config", "user.email", "t@t")
    _git(main, "config", "user.name", "t")
    _git(main, "config", "commit.gpgsign", "false")
    (main / "a.txt").write_text("a\n")
    _git(main, "add", "-A")
    _git(main, "commit", "-q", "-m", "init")
    wt = tmp_path / "wt"
    _git(main, "worktree", "add", "-q", "-b", "side", str(wt))
    (wt / "a.txt").write_text("b\n")
    _git(wt, "add", "a.txt")
    return main, wt


def _env() -> dict:
    env = dict(os.environ)
    env.update({
        "INTERMUTE_URL": "http://127.0.0.1:9",
        "INTERMUTE_SOCKET": "/nonexistent/intermute.sock",
        "INTERMUTE_PROJECT": "lock-test",
        "INTERMUTE_AGENT_ID": "tester",
        "INTERLOCK_COMMIT_LOCK_TIMEOUT": "2",
    })
    return env


def _run_hook(hook: Path, cwd: Path) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["bash", str(hook)], cwd=str(cwd), env=_env(), capture_output=True, text=True, timeout=60,
    )


def _common(wt: Path) -> Path:
    return Path(_git(wt, "rev-parse", "--path-format=absolute", "--git-common-dir"))


def test_the_hook_passes_a_worktree_commit(worktree):
    _main, wt = worktree
    r = _run_hook(HOOK, wt)
    assert r.returncode == 0, r.stderr
    assert "Commit lock timeout" not in r.stderr
    assert not (_common(wt) / "commit.lock.d").exists(), "the lock was not released"


def test_the_hook_still_passes_in_the_main_checkout(worktree):
    main, _wt = worktree
    (main / "b.txt").write_text("x\n")
    _git(main, "add", "b.txt")
    r = _run_hook(HOOK, main)
    assert r.returncode == 0, r.stderr
    assert "Commit lock timeout" not in r.stderr
    assert not (main / ".git" / "commit.lock.d").exists()


def test_a_live_holder_in_the_common_dir_blocks_the_worktree(worktree):
    """One lock for every worktree: a holder that is still alive makes a
    worktree commit wait and time out, so two checkouts of one repo still
    serialize."""
    _main, wt = worktree
    d = _common(wt) / "commit.lock.d"
    d.mkdir()
    (d / "pid").write_text(str(os.getpid()))
    try:
        r = _run_hook(HOOK, wt)
    finally:
        shutil.rmtree(d, ignore_errors=True)
    assert r.returncode == 1
    assert "Commit lock timeout" in r.stderr


@pytest.fixture()
def pre_fix_hook(tmp_path: Path) -> Path:
    p = subprocess.run(
        ["git", "-C", str(REPO), "show", f"{PRE_FIX_COMMIT}:scripts/interlock-precommit-hook"],
        capture_output=True, text=True,
    )
    if p.returncode != 0:
        pytest.skip("pre-fix hook is not in this clone's history")
    hook = tmp_path / "pre-fix-hook"
    hook.write_text(p.stdout.replace("LOCK_TIMEOUT=30\n", "LOCK_TIMEOUT=2\n"))
    return hook


def test_the_fixture_bites_on_the_pre_fix_hook(worktree, pre_fix_hook):
    """Control: the hook before the fix fails this exact fixture."""
    _main, wt = worktree
    r = _run_hook(pre_fix_hook, wt)
    assert r.returncode == 1, r.stderr
    assert "Commit lock timeout" in r.stderr
