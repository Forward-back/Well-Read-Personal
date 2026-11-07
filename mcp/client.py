"""Read-only HTTP client for the WellRead API used by MCP tools."""

import json
import logging
import os
import re
import threading
from typing import Any, Dict

import requests
import urllib3
from requests.exceptions import RequestException

logger = logging.getLogger(__name__)


def env_value(name: str, default: str = "") -> str:
    return os.getenv(f"WELLREAD_{name}", os.getenv(f"GAZE_{name}", default))


try:
    CHAT_TIMEOUT = int(env_value("CHAT_TIMEOUT", "300"))
except ValueError:
    logger.warning("WELLREAD_CHAT_TIMEOUT is not a valid integer; falling back to 300s.")
    CHAT_TIMEOUT = 300


def normalize_kb_entries(resp: object) -> list[Dict]:
    """Flatten the knowledge-base list API response into KB rows."""
    data = resp.get("data", resp) if isinstance(resp, dict) else resp
    if isinstance(data, dict):
        data = data.get("list", data.get("items", []))
    out: list[Dict] = []
    for item in (data or []):
        if not isinstance(item, dict):
            continue
        nested = item.get("knowledge_base")
        if isinstance(nested, dict) and nested.get("id"):
            out.append(nested)
        elif item.get("id"):
            out.append(item)
    return out


class GazeClient:
    """Read-only client for the WellRead personal MCP key."""

    # ── UUID pattern (8-4-4-4-12 hex) ──────────────────────────────────────
    _UUID_RE = re.compile(
        r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
        re.IGNORECASE,
    )

    def __init__(self, base_url: str, api_key: str):
        """Initialize the Gaze API client with base URL and authentication"""
        self.base_url = base_url
        self.api_key = api_key
        # SSL verification: enabled by default. Set GAZE_VERIFY_SSL=false to disable
        # (e.g. for self-signed certs in dev environments — NOT recommended for production).
        self.verify_ssl = env_value("VERIFY_SSL", "true").lower() != "false"
        if not self.verify_ssl:
            logger.warning(
                "SSL certificate verification is DISABLED (WELLREAD_VERIFY_SSL=false). "
                "This is insecure and should not be used in production."
            )
            urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
        # MCP 2.x runs sync @mcp.tool() handlers on worker threads; use a
        # thread-local Session because requests.Session is not thread-safe.
        self._session_local = threading.local()

    def _new_session(self) -> requests.Session:
        session = requests.Session()
        session.verify = self.verify_ssl
        session.headers.update(
            {
                "X-API-Key": self.api_key,
                "Content-Type": "application/json",
            }
        )
        return session

    @property
    def session(self) -> requests.Session:
        if not getattr(self._session_local, "session", None):
            self._session_local.session = self._new_session()
        return self._session_local.session

    def _request(self, method: str, endpoint: str, **kwargs) -> Dict[str, Any]:
        """Make a request to the Gaze API

        Args:
            method: HTTP method (GET, POST, PUT, DELETE)
            endpoint: API endpoint path
            **kwargs: Additional arguments to pass to requests

        Returns:
            JSON response as dictionary
        """
        url = f"{self.base_url}{endpoint}"
        try:
            # Execute HTTP request with the specified method
            response = self.session.request(method, url, **kwargs)
            # Raise exception for HTTP error status codes (4xx, 5xx)
            response.raise_for_status()
            # Parse and return JSON response
            return response.json()
        except RequestException as e:
            logger.error(f"API request failed: {e}")
            raise

    def resolve_agent_id(self, agent_id_or_name: str) -> str:
        """Resolve an agent ID or name to its canonical ID.

        If *agent_id_or_name* is already a UUID it is returned unchanged.
        Otherwise all agents are listed and the first one whose ``id``
        matches exactly or whose ``name`` matches case-insensitively is
        returned.
        Raises ValueError when no match is found.
        """
        if self._UUID_RE.match(agent_id_or_name):
            return agent_id_or_name
        resp = self._request("GET", "/agents")
        agents = resp.get("data", resp) if isinstance(resp, dict) else resp
        if isinstance(agents, dict):
            agents = agents.get("list", agents.get("items", []))
        needle = agent_id_or_name.lower()
        for agent in (agents or []):
            if not isinstance(agent, dict):
                continue
            if agent.get("id") == agent_id_or_name:
                return agent["id"]
            if agent.get("name", "").lower() == needle:
                return agent["id"]
        raise ValueError(
            f"Agent {agent_id_or_name!r} not found. "
            "Use list_agents to see available agent IDs and names."
        )

    def resolve_kb_id(self, kb_id_or_name: str) -> str:
        """Resolve a knowledge base name to its UUID if needed.

        If *kb_id_or_name* is already a UUID it is returned unchanged.
        Otherwise all knowledge bases are listed and the first one whose
        ``name`` matches (case-insensitive) is returned.
        Raises ValueError when no match is found.
        """
        if self._UUID_RE.match(kb_id_or_name):
            return kb_id_or_name
        needle = kb_id_or_name.lower()
        for kb in normalize_kb_entries(self.list_knowledge_bases()):
            if kb.get("name", "").lower() == needle:
                return kb["id"]
        raise ValueError(
            f"Knowledge base {kb_id_or_name!r} not found. "
            "Use list_knowledge_bases to see available IDs and names."
        )

    def hybrid_search(self, kb_id: str, query: str, config: Dict) -> Dict:
        """Perform hybrid search combining vector and keyword search"""
        data = {
            "query_text": query,
            **config,  # Include thresholds and match count
        }
        return self._request(
            "POST", f"/knowledge-bases/{kb_id}/hybrid-search", json=data
        )

    # Knowledge Management - Methods for creating and managing knowledge entries
    def list_knowledge_bases(self) -> Dict:
        """List all knowledge bases"""
        return self._request("GET", "/knowledge-bases")

    def get_knowledge_base(self, kb_id: str) -> Dict:
        """Get knowledge base details"""
        return self._request("GET", f"/knowledge-bases/{kb_id}")

    def list_knowledge(self, kb_id: str, page: int = 1, page_size: int = 20) -> Dict:
        """List knowledge in a knowledge base"""
        params = {"page": page, "page_size": page_size}
        return self._request(
            "GET", f"/knowledge-bases/{kb_id}/knowledge", params=params
        )

    def get_knowledge(self, knowledge_id: str) -> Dict:
        """Get knowledge details"""
        return self._request("GET", f"/knowledge/{knowledge_id}")

    def create_session(
        self,
        kb_id: str,
        max_rounds: int = 5,
        enable_rewrite: bool = True,
        fallback_response: str = "Sorry, I cannot answer this question.",
        summary_model_id: str = "",
        title: str = "",
        description: str = "",
    ) -> Dict:
        """Create a new chat session with strategy configuration"""
        strategy = {
            "max_rounds": max_rounds,
            "enable_rewrite": enable_rewrite,
            "fallback_strategy": "FIXED_RESPONSE",
            "fallback_response": fallback_response,
            "embedding_top_k": 10,
            "keyword_threshold": 0.5,
            "vector_threshold": 0.7,
            "summary_model_id": summary_model_id,
        }
        data = {
            "knowledge_base_id": kb_id,
            "session_strategy": strategy,
        }
        if title:
            data["title"] = title
        if description:
            data["description"] = description
        return self._request("POST", "/sessions", json=data)

    def get_session(self, session_id: str) -> Dict:
        """Get session details"""
        return self._request("GET", f"/sessions/{session_id}")

    def list_sessions(self, page: int = 1, page_size: int = 20) -> Dict:
        """List sessions"""
        params = {"page": page, "page_size": page_size}
        return self._request("GET", "/sessions", params=params)

    def delete_session(self, session_id: str) -> Dict:
        """Delete session"""
        return self._request("DELETE", f"/sessions/{session_id}")

    # Chat Functionality - Methods for conversational interactions
    def _consume_sse_stream(self, url: str, body: Dict[str, Any]) -> Dict:
        """POST to *url* with *body*, consume the SSE stream, and return the assembled result.

        Centralised helper used by both chat() and agent_chat().
        Timeout: (10s connect, WELLREAD_CHAT_TIMEOUT read) — configurable via env var.
        
        Server-Sent Events (SSE) stream format:
          data: {"response_type": "answer", "content": "..."}
          data: {"response_type": "references", "knowledge_references": [...]}
          data: {"response_type": "complete"}
        
        We accumulate answer chunks and extract references, returning them as a dict.
        """
        try:
            # POST with stream=True to receive server-sent events incrementally
            # Timeout: 10s to establish connection, WELLREAD_CHAT_TIMEOUT for reading response
            response = self.session.post(
                url, json=body, stream=True,
                timeout=(10, CHAT_TIMEOUT),
            )
            response.raise_for_status()

            answer_chunks: list = []
            references: list = []
            debug_events: list = []

            # Use context manager to ensure the connection is returned to the pool
            # even when breaking early on a 'complete' event.
            with response:
                for raw_line in response.iter_lines():
                    if not raw_line:
                        continue
                    if isinstance(raw_line, bytes):
                        raw_line = raw_line.decode("utf-8")
                    # Each SSE event is prefixed with "data: " followed by JSON payload
                    if not raw_line.startswith("data:"):
                        continue
                    payload = raw_line[5:].lstrip(" ")
                    try:
                        event_data = json.loads(payload)
                    except json.JSONDecodeError:
                        continue

                    response_type = event_data.get("response_type", "")
                    debug_events.append({"type": response_type, "content": event_data.get("content", "")[:80]})

                    # Parse different SSE event types: answer chunks, references, errors, completion
                    if response_type == "answer":
                        chunk = event_data.get("content", "")
                        if chunk:
                            answer_chunks.append(chunk)
                    elif response_type == "references":
                        references = event_data.get("knowledge_references") or []
                    elif response_type == "error":
                        raise RequestException(
                            f"Server error: {event_data.get('content', 'unknown error')}"
                        )
                    elif response_type == "complete":
                        break

            return {
                "answer": "".join(answer_chunks),
                "references": references,
                "_debug_events": debug_events,
            }
        except RequestException as e:
            logger.error(f"SSE stream request failed ({url}): {e}")
            raise

    def chat(
        self,
        session_id: str,
        query: str,
        knowledge_base_ids: list = None,
        web_search_enabled: bool = False,
    ) -> Dict:
        """Send a message to the RAG pipeline (knowledge-chat) and return the assembled answer.

        Provide *knowledge_base_ids* (UUID or name) so the backend can retrieve
        relevant chunks before summarising with the LLM.
        For agentic tool-calling use agent_chat() instead.
        """
        url = f"{self.base_url}/knowledge-chat/{session_id}"
        body: Dict[str, Any] = {"query": query, "channel": "api"}
        if knowledge_base_ids:
            body["knowledge_base_ids"] = knowledge_base_ids
        if web_search_enabled:
            body["web_search_enabled"] = True
        result = self._consume_sse_stream(url, body)
        result["session_id"] = session_id
        return result

    def agent_chat(
        self,
        session_id: str,
        query: str,
        agent_id: str,
        knowledge_base_ids: list = None,
        web_search_enabled: bool = False,
    ) -> Dict:
        """Send a message to the agentic pipeline (agent-chat) and return the assembled answer.

        *agent_id* is required — the backend uses the CustomAgent config for
        tool selection (knowledge_search, web_search, SQL, etc.).
        The agent autonomously decides which knowledge bases to query;
        pass *knowledge_base_ids* to override or supplement the agent's default KBs.
        """
        url = f"{self.base_url}/agent-chat/{session_id}"
        body: Dict[str, Any] = {"query": query, "agent_id": agent_id, "channel": "api"}
        if knowledge_base_ids:
            body["knowledge_base_ids"] = knowledge_base_ids
        if web_search_enabled:
            body["web_search_enabled"] = True
        result = self._consume_sse_stream(url, body)
        result["session_id"] = session_id
        return result

    def list_agents(self, page: int = 1, page_size: int = 50) -> Dict:
        """List all custom agents available to the current tenant."""
        return self._request("GET", "/agents", params={"page": page, "page_size": page_size})

    def get_agent(self, agent_id: str) -> Dict:
        """Get full config of a single agent by UUID."""
        return self._request("GET", f"/agents/{agent_id}")

    # Chunk Management - Methods for managing knowledge chunks (text segments)
    def list_chunks(
        self, knowledge_id: str, page: int = 1, page_size: int = 20
    ) -> Dict:
        """List text chunks of a knowledge entry with pagination"""
        params = {"page": page, "page_size": page_size}
        return self._request("GET", f"/chunks/{knowledge_id}", params=params)

    def gaze_search(self, kb_id: str, query: str, limit: int = 10) -> Dict:
        """Search gaze pages by full-text query"""
        return self._request(
            "GET",
            f"/knowledgebase/{kb_id}/gaze/search",
            params={"q": query, "limit": limit},
        )

    def gaze_read_page(self, kb_id: str, slug: str) -> Dict:
        """Read a gaze page by slug, returns full markdown + metadata + links"""
        return self._request("GET", f"/knowledgebase/{kb_id}/gaze/pages/{slug}")

    def gaze_index_view(self, kb_id: str, limit: int = 50) -> Dict:
        """Get structured gaze index with per-type directory groups"""
        return self._request(
            "GET",
            f"/knowledgebase/{kb_id}/gaze/index",
            params={"limit": limit},
        )


