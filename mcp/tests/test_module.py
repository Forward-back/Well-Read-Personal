#!/usr/bin/env python3
"""
Gaze MCP Server 模组测试脚本

测试模组的各种启动方式和功能。可运行: uv run pytest -q tests/test_module.py
"""

import os
import subprocess
import sys
import unittest
from pathlib import Path

MCP_SERVER_DIR = Path(__file__).resolve().parents[1]

REQUIRED_FILES = [
    "main.py",
    "run_server.py",
    "gaze_mcp_server.py",
    "client.py",
    "tools.py",
    "requirements.txt",
    "setup.py",
    "pyproject.toml",
    "README.md",
    "docs/安装与运行.md",
    "LICENSE",
    "MANIFEST.in",
]


class ModuleIntegrationTest(unittest.TestCase):
    def test_imports(self):
        import mcp  # noqa: F401
        import requests  # noqa: F401
        import gaze_mcp_server  # noqa: F401
        import tools  # noqa: F401
        from client import GazeClient  # noqa: F401
        from gaze_mcp_server import run  # noqa: F401
        import main  # noqa: F401

    def test_environment_optional_vars(self):
        os.getenv("GAZE_BASE_URL")
        os.getenv("GAZE_API_KEY")

    def test_client_creation(self):
        from client import GazeClient

        base_url = os.getenv("GAZE_BASE_URL", "http://localhost:8080/api/v1")
        api_key = os.getenv("GAZE_API_KEY", "test_key")
        client = GazeClient(base_url, api_key)
        self.assertEqual(client.base_url, base_url)
        self.assertEqual(client.api_key, api_key)

    def test_required_files_exist(self):
        missing = [name for name in REQUIRED_FILES if not (MCP_SERVER_DIR / name).exists()]
        self.assertEqual(missing, [], f"Missing files: {missing}")

    def test_main_help(self):
        result = subprocess.run(
            [sys.executable, "main.py", "--help"],
            capture_output=True,
            text=True,
            timeout=10,
            cwd=MCP_SERVER_DIR,
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_main_check_only(self):
        result = subprocess.run(
            [sys.executable, "main.py", "--check-only"],
            capture_output=True,
            text=True,
            timeout=10,
            cwd=MCP_SERVER_DIR,
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_gaze_tools(self):
        from client import GazeClient

        client = GazeClient("http://localhost:8080/api/v1", "test")
        for method in ["gaze_search", "gaze_read_page", "gaze_index_view"]:
            self.assertTrue(hasattr(client, method), f"GazeClient missing: {method}")
            self.assertTrue(callable(getattr(client, method)), f"{method} not callable")

    def test_pyproject_metadata(self):
        text = (MCP_SERVER_DIR / "pyproject.toml").read_text(encoding="utf-8")
        self.assertIn("ylsw-wellread-mcp", text)
        self.assertIn("wellread-server", text)


if __name__ == "__main__":
    unittest.main(verbosity=2)
