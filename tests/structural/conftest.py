"""Shared fixtures for Interlock structural tests."""
import json
from pathlib import Path

import pytest


@pytest.fixture(scope="session")
def project_root() -> Path:
    return Path(__file__).resolve().parent.parent.parent


@pytest.fixture(scope="session")
def plugin_json(project_root: Path) -> dict:
    with open(project_root / ".claude-plugin" / "plugin.json") as f:
        return json.load(f)
