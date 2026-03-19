"""
Tests for RAG embedding service.
"""

import pytest

from services.destination_agent.src.rag import (
    EmbeddingService,
    SentenceTransformerEmbedding,
)


class TestSentenceTransformerEmbedding:
    """Tests for SentenceTransformerEmbedding."""

    @pytest.fixture
    def embedding(self):
        """Create embedding model."""
        return SentenceTransformerEmbedding(model_name="english")

    def test_model_presets(self):
        """Test that model presets are defined."""
        assert "multilingual" in SentenceTransformerEmbedding.MODEL_PRESETS
        assert "english" in SentenceTransformerEmbedding.MODEL_PRESETS

    def test_get_dimension(self, embedding):
        """Test getting embedding dimension."""
        dim = embedding.get_dimension()

        assert dim > 0
        assert dim in [384, 768]  # Common dimensions

    @pytest.mark.asyncio
    async def test_embed_single_text(self, embedding):
        """Test embedding a single text."""
        text = "Hello, world!"
        embedding_vector = await embedding.embed([text])

        assert len(embedding_vector) == 1
        assert len(embedding_vector[0]) == embedding.get_dimension()

    @pytest.mark.asyncio
    async def test_embed_multiple_texts(self, embedding):
        """Test embedding multiple texts."""
        texts = ["Hello", "World", "Test"]
        embeddings = await embedding.embed(texts)

        assert len(embeddings) == 3
        for emb in embeddings:
            assert len(emb) == embedding.get_dimension()

    @pytest.mark.asyncio
    async def test_embed_empty_list(self, embedding):
        """Test embedding empty list."""
        embeddings = await embedding.embed([])
        assert embeddings == []

    def test_get_model_name(self, embedding):
        """Test getting model name."""
        name = embedding.get_model_name()
        assert "MiniLM" in name or "all-" in name


class TestEmbeddingService:
    """Tests for EmbeddingService."""

    @pytest.fixture
    def service(self):
        """Create embedding service."""
        return EmbeddingService(model="english", cache_size=100)

    def test_service_creation(self, service):
        """Test service creation."""
        assert service._embedding_model is not None

    @pytest.mark.asyncio
    async def test_embed_text(self, service):
        """Test embedding a single text."""
        text = "Test text for embedding"
        embedding = await service.embed_text(text)

        assert len(embedding) == service.get_dimension()

    @pytest.mark.asyncio
    async def test_embed_texts(self, service):
        """Test embedding multiple texts."""
        texts = ["Text one", "Text two"]
        embeddings = await service.embed_texts(texts)

        assert len(embeddings) == 2

    @pytest.mark.asyncio
    async def test_caching(self, service):
        """Test that caching works."""
        text = "Cache test"

        # First call
        emb1 = await service.embed_text(text)

        # Second call should use cache
        emb2 = await service.embed_text(text)

        assert emb1 == emb2

    @pytest.mark.asyncio
    async def test_embed_query(self, service):
        """Test embedding a query."""
        query = "What is Kyoto famous for?"
        embedding = await service.embed_query(query)

        assert len(embedding) == service.get_dimension()

    def test_get_dimension(self, service):
        """Test getting dimension."""
        dim = service.get_dimension()
        assert dim > 0

    def test_clear_cache(self, service):
        """Test clearing cache."""
        service._cache["test_key"] = [0.1, 0.2]
        service.clear_cache()
        assert len(service._cache) == 0