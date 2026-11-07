#!/usr/bin/env python3
"""Regression tests for MCP 2.x transport compatibility."""

import asyncio
import http.client
import json
import os
import subprocess
import sys
import threading
import unittest
from pathlib import Path

MCP_SERVER_DIR = Path(__file__).resolve().parents[1]


class TransportRegressionTest(unittest.TestCase):
    def test_http_transport_is_stateless(self):
        import gaze_mcp_server as srv
        from mcp.server import MCPServer

        probe = MCPServer("probe")
        probe.streamable_http_app(
            host="127.0.0.1", stateless_http=srv.STREAMABLE_HTTP_STATELESS
        )
        self.assertTrue(probe.session_manager.stateless)

    def test_sse_message_path_matches_legacy_mount(self):
        import gaze_mcp_server as srv
        from mcp.server import MCPServer
        from starlette.routing import Mount, Route

        probe = MCPServer("probe")
        app = probe.sse_app(host="127.0.0.1", message_path=srv.SSE_MESSAGE_PATH)
        mount_paths = [
            route.path
            for route in app.routes
            if isinstance(route, (Mount, Route))
        ]
        self.assertIn("/sse", mount_paths)
        self.assertIn(
            srv.SSE_MESSAGE_PATH.rstrip("/"),
            {path.rstrip("/") for path in mount_paths},
        )

    def test_gaze_client_session_is_thread_local(self):
        from client import GazeClient

        client = GazeClient("http://localhost:8080/api/v1", "test-key")
        barrier = threading.Barrier(2)
        sessions: dict[str, object] = {}

        def worker(name: str) -> None:
            barrier.wait()
            sessions[name] = client.session

        threads = [
            threading.Thread(target=worker, args=(name,))
            for name in ("a", "b")
        ]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()

        self.assertEqual(len(sessions), 2)
        self.assertIsNot(sessions["a"], sessions["b"])


class StdioToolsListTest(unittest.TestCase):
    def test_tools_list_returns_17_read_only_tools(self):
        async def _run() -> int:
            from mcp import ClientSession, StdioServerParameters
            from mcp.client.stdio import stdio_client

            params = StdioServerParameters(
                command=sys.executable,
                args=[str(MCP_SERVER_DIR / "gaze_mcp_server.py")],
                env={
                    **os.environ,
                    "GAZE_API_KEY": "test-key",
                },
            )
            async with stdio_client(params) as (read, write):
                async with ClientSession(read, write) as session:
                    await session.initialize()
                    tools = await session.list_tools()
                    return len(tools.tools)

        count = asyncio.run(_run())
        self.assertEqual(count, 17)


class HttpStatelessSmokeTest(unittest.TestCase):
    def test_initialize_does_not_require_mcp_session_id(self):
        import time

        port = 19876
        env = {
            **os.environ,
            "MCP_SERVER_AUTH_TOKEN": "test-token",
            "GAZE_API_KEY": "test-key",
        }
        proc = subprocess.Popen(
            [
                sys.executable,
                "gaze_mcp_server.py",
                "--transport",
                "http",
                "--host",
                "127.0.0.1",
                "--port",
                str(port),
            ],
            cwd=MCP_SERVER_DIR,
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        try:
            deadline = time.time() + 10
            status = None
            headers = {}
            while time.time() < deadline:
                try:
                    conn = http.client.HTTPConnection("127.0.0.1", port, timeout=2)
                    conn.request(
                        "POST",
                        "/mcp",
                        body=json.dumps(
                            {
                                "jsonrpc": "2.0",
                                "id": 1,
                                "method": "initialize",
                                "params": {
                                    "protocolVersion": "2025-03-26",
                                    "capabilities": {},
                                    "clientInfo": {"name": "t", "version": "1"},
                                },
                            }
                        ),
                        headers={
                            "Authorization": "Bearer test-token",
                            "Content-Type": "application/json",
                            "Accept": "application/json, text/event-stream",
                        },
                    )
                    response = conn.getresponse()
                    status = response.status
                    headers = {k.lower(): v for k, v in response.getheaders()}
                    response.read()
                    conn.close()
                    break
                except OSError:
                    time.sleep(0.2)
            else:
                self.fail("HTTP server did not become ready in time")

            self.assertEqual(status, 200)
            self.assertNotIn("mcp-session-id", headers)
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()


if __name__ == "__main__":
    unittest.main()
