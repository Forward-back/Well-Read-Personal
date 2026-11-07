#!/usr/bin/env python3
"""WellRead MCP server for the personal read-only API key."""

import argparse
import asyncio
import logging
import os
import secrets
import sys

from mcp.server import MCPServer

from client import GazeClient, env_value
from tools import register_tools

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


BASE_URL = env_value("BASE_URL", "http://localhost:8080/api/v1")
API_KEY = env_value("API_KEY", "")

# Network transport defaults kept for backward compatibility with pre-2.x deployments.
SSE_MESSAGE_PATH = "/sse/messages/"
STREAMABLE_HTTP_STATELESS = True


def network_transport_auth_token() -> str:
    """Shared secret clients must present for SSE/HTTP transports."""
    return os.getenv("MCP_SERVER_AUTH_TOKEN", "").strip()


def require_network_transport_auth(transport: str) -> str:
    """SSE/HTTP must not start without a configured auth token."""
    token = network_transport_auth_token()
    if transport in ("sse", "http") and not token:
        logger.error(
            "MCP_SERVER_AUTH_TOKEN is required for %s transport. "
            "Set a strong shared secret; clients must send "
            "Authorization: Bearer <token> or X-MCP-Auth-Token.",
            transport,
        )
        sys.exit(1)
    return token


class MCPAuthMiddleware:
    """ASGI middleware that gates network MCP transports behind a shared secret."""

    def __init__(self, app, token: str):
        self.app = app
        self.token = token

    async def __call__(self, scope, receive, send):
        if scope.get("type") != "http":
            await self.app(scope, receive, send)
            return

        headers = {
            k.decode("latin-1").lower(): v.decode("latin-1")
            for k, v in scope.get("headers", [])
        }
        provided = ""
        auth = headers.get("authorization", "")
        if auth.lower().startswith("bearer "):
            provided = auth[7:].strip()
        elif "x-mcp-auth-token" in headers:
            provided = headers["x-mcp-auth-token"]

        if not provided or not secrets.compare_digest(provided, self.token):
            body = b'{"error":"unauthorized"}'
            await send(
                {
                    "type": "http.response.start",
                    "status": 401,
                    "headers": [[b"content-type", b"application/json"]],
                }
            )
            await send({"type": "http.response.body", "body": body})
            return

        await self.app(scope, receive, send)


mcp = MCPServer("gaze-server", version="1.2.0")
client = GazeClient(BASE_URL, API_KEY)
register_tools(mcp, client)


# ---------------------------------------------------------------------------
# Transports
# ---------------------------------------------------------------------------


async def run_stdio():
    """Run the MCP server using stdio transport."""
    await mcp.run_stdio_async()


async def run_sse(host: str, port: int):
    """Run the MCP server using SSE transport (legacy MCP clients)."""
    auth_token = require_network_transport_auth("sse")
    try:
        import uvicorn
    except ImportError as e:
        raise ImportError(
            f"SSE transport requires 'starlette' and 'uvicorn': pip install starlette uvicorn\n{e}"
        ) from e

    starlette_app = MCPAuthMiddleware(
        mcp.sse_app(host=host, message_path=SSE_MESSAGE_PATH),
        auth_token,
    )

    logger.info("Starting SSE MCP server on %s:%d", host, port)
    logger.info("SSE endpoint:  http://%s:%d/sse", host, port)
    logger.info("SSE messages: http://%s:%d%s", host, port, SSE_MESSAGE_PATH)
    config = uvicorn.Config(starlette_app, host=host, port=port, log_level="info")
    server = uvicorn.Server(config)
    await server.serve()


async def run_http(host: str, port: int):
    """Run the MCP server using Streamable HTTP transport (MCP 2025-03-26 spec)."""
    auth_token = require_network_transport_auth("http")
    try:
        import uvicorn
    except ImportError as e:
        raise ImportError(
            f"HTTP transport requires 'starlette' and 'uvicorn': pip install starlette uvicorn\n{e}"
        ) from e

    starlette_app = MCPAuthMiddleware(
        mcp.streamable_http_app(host=host, stateless_http=STREAMABLE_HTTP_STATELESS),
        auth_token,
    )

    logger.info("Starting Streamable HTTP MCP server on %s:%d", host, port)
    logger.info("MCP endpoint:  http://%s:%d/mcp", host, port)
    config = uvicorn.Config(starlette_app, host=host, port=port, log_level="info")
    server = uvicorn.Server(config)
    await server.serve()


# Backward-compatible alias used by run_server.py
run = run_stdio


def main():
    """Main entry point — supports stdio, sse, and http transports."""
    parser = argparse.ArgumentParser(description="WellRead MCP Server")
    parser.add_argument(
        "--transport",
        choices=["stdio", "sse", "http"],
        default=os.getenv("MCP_TRANSPORT", "stdio"),
        help="Transport type: stdio (default), sse, or http",
    )
    parser.add_argument(
        "--host",
        default=os.getenv("MCP_HOST", "127.0.0.1"),
        help="Bind host for network transports (default: 127.0.0.1)",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=int(os.getenv("MCP_PORT", "8000")),
        help="Bind port for network transports (default: 8000)",
    )
    args = parser.parse_args()

    if args.transport == "stdio":
        asyncio.run(run_stdio())
    elif args.transport == "sse":
        asyncio.run(run_sse(args.host, args.port))
    elif args.transport == "http":
        asyncio.run(run_http(args.host, args.port))


if __name__ == "__main__":
    main()
