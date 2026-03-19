"""
RAG Query Service for destination knowledge retrieval.

Combines vector search with LLM for intelligent answers.
"""

import logging
from dataclasses import dataclass
from typing import Optional

from anthropic import AsyncAnthropic

from .embeddings import EmbeddingService
from .vector_store import QdrantVectorStore

logger = logging.getLogger(__name__)


@dataclass
class RAGResponse:
    """Response from a RAG query."""

    answer: str
    sources: list[dict]
    confidence: float
    query: str


@dataclass
class SearchContext:
    """Context for RAG search."""

    collection_name: str
    query: str
    top_k: int = 5
    score_threshold: float = 0.5
    filter_conditions: Optional[dict] = None


class RAGService:
    """
    Complete RAG pipeline for destination queries.

    Combines:
    1. Vector search for relevant documents
    2. Context building
    3. LLM generation with Claude
    """

    # System prompt for travel guide persona
    SYSTEM_PROMPT = """你是一位专业的旅行导游助手。你的任务是根据提供的目的地知识库信息，为用户提供准确、有用的旅行建议。

规则:
1. 只使用提供的上下文信息回答问题
2. 如果信息不足，坦诚告知用户
3. 保持回答简洁但有价值
4. 适当添加文化背景和有趣的事实
5. 使用友好的语气，像一位本地导游一样交流"""

    def __init__(
        self,
        vector_store: QdrantVectorStore,
        embedding_service: EmbeddingService,
        anthropic_client: Optional[AsyncAnthropic] = None,
        model: str = "claude-sonnet-4-20250514",
        max_context_tokens: int = 4000,
    ):
        self.vector_store = vector_store
        self.embedding_service = embedding_service
        self.anthropic_client = anthropic_client
        self.model = model
        self.max_context_tokens = max_context_tokens

    def set_anthropic_client(self, client: AsyncAnthropic) -> None:
        """Set the Anthropic client."""
        self.anthropic_client = client

    async def query(
        self,
        context: SearchContext,
    ) -> RAGResponse:
        """
        Execute a RAG query.

        1. Search for relevant documents
        2. Build context from results
        3. Generate answer with Claude
        """
        # Step 1: Search
        search_results = await self.vector_store.search(
            collection_name=context.collection_name,
            query=context.query,
            limit=context.top_k,
            score_threshold=context.score_threshold,
            filter_conditions=context.filter_conditions,
        )

        if not search_results:
            return RAGResponse(
                answer="抱歉，我没有找到相关的信息。请尝试换一个方式提问。",
                sources=[],
                confidence=0.0,
                query=context.query,
            )

        # Step 2: Build context
        context_text = self._build_context(search_results)

        # Step 3: Generate answer
        answer = await self._generate_answer(context.query, context_text)

        # Calculate confidence based on search scores
        avg_score = sum(r["score"] for r in search_results) / len(search_results)

        return RAGResponse(
            answer=answer,
            sources=[
                {
                    "content": r["content"][:200] + "...",
                    "score": r["score"],
                    "document_id": r.get("document_id"),
                }
                for r in search_results
            ],
            confidence=min(avg_score * 1.2, 1.0),  # Scale confidence
            query=context.query,
        )

    async def search_only(
        self,
        context: SearchContext,
    ) -> list[dict]:
        """
        Only search without LLM generation.

        Useful for getting raw search results.
        """
        return await self.vector_store.search(
            collection_name=context.collection_name,
            query=context.query,
            limit=context.top_k,
            score_threshold=context.score_threshold,
            filter_conditions=context.filter_conditions,
        )

    def _build_context(self, search_results: list[dict]) -> str:
        """Build context string from search results."""
        context_parts = []
        current_tokens = 0

        for i, result in enumerate(search_results):
            content = result["content"]
            # Estimate tokens (rough: ~4 chars per token for Chinese)
            estimated_tokens = len(content) // 3

            if current_tokens + estimated_tokens > self.max_context_tokens:
                # Truncate to fit
                remaining_tokens = self.max_context_tokens - current_tokens
                if remaining_tokens > 100:  # Only add if meaningful
                    truncated_content = content[: remaining_tokens * 3]
                    context_parts.append(f"[文档{i+1}]\n{truncated_content}...")
                break

            context_parts.append(f"[文档{i+1}]\n{content}")
            current_tokens += estimated_tokens

        return "\n\n".join(context_parts)

    async def _generate_answer(self, query: str, context: str) -> str:
        """Generate answer using Claude."""
        if self.anthropic_client is None:
            logger.warning("No Anthropic client configured, returning context-only response")
            return f"根据知识库信息:\n\n{context[:500]}..."

        try:
            response = await self.anthropic_client.messages.create(
                model=self.model,
                max_tokens=1024,
                system=self.SYSTEM_PROMPT,
                messages=[
                    {
                        "role": "user",
                        "content": f"""用户问题: {query}

参考信息:
{context}

请根据以上信息回答用户的问题。如果参考信息中没有相关内容，请坦诚告知。""",
                    }
                ],
            )

            return response.content[0].text

        except Exception as e:
            logger.error(f"Failed to generate answer: {e}")
            return f"抱歉，生成回答时出现错误。请稍后再试。"

    async def stream_answer(
        self,
        context: SearchContext,
    ):
        """
        Stream answer generation.

        Yields answer chunks for real-time display.
        """
        # Search first
        search_results = await self.vector_store.search(
            collection_name=context.collection_name,
            query=context.query,
            limit=context.top_k,
            score_threshold=context.score_threshold,
        )

        if not search_results:
            yield "抱歉，我没有找到相关的信息。"
            return

        context_text = self._build_context(search_results)

        if self.anthropic_client is None:
            yield context_text
            return

        try:
            async with self.anthropic_client.messages.stream(
                model=self.model,
                max_tokens=1024,
                system=self.SYSTEM_PROMPT,
                messages=[
                    {
                        "role": "user",
                        "content": f"""用户问题: {context.query}

参考信息:
{context_text}

请根据以上信息回答用户的问题。""",
                    }
                ],
            ) as stream:
                async for text in stream.text_stream:
                    yield text

        except Exception as e:
            logger.error(f"Failed to stream answer: {e}")
            yield f"生成回答时出错: {str(e)}"


class DestinationKnowledgeBase:
    """
    High-level interface for destination knowledge.

    Combines all RAG components for easy use.
    """

    def __init__(
        self,
        rag_service: RAGService,
        vector_store: QdrantVectorStore,
    ):
        self.rag = rag_service
        self.vector_store = vector_store

    async def ask(
        self,
        collection_name: str,
        question: str,
        top_k: int = 5,
    ) -> RAGResponse:
        """
        Ask a question about a destination.

        Args:
            collection_name: The destination's collection
            question: User's question
            top_k: Number of documents to retrieve

        Returns:
            RAGResponse with answer and sources
        """
        context = SearchContext(
            collection_name=collection_name,
            query=question,
            top_k=top_k,
        )
        return await self.rag.query(context)

    async def search_landmarks(
        self,
        collection_name: str,
        query: str,
        limit: int = 10,
    ) -> list[dict]:
        """
        Search for landmarks and attractions.

        Returns raw search results without LLM processing.
        """
        context = SearchContext(
            collection_name=collection_name,
            query=query,
            top_k=limit,
        )
        return await self.rag.search_only(context)

    async def get_nearby_points(
        self,
        collection_name: str,
        latitude: float,
        longitude: float,
        radius_km: float = 1.0,
        limit: int = 10,
    ) -> list[dict]:
        """
        Get points of interest near a location.

        Note: Requires location metadata in indexed documents.
        """
        # TODO: Implement geo-filtering when Qdrant supports it
        # For now, do post-filtering
        results = await self.vector_store.search(
            collection_name=collection_name,
            query=f"location near {latitude},{longitude}",
            limit=limit * 2,  # Get more to filter
        )

        # Filter by distance if location data available
        # This is a placeholder - real implementation would use proper geo-filtering
        return results[:limit]


__all__ = [
    "RAGResponse",
    "SearchContext",
    "RAGService",
    "DestinationKnowledgeBase",
]