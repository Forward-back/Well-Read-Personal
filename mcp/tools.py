"""Read-only tool registrations for the WellRead personal MCP surface."""

import asyncio
import functools
import logging
from typing import Dict

from client import GazeClient, normalize_kb_entries

logger = logging.getLogger(__name__)


def register_tools(mcp, client: GazeClient) -> None:
    """Register the 17 tools backed by the personal read-only API key."""

    @mcp.tool()
    def list_knowledge_bases() -> dict:
        """List all knowledge bases."""
        return client.list_knowledge_bases()

    @mcp.tool()
    def get_knowledge_base(kb_id: str) -> dict:
        """Get knowledge base details."""
        return client.get_knowledge_base(kb_id)

    @mcp.tool()
    def hybrid_search(
        kb_id: str,
        query: str,
        vector_threshold: float = 0.5,
        keyword_threshold: float = 0.3,
        match_count: int = 5,
    ) -> dict:
        """Perform hybrid (vector + keyword) search in a knowledge base.

        kb_id may be a UUID or a knowledge-base name (resolved automatically).
        Use list_knowledge_bases to discover available knowledge bases.
        """
        config = {
            "vector_threshold": vector_threshold,
            "keyword_threshold": keyword_threshold,
            "match_count": match_count,
        }
        resolved = client.resolve_kb_id(kb_id)
        return client.hybrid_search(resolved, query, config)

    @mcp.tool()
    def list_knowledge(kb_id: str, page: int = 1, page_size: int = 20) -> dict:
        """List knowledge entries in a knowledge base."""
        return client.list_knowledge(kb_id, page, page_size)

    @mcp.tool()
    def get_knowledge(knowledge_id: str) -> dict:
        """Get knowledge details."""
        return client.get_knowledge(knowledge_id)

    @mcp.tool()
    def list_chunks(knowledge_id: str, page: int = 1, page_size: int = 20) -> dict:
        """List chunks (text segments) of a knowledge entry."""
        return client.list_chunks(knowledge_id, page, page_size)

    @mcp.tool()
    def create_session(
        kb_id: str,
        max_rounds: int = 5,
        enable_rewrite: bool = True,
        fallback_response: str = "Sorry, I cannot answer this question.",
        summary_model_id: str = "",
        title: str = "",
        description: str = "",
    ) -> dict:
        """Create a new chat session bound to a knowledge base.

        kb_id may be a UUID or a knowledge-base name (resolved automatically).
        """
        return client.create_session(
            kb_id=client.resolve_kb_id(kb_id),
            max_rounds=max_rounds,
            enable_rewrite=enable_rewrite,
            fallback_response=fallback_response,
            summary_model_id=summary_model_id,
            title=title,
            description=description,
        )

    @mcp.tool()
    def get_session(session_id: str) -> dict:
        """Get session details."""
        return client.get_session(session_id)

    @mcp.tool()
    def list_sessions(page: int = 1, page_size: int = 20) -> dict:
        """List chat sessions."""
        return client.list_sessions(page, page_size)

    @mcp.tool()
    def delete_session(session_id: str) -> dict:
        """Delete a session created through MCP."""
        return client.delete_session(session_id)

    @mcp.tool()
    async def chat(
        session_id: str,
        query: str,
        knowledge_base_ids: list[str] | None = None,
        web_search_enabled: bool = False,
    ) -> dict:
        """RAG pipeline chat: retrieve relevant chunks, then summarise with an LLM.

        ALWAYS provide knowledge_base_ids (names or UUIDs) so retrieval can run;
        without them the answer is based on LLM knowledge only. Use
        list_knowledge_bases to discover available knowledge bases. For
        multi-step reasoning or tool-calling use agent_chat instead.
        """
        kb_ids = (
            [client.resolve_kb_id(k) for k in knowledge_base_ids]
            if knowledge_base_ids
            else None
        )
        fn = functools.partial(
            client.chat,
            session_id,
            query,
            knowledge_base_ids=kb_ids,
            web_search_enabled=web_search_enabled,
        )
        return await asyncio.get_running_loop().run_in_executor(None, fn)

    @mcp.tool()
    async def agent_chat(
        session_id: str,
        query: str,
        agent_id: str,
        knowledge_base_ids: list[str] | None = None,
        web_search_enabled: bool = False,
    ) -> dict:
        """Agentic pipeline chat: the agent autonomously calls its own tools.

        REQUIRED: agent_id (name or UUID) — use list_agents to discover agents.
        IMPORTANT: many agents have KBSelectionMode=none and NO built-in
        knowledge bases. In that case you MUST pass knowledge_base_ids,
        otherwise the agent fails with 'no search targets available'. Use
        get_agent to inspect kb_selection_mode and knowledge_bases first.
        """
        resolved_agent_id = client.resolve_agent_id(agent_id)
        kb_ids = (
            [client.resolve_kb_id(k) for k in knowledge_base_ids]
            if knowledge_base_ids
            else None
        )
        if not kb_ids:
            try:
                agent_info = client.get_agent(resolved_agent_id)
                cfg = (agent_info.get("data") or agent_info).get("config") or {}
                mode = cfg.get("kb_selection_mode", "selected")
                built_in_kbs = cfg.get("knowledge_bases") or []
                needs_kbs = (mode == "none") or (
                    mode in ("selected", "") and not built_in_kbs
                )
                if needs_kbs:
                    all_kbs = normalize_kb_entries(client.list_knowledge_bases())
                    seen_ids: set[str] = set()
                    unique_kbs: list[Dict] = []
                    for kb in all_kbs:
                        kb_id = kb.get("id")
                        if kb_id and kb_id not in seen_ids:
                            seen_ids.add(kb_id)
                            unique_kbs.append(kb)
                    kb_summary = ", ".join(
                        f"{kb.get('name')} ({kb.get('id')})" for kb in unique_kbs[:10]
                    )
                    raise ValueError(
                        f"Agent '{agent_id}' has kb_selection_mode='{mode}' with no built-in "
                        f"knowledge bases. You must provide knowledge_base_ids. "
                        f"Available knowledge bases: [{kb_summary}]"
                    )
            except ValueError:
                raise
            except Exception as preflight_err:
                logger.warning(
                    "agent_chat preflight KB check failed (non-fatal): %s",
                    preflight_err,
                )
        fn = functools.partial(
            client.agent_chat,
            session_id,
            query,
            resolved_agent_id,
            knowledge_base_ids=kb_ids,
            web_search_enabled=web_search_enabled,
        )
        return await asyncio.get_running_loop().run_in_executor(None, fn)

    @mcp.tool()
    def list_agents(page: int = 1, page_size: int = 50) -> dict:
        """List agents available to the current key."""
        return client.list_agents(page=page, page_size=page_size)

    @mcp.tool()
    def get_agent(agent_id: str) -> dict:
        """Get an agent's configuration by UUID or name."""
        return client.get_agent(client.resolve_agent_id(agent_id))

    @mcp.tool()
    def gaze_search(kb_id: str, query: str, limit: int = 10) -> dict:
        """Search gaze pages by full-text query."""
        return client.gaze_search(kb_id, query, limit)

    @mcp.tool()
    def gaze_read_page(kb_id: str, slug: str) -> dict:
        """Read a gaze page by slug, returning markdown, metadata and links."""
        return client.gaze_read_page(kb_id, slug)

    @mcp.tool()
    def gaze_index_view(kb_id: str, limit: int = 50) -> dict:
        """Get the structured gaze index grouped by page type."""
        return client.gaze_index_view(kb_id, limit)
