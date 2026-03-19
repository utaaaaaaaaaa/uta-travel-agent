"""
Tests for RAG service and query functionality.
"""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from services.destination_agent.src.rag import (
    QdrantVectorStore,
    RAGResponse,
    RAGService,
    SearchContext,
)


class TestQdrantVectorStore:
    """Tests for QdrantVectorStore."""

    @pytest.fixture
    def mock_qdrant_client(self):
        """Create a mock Qdrant client."""
        client = MagicMock()
        return client

    @pytest.fixture
    def vector_store(self, mock_qdrant_client):
        """Create vector store with mocked client."""
        store = QdrantVectorStore(host="localhost", port=6333)
        store._client = mock_qdrant_client
        return store

    @pytest.mark.asyncio
    async def test_create_collection(self, vector_store, mock_qdrant_client):
        """Test creating a collection."""
        mock_qdrant_client.create_collection.return_value = None

        result = await vector_store.create_collection(
            collection_name="test_collection",
            vector_size=384,
        )

        assert result is True
        mock_qdrant_client.create_collection.assert_called_once()

    @pytest.mark.asyncio
    async def test_collection_exists(self, vector_store, mock_qdrant_client):
        """Test checking if collection exists."""
        mock_qdrant_client.get_collection.return_value = MagicMock()

        exists = await vector_store.collection_exists("test")

        assert exists is True

    @pytest.mark.asyncio
    async def test_collection_not_exists(self, vector_store, mock_qdrant_client):
        """Test checking non-existent collection."""
        mock_qdrant_client.get_collection.side_effect = Exception("Not found")

        exists = await vector_store.collection_exists("nonexistent")

        assert exists is False

    @pytest.mark.asyncio
    async def test_delete_collection(self, vector_store, mock_qdrant_client):
        """Test deleting a collection."""
        mock_qdrant_client.delete_collection.return_value = None

        result = await vector_store.delete_collection("test")

        assert result is True


class TestRAGService:
    """Tests for RAGService."""

    @pytest.fixture
    def mock_vector_store(self):
        """Create mock vector store."""
        store = MagicMock(spec=QdrantVectorStore)
        store.search = AsyncMock(return_value=[
            {
                "id": "chunk-1",
                "score": 0.9,
                "content": "Kyoto is famous for its temples.",
                "document_id": "doc-1",
            }
        ])
        return store

    @pytest.fixture
    def mock_embedding(self):
        """Create mock embedding service."""
        from services.destination_agent.src.rag import EmbeddingService
        service = MagicMock(spec=EmbeddingService)
        service.embed_query = AsyncMock(return_value=[0.1] * 384)
        service.get_dimension.return_value = 384
        return service

    @pytest.fixture
    def rag_service(self, mock_vector_store, mock_embedding):
        """Create RAG service with mocks."""
        return RAGService(
            vector_store=mock_vector_store,
            embedding_service=mock_embedding,
            anthropic_client=None,  # No LLM for unit tests
        )

    @pytest.mark.asyncio
    async def test_search_only(self, rag_service, mock_vector_store):
        """Test search without LLM generation."""
        context = SearchContext(
            collection_name="test",
            query="What is Kyoto famous for?",
            top_k=5,
        )

        results = await rag_service.search_only(context)

        assert len(results) > 0
        mock_vector_store.search.assert_called_once()

    @pytest.mark.asyncio
    async def test_query_with_no_results(self, rag_service, mock_vector_store):
        """Test query with no search results."""
        mock_vector_store.search.return_value = []

        context = SearchContext(
            collection_name="test",
            query="Unknown query",
        )

        response = await rag_service.query(context)

        assert "没有找到" in response.answer or response.confidence == 0.0

    @pytest.mark.asyncio
    async def test_build_context(self, rag_service):
        """Test context building from search results."""
        results = [
            {"content": "Content 1", "score": 0.9},
            {"content": "Content 2", "score": 0.8},
        ]

        context = rag_service._build_context(results)

        assert "Content 1" in context
        assert "Content 2" in context
        assert "[文档" in context


class TestSearchContext:
    """Tests for SearchContext."""

    def test_context_creation(self):
        """Test creating search context."""
        context = SearchContext(
            collection_name="kyoto",
            query="Best temples in Kyoto",
            top_k=10,
            score_threshold=0.7,
        )

        assert context.collection_name == "kyoto"
        assert context.top_k == 10
        assert context.score_threshold == 0.7

    def test_context_defaults(self):
        """Test default values."""
        context = SearchContext(
            collection_name="test",
            query="query",
        )

        assert context.top_k == 5
        assert context.score_threshold == 0.5


class TestRAGResponse:
    """Tests for RAGResponse."""

    def test_response_creation(self):
        """Test creating a response."""
        response = RAGResponse(
            answer="Kyoto is famous for temples.",
            sources=[{"content": "...", "score": 0.9}],
            confidence=0.85,
            query="What is Kyoto famous for?",
        )

        assert response.answer == "Kyoto is famous for temples."
        assert len(response.sources) == 1
        assert response.confidence == 0.85
