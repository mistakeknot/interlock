"""Structural tests for interlock plugin."""
import json
import os
import re
import subprocess
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
        assert "interlock-mcp" in srv["command"]


class TestDirectoryStructure:
    @pytest.mark.parametrize("dirname", [
        ".claude-plugin", "cmd", "internal", "hooks", "scripts",
        "commands", "skills", "tests", "bin",
    ])
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
    EXPECTED_TOOLS = sorted([
        "reserve_files", "release_files", "release_all",
        "check_conflicts", "my_reservations",
        "send_message", "fetch_inbox", "list_agents", "request_release",
    ])

    def test_tool_count(self, project_root):
        names = self._find_tool_names(project_root)
        assert len(names) == 9, f"Expected 9 tools, found {len(names)}: {names}"

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
            assert desc_count >= tool_count, f"{f.name}: {tool_count} tools but only {desc_count} descriptions"

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

    def test_pretooluse_matches_edit(self, project_root):
        with open(project_root / "hooks" / "hooks.json") as f:
            data = json.load(f)
        assert data["hooks"]["PreToolUse"][0]["matcher"] == "Edit"

    def test_sessionstart_is_async(self, project_root):
        with open(project_root / "hooks" / "hooks.json") as f:
            data = json.load(f)
        assert data["hooks"]["SessionStart"][0]["hooks"][0].get("async") is True

    @pytest.mark.parametrize("script", [
        "hooks/lib.sh", "hooks/session-start.sh", "hooks/pre-edit.sh", "hooks/stop.sh",
        "scripts/interlock-register.sh", "scripts/interlock-check.sh",
        "scripts/interlock-cleanup.sh", "scripts/interlock-signal.sh",
    ])
    def test_script_syntax(self, project_root, script):
        path = project_root / script
        assert path.is_file(), f"Missing: {script}"
        result = subprocess.run(["bash", "-n", str(path)], capture_output=True)
        assert result.returncode == 0, f"{script} syntax error: {result.stderr.decode()}"

    @pytest.mark.parametrize("script", [
        "hooks/session-start.sh", "hooks/pre-edit.sh", "hooks/stop.sh",
        "scripts/interlock-register.sh", "scripts/interlock-check.sh",
        "scripts/interlock-cleanup.sh", "scripts/interlock-signal.sh",
    ])
    def test_script_executable(self, project_root, script):
        assert os.access(project_root / script, os.X_OK), f"{script} not executable"

    def test_hooks_source_lib(self, project_root):
        for name in ["session-start.sh", "pre-edit.sh", "stop.sh"]:
            content = (project_root / "hooks" / name).read_text()
            assert "lib.sh" in content

    def test_hooks_exit_zero(self, project_root):
        for name in ["session-start.sh", "pre-edit.sh", "stop.sh"]:
            lines = (project_root / "hooks" / name).read_text().strip().split('\n')
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
        lines = (project_root / "skills" / skill / "SKILL.md").read_text().count('\n')
        assert lines < 100, f"{skill} has {lines} lines"

    @pytest.mark.parametrize("skill", ["coordination-protocol", "conflict-recovery"])
    def test_skill_has_frontmatter(self, project_root, skill):
        content = (project_root / "skills" / skill / "SKILL.md").read_text()
        assert content.startswith("---")
        assert "name:" in content
        assert "description:" in content

    def test_coordination_references_all_tools(self, project_root):
        content = (project_root / "skills" / "coordination-protocol" / "SKILL.md").read_text()
        for tool in ["reserve_files", "release_files", "release_all", "check_conflicts",
                      "my_reservations", "send_message", "fetch_inbox", "list_agents", "request_release"]:
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
