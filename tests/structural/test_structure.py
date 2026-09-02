"""Structural tests for interlock plugin."""

import json
import os
import re
import subprocess
import tempfile
from pathlib import Path

import pytest

REPO = Path(__file__).resolve().parents[2]


class TestPluginManifest:
    def test_valid_json(self, plugin_json):
        assert plugin_json["name"] == "interlock"

    def test_has_version(self, plugin_json):
        assert "version" in plugin_json

    def test_has_description(self, plugin_json):
        assert len(plugin_json.get("description", "")) > 10

    def test_mcp_server_declared(self, plugin_json):
        assert "interlock" in plugin_json["mcpServers"]
        srv = plugin_json["mcpServers"]["interlock"]
        assert srv["type"] == "stdio"
        assert "interlock-mcp" in srv["command"] or "launch-mcp" in srv["command"]


class TestDirectoryStructure:
    @pytest.mark.parametrize(
        "dirname",
        [
            ".claude-plugin",
            "cmd",
            "internal",
            "hooks",
            "scripts",
            "commands",
            "skills",
            "tests",
            "bin",
        ],
    )
    def test_required_directory(self, project_root, dirname):
        assert (project_root / dirname).is_dir()

    def test_claude_md_exists(self, project_root):
        assert (project_root / "CLAUDE.md").is_file()

    def test_go_mod_exists(self, project_root):
        assert (project_root / "go.mod").is_file()
        content = (project_root / "go.mod").read_text()
        assert "github.com/mistakeknot/interlock" in content

    def test_marker_file(self, project_root):
        marker = project_root / "scripts" / "interlock.sh"
        assert marker.is_file()
        assert os.access(marker, os.X_OK)


class TestMCPTools:
    EXPECTED_TOOLS = sorted(
        [
            # Reservation tools
            "reserve_files",
            "release_files",
            "release_all",
            "check_conflicts",
            "my_reservations",
            # Messaging tools
            "send_message",
            "fetch_inbox",
            "broadcast_message",
            "list_topic_messages",
            # Agent / window identity tools
            "list_agents",
            "list_window_identities",
            "rename_window",
            "expire_window",
            # Release negotiation tools
            "request_release",
            "negotiate_release",
            "respond_to_release",
            "force_release_negotiation",
            "fetch_stale_acks",
            # Contact policy tools
            "get_contact_policy",
            "set_contact_policy",
        ]
    )

    def test_tool_count(self, project_root):
        names = self._find_tool_names(project_root)
        assert len(names) == 20, f"Expected 20 tools, found {len(names)}: {names}"

    def test_tool_names(self, project_root):
        names = self._find_tool_names(project_root)
        assert names == self.EXPECTED_TOOLS

    def test_each_tool_has_description(self, project_root):
        tools_dir = project_root / "internal" / "tools"
        for f in tools_dir.glob("*.go"):
            if f.name.endswith("_test.go"):
                continue
            content = f.read_text()
            # Every NewTool should have WithDescription
            tool_count = content.count("mcp.NewTool(")
            desc_count = content.count("mcp.WithDescription(")
            assert (
                desc_count >= tool_count
            ), f"{f.name}: {tool_count} tools but only {desc_count} descriptions"

    def _find_tool_names(self, project_root):
        tools_dir = project_root / "internal" / "tools"
        names = []
        for f in tools_dir.glob("*.go"):
            if f.name.endswith("_test.go"):
                continue
            for m in re.finditer(r'mcp\.NewTool\(\s*"(\w+)"', f.read_text()):
                names.append(m.group(1))
        return sorted(names)


class TestHooks:
    def test_hooks_json_valid(self, project_root):
        with open(project_root / "hooks" / "hooks.json") as f:
            data = json.load(f)
        assert "hooks" in data
        assert "SessionStart" in data["hooks"]
        assert "PreToolUse" in data["hooks"]
        assert "Stop" in data["hooks"]

    def test_pretooluse_matches_file_mutations(self, project_root):
        with open(project_root / "hooks" / "hooks.json") as f:
            data = json.load(f)
        assert data["hooks"]["PreToolUse"][0]["matcher"] == "Edit|Write|MultiEdit"

    def test_sessionstart_is_async(self, project_root):
        with open(project_root / "hooks" / "hooks.json") as f:
            data = json.load(f)
        assert data["hooks"]["SessionStart"][0]["hooks"][0].get("async") is True

    @pytest.mark.parametrize(
        "script",
        [
            "hooks/lib.sh",
            "hooks/session-start.sh",
            "hooks/pre-edit.sh",
            "hooks/stop.sh",
            "scripts/interlock-register.sh",
            "scripts/interlock-check.sh",
            "scripts/interlock-cleanup.sh",
            "scripts/interlock-signal.sh",
            "scripts/interlock-precommit-hook",
            "scripts/interlock-postcommit-hook",
            "scripts/interlock-install-hooks",
        ],
    )
    def test_script_syntax(self, project_root, script):
        path = project_root / script
        assert path.is_file(), f"Missing: {script}"
        result = subprocess.run(["bash", "-n", str(path)], capture_output=True)
        assert (
            result.returncode == 0
        ), f"{script} syntax error: {result.stderr.decode()}"

    @pytest.mark.parametrize(
        "script",
        [
            "hooks/session-start.sh",
            "hooks/pre-edit.sh",
            "hooks/stop.sh",
            "scripts/interlock-register.sh",
            "scripts/interlock-check.sh",
            "scripts/interlock-cleanup.sh",
            "scripts/interlock-signal.sh",
            "scripts/interlock-precommit-hook",
            "scripts/interlock-postcommit-hook",
        ],
    )
    def test_script_executable(self, project_root, script):
        assert os.access(project_root / script, os.X_OK), f"{script} not executable"

    def test_hooks_source_lib(self, project_root):
        for name in ["session-start.sh", "pre-edit.sh", "stop.sh"]:
            content = (project_root / "hooks" / name).read_text()
            assert "lib.sh" in content

    def test_hooks_exit_zero(self, project_root):
        for name in ["session-start.sh", "pre-edit.sh", "stop.sh"]:
            lines = (project_root / "hooks" / name).read_text().strip().split("\n")
            assert lines[-1].strip() == "exit 0"

    def test_session_start_checks_join(self, project_root):
        content = (project_root / "hooks" / "session-start.sh").read_text()
        assert "is_joined" in content

    def test_pre_edit_checks_agent_id(self, project_root):
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "INTERMUTE_AGENT_ID" in content

    def test_stop_checks_agent_id(self, project_root):
        content = (project_root / "hooks" / "stop.sh").read_text()
        assert "INTERMUTE_AGENT_ID" in content

    def test_stop_has_active_guard(self, project_root):
        content = (project_root / "hooks" / "stop.sh").read_text()
        assert "stop_hook_active" in content


class TestCommands:
    @pytest.mark.parametrize("cmd", ["join.md", "leave.md", "status.md", "setup.md"])
    def test_command_exists(self, project_root, cmd):
        assert (project_root / "commands" / cmd).is_file()

    @pytest.mark.parametrize("cmd", ["join.md", "leave.md", "status.md", "setup.md"])
    def test_command_has_frontmatter(self, project_root, cmd):
        content = (project_root / "commands" / cmd).read_text()
        assert content.startswith("---")
        assert "name:" in content
        assert "description:" in content


class TestSkills:
    @pytest.mark.parametrize("skill", ["coordination-protocol", "conflict-recovery"])
    def test_skill_exists(self, project_root, skill):
        assert (project_root / "skills" / skill / "SKILL.md").is_file()

    @pytest.mark.parametrize("skill", ["coordination-protocol", "conflict-recovery"])
    def test_skill_under_100_lines(self, project_root, skill):
        lines = (project_root / "skills" / skill / "SKILL.md").read_text().count("\n")
        assert lines < 100, f"{skill} has {lines} lines"

    @pytest.mark.parametrize("skill", ["coordination-protocol", "conflict-recovery"])
    def test_skill_has_frontmatter(self, project_root, skill):
        content = (project_root / "skills" / skill / "SKILL.md").read_text()
        assert content.startswith("---")
        assert "name:" in content
        assert "description:" in content

    def test_coordination_references_all_tools(self, project_root):
        content = (
            project_root / "skills" / "coordination-protocol" / "SKILL.md"
        ).read_text()
        for tool in [
            "reserve_files",
            "release_files",
            "release_all",
            "check_conflicts",
            "my_reservations",
            "send_message",
            "fetch_inbox",
            "list_agents",
            "request_release",
            "negotiate_release",
            "respond_to_release",
        ]:
            assert tool in content, f"Missing tool reference: {tool}"


class TestGitHook:
    def test_precommit_hook_exists(self, project_root):
        assert (project_root / "scripts" / "interlock-precommit-hook").is_file()

    def test_installer_exists(self, project_root):
        assert (project_root / "scripts" / "interlock-install-hooks").is_file()

    def test_precommit_has_marker(self, project_root):
        content = (project_root / "scripts" / "interlock-precommit-hook").read_text()
        assert "INTERLOCK_HOOK_MARKER" in content

    def test_precommit_checks_agent(self, project_root):
        content = (project_root / "scripts" / "interlock-precommit-hook").read_text()
        assert "INTERMUTE_AGENT_ID" in content

    def test_precommit_uses_git_diff(self, project_root):
        content = (project_root / "scripts" / "interlock-precommit-hook").read_text()
        assert "git diff --cached --name-only" in content

    def test_precommit_queries_api(self, project_root):
        content = (project_root / "scripts" / "interlock-precommit-hook").read_text()
        assert "/api/reservations" in content

    def test_precommit_has_resolve_hint(self, project_root):
        content = (project_root / "scripts" / "interlock-precommit-hook").read_text()
        assert "no-verify" in content


class TestMandatoryReservations:
    """Tests for Phase 2 mandatory reservation features."""

    def test_pre_edit_blocks_on_conflict(self, project_root):
        """Edit hook must return blocking decision, not just advisory context."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert '"decision": "block"' in content or '"decision":"block"' in content

    def test_pre_edit_no_longer_advisory(self, project_root):
        """Edit hook must not contain the old advisory-only comment."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "advisory conflict warning (never blocks)" not in content.lower()

    def test_pre_edit_auto_reserves(self, project_root):
        """Edit hook must auto-reserve files via Intermute API."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "/api/reservations" in content
        assert "auto-reserve" in content
        assert "ttl_minutes" in content

    def test_pre_edit_uses_exclusive_reservation(self, project_root):
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "exclusive" in content

    def test_postcommit_releases_reservations(self, project_root):
        """Post-commit hook must release reservations for committed files."""
        content = (project_root / "scripts" / "interlock-postcommit-hook").read_text()
        assert "Auto-release" in content or "auto-release" in content
        assert "DELETE" in content
        assert "/api/reservations/" in content

    def test_postcommit_only_releases_committed_files(self, project_root):
        """Post-commit should match committed files to reservations, not release all."""
        content = (project_root / "scripts" / "interlock-postcommit-hook").read_text()
        assert "CHANGED_FILES" in content
        assert "path_pattern" in content  # jq filter matching against patterns


class TestMultiSessionCoordination:
    """Tests for Phase 1 multi-session git safety features."""

    def test_session_start_does_not_install_git_function_wrapper(self, project_root):
        # Regression test for sylveste-4pth: git function wrappers with a
        # per-session index can still commit stale trees. Worktree isolation
        # must not reintroduce that wrapper.
        content = (project_root / "hooks" / "session-start.sh").read_text()
        assert "export GIT_INDEX_FILE=" not in content
        assert '"export GIT_INDEX_FILE=' not in content
        assert "export -f git" not in content
        assert "git()" not in content

    def test_pre_edit_allows_absolute_worktree_path(self, project_root):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            original = tmp_path / "repo"
            worktree = tmp_path / "worktree"
            original.mkdir()
            worktree.mkdir()
            subprocess.run(["git", "init", "-b", "main"], cwd=worktree, check=True)

            fake_bin = tmp_path / "bin"
            fake_bin.mkdir()
            (fake_bin / "curl").write_text("#!/usr/bin/env bash\nexit 1\n")
            (fake_bin / "curl").chmod(0o755)
            (fake_bin / "ic").write_text("#!/usr/bin/env bash\nexit 2\n")
            (fake_bin / "ic").chmod(0o755)

            env = os.environ.copy()
            env["HOME"] = str(tmp_path / "home")
            env["PATH"] = f"{fake_bin}:{env['PATH']}"
            env["INTERMUTE_AGENT_ID"] = "agent-1"
            env["CLAUDE_SESSION_ID"] = "session-1"
            env["INTERLOCK_PROJECT_ROOT"] = str(original)
            env["INTERLOCK_SESSION_WORKTREE"] = str(worktree)

            result = subprocess.run(
                [str(project_root / "hooks" / "pre-edit.sh")],
                cwd=worktree,
                input=json.dumps(
                    {
                        "cwd": str(worktree),
                        "tool_input": {"file_path": str(worktree / "README.md")},
                    }
                ),
                text=True,
                capture_output=True,
                env=env,
            )
            assert result.returncode == 0, result.stderr
            assert '"decision":"block"' not in result.stdout

    def test_precommit_has_commit_lock(self, project_root):
        content = (project_root / "scripts" / "interlock-precommit-hook").read_text()
        assert "commit.lock" in content
        assert "acquire_commit_lock" in content
        assert "release_commit_lock" in content

    def test_precommit_lock_has_timeout(self, project_root):
        content = (project_root / "scripts" / "interlock-precommit-hook").read_text()
        assert "LOCK_TIMEOUT" in content

    def test_precommit_lock_has_stale_detection(self, project_root):
        content = (project_root / "scripts" / "interlock-precommit-hook").read_text()
        assert "kill -0" in content  # PID liveness check

    def test_precommit_does_not_refresh_session_index(self, project_root):
        content = (project_root / "scripts" / "interlock-precommit-hook").read_text()
        assert "GIT_INDEX_FILE" not in content
        assert "git read-tree HEAD" not in content

    def test_postcommit_hook_exists(self, project_root):
        assert (project_root / "scripts" / "interlock-postcommit-hook").is_file()

    def test_postcommit_hook_executable(self, project_root):
        assert os.access(
            project_root / "scripts" / "interlock-postcommit-hook", os.X_OK
        )

    def test_postcommit_has_marker(self, project_root):
        content = (project_root / "scripts" / "interlock-postcommit-hook").read_text()
        assert "INTERLOCK_HOOK_MARKER" in content

    def test_postcommit_does_not_refresh_session_index(self, project_root):
        content = (project_root / "scripts" / "interlock-postcommit-hook").read_text()
        assert "GIT_INDEX_FILE" not in content
        assert "git read-tree HEAD" not in content

    def test_postcommit_broadcasts_commit(self, project_root):
        content = (project_root / "scripts" / "interlock-postcommit-hook").read_text()
        assert "/api/messages" in content
        assert "/api/agents" in content

    def test_postcommit_syntax(self, project_root):
        path = project_root / "scripts" / "interlock-postcommit-hook"
        result = subprocess.run(["bash", "-n", str(path)], capture_output=True)
        assert result.returncode == 0, f"Syntax error: {result.stderr.decode()}"

    def test_installer_handles_postcommit(self, project_root):
        content = (project_root / "scripts" / "interlock-install-hooks").read_text()
        assert "post-commit" in content
        assert "interlock-postcommit-hook" in content


class TestSignals:
    def test_signal_script_exists(self, project_root):
        assert (project_root / "scripts" / "interlock-signal.sh").is_file()

    def test_signal_script_executable(self, project_root):
        assert os.access(project_root / "scripts" / "interlock-signal.sh", os.X_OK)

    def test_signal_uses_strict_mode(self, project_root):
        content = (project_root / "scripts" / "interlock-signal.sh").read_text()
        assert "set -euo pipefail" in content

    def test_hooks_emit_signals(self, project_root):
        for hook in ["session-start.sh", "stop.sh"]:
            content = (project_root / "hooks" / hook).read_text()
            assert "interlock-signal.sh" in content, f"{hook} must emit signals"


class TestWorkflowIntegration:
    """Tests for Phase 3 workflow integration features."""

    def test_pre_edit_checks_inbox(self, project_root):
        """Edit hook should check inbox for commit notifications."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "inbox" in content.lower() or "/api/messages" in content

    def test_pre_edit_has_pull_logic(self, project_root):
        """Edit hook should pull when commit messages found."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "git pull --rebase" in content

    def test_pre_edit_has_pull_cache(self, project_root):
        """Edit hook should throttle inbox checks to avoid per-edit latency."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "inbox_check_path" in content or "interlock-pull-checked" in content

    def test_pre_edit_has_rebase_abort_fallback(self, project_root):
        """Edit hook should abort rebase on conflict rather than blocking."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "rebase --abort" in content

    def test_lib_has_inbox_helper(self, project_root):
        """lib.sh should have the inbox_check_path helper for throttle flag files."""
        content = (project_root / "hooks" / "lib.sh").read_text()
        assert "inbox_check_path" in content


class TestNegotiationProtocol:
    """Tests for Phase 4a negotiation protocol features."""

    def test_pre_edit_has_auto_release_flag(self, project_root):
        """Pre-edit hook must check INTERLOCK_AUTO_RELEASE feature flag."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "INTERLOCK_AUTO_RELEASE" in content

    def test_pre_edit_has_advisory_release(self, project_root):
        """Pre-edit hook must use advisory mode (additionalContext), not auto-delete."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert "additionalContext" in content
        assert "respond_to_release" in content

    def test_pre_edit_has_negotiation_throttle(self, project_root):
        """Pre-edit hook must throttle release-request inbox checks."""
        content = (project_root / "hooks" / "pre-edit.sh").read_text()
        assert (
            "negotiation_check_path" in content
            or "interlock-negotiate-checked" in content
        )

    def test_lib_has_negotiation_check_path(self, project_root):
        """lib.sh should have the negotiation_check_path helper."""
        content = (project_root / "hooks" / "lib.sh").read_text()
        assert "negotiation_check_path" in content

    def test_lib_has_fast_curl(self, project_root):
        """lib.sh should have intermute_curl_fast with circuit breaker timeout."""
        content = (project_root / "hooks" / "lib.sh").read_text()
        assert "intermute_curl_fast" in content
        assert "max-time" in content

    def test_tools_have_exported_constants(self, project_root):
        """Client should export timeout constants for tools layer."""
        content = (project_root / "internal" / "client" / "client.go").read_text()
        assert "NormalTimeoutMinutes" in content
        assert "UrgentTimeoutMinutes" in content
        assert "NegotiationPollInterval" in content

    def test_advisory_timeout_no_force_release(self, project_root):
        """CheckExpiredNegotiations must NOT call ReleaseByPattern (advisory-only)."""
        content = (project_root / "internal" / "client" / "client.go").read_text()
        # Find the CheckExpiredNegotiations function and verify no ReleaseByPattern call
        in_func = False
        found_advisory_comment = False
        for line in content.splitlines():
            if "func (c *Client) CheckExpiredNegotiations" in line:
                in_func = True
            elif in_func:
                if line.startswith("func ") or (
                    line.startswith("}") and not line.startswith("})")
                ):
                    break
                assert (
                    "ReleaseByPattern" not in line
                ), "CheckExpiredNegotiations must not call ReleaseByPattern (advisory-only)"
                if "advisory" in line.lower() or "Advisory" in line:
                    found_advisory_comment = True
        assert (
            found_advisory_comment
        ), "CheckExpiredNegotiations should have advisory comment"


class TestVersionAgreement:
    def test_main_go_version_matches_manifest(self, project_root):
        import json, re
        manifest = json.loads((project_root / ".claude-plugin" / "plugin.json").read_text())["version"]
        kimi = json.loads((project_root / "kimi.plugin.json").read_text())["version"]
        main_go = (project_root / "cmd" / "interlock-mcp" / "main.go").read_text()
        m = re.search(r'var version = "([^"]+)"', main_go)
        assert m, "main.go must declare var version"
        assert m.group(1) == manifest == kimi


class TestToolDocumentation:
    def test_every_tool_is_documented_in_readme(self, project_root):
        import re
        tools_go = (project_root / "internal" / "tools" / "tools.go").read_text()
        tools = set(re.findall(r'mcp\.NewTool\("([a-z_]+)"', tools_go))
        readme = (project_root / "README.md").read_text()
        missing = sorted(t for t in tools if f"`{t}`" not in readme)
        assert tools, "no tools found in tools.go"
        assert not missing, f"tools missing from README.md: {missing}"
