"""
Embedding service for creating vector embeddings from text.

Supports multiple embedding models with a focus on multilingual support.
"""

import asyncio
import logging
from abc import ABC, abstractmethod
from typing import Optional

import numpy as np
from sentence_transformers import SentenceTransformer

logger = logging.getLogger(__name__)


class EmbeddingModel(ABC):
    """Abstract base class for embedding models."""

    @abstractmethod
    async def embed(self, texts: list[str]) -> list[list[float]]:
        """Create embeddings for a list of texts."""
        pass

    @abstractmethod
    def get_dimension(self) -> int:
        """Get the dimension of embeddings."""
        pass

    @abstractmethod
    def get_model_name(self) -> str:
        """Get the model name."""
        pass


class SentenceTransformerEmbedding(EmbeddingModel):
    """
    Embedding using sentence-transformers models.

    Uses multilingual models for better travel content support.
    """

    # Model presets for different use cases
    MODEL_PRESETS = {
        "multilingual": "paraphrase-multilingual-MiniLM-L12-v2",  # 384 dims, 50+ languages
        "english": "all-MiniLM-L6-v2",  # 384 dims, fast
        "quality": "all-mpnet-base-v2",  # 768 dims, best quality
        "asian": "distiluse-base-multilingual-cased-v1",  # 768 dims, good for Asian languages
    }

    def __init__(
        self,
        model_name: str = "multilingual",
        device: Optional[str] = None,
        batch_size: int = 32,
    ):
        # Resolve preset or use direct model name
        if model_name in self.MODEL_PRESETS:
            model_name = self.MODEL_PRESETS[model_name]

        self.model_name = model_name
        self.batch_size = batch_size
        self.device = device

        # Load model (lazy loading in practice)
        self._model: Optional[SentenceTransformer] = None

    @property
    def model(self) -> SentenceTransformer:
        """Lazy load the model."""
        if self._model is None:
            logger.info(f"Loading embedding model: {self.model_name}")
            self._model = SentenceTransformer(self.model_name, device=self.device)
        return self._model

    async def embed(self, texts: list[str]) -> list[list[float]]:
        """
        Create embeddings for texts.

        Uses batching for efficiency.
        """
        if not texts:
            return []

        # Run in thread pool to avoid blocking
        loop = asyncio.get_event_loop()
        embeddings = await loop.run_in_executor(
            None,
            self._embed_batch,
            texts,
        )

        return embeddings

    def _embed_batch(self, texts: list[str]) -> list[list[float]]:
        """Synchronous embedding with batching."""
        embeddings = self.model.encode(
            texts,
            batch_size=self.batch_size,
            show_progress_bar=False,
            convert_to_numpy=True,
        )
        return embeddings.tolist()

    def get_dimension(self) -> int:
        """Get embedding dimension."""
        return self.model.get_sentence_embedding_dimension()

    def get_model_name(self) -> str:
        """Get model name."""
        return self.model_name


class EmbeddingService:
    """
    High-level embedding service.

    Manages embedding models and provides caching.
    """

    def __init__(
        self,
        model: str = "multilingual",
        device: Optional[str] = None,
        cache_size: int = 10000,
    ):
        self._embedding_model = SentenceTransformerEmbedding(
            model_name=model,
            device=device,
        )
        self._cache: dict[str, list[float]] = {}
        self._cache_size = cache_size

    async def embed_text(self, text: str) -> list[float]:
        """Create embedding for a single text."""
        # Check cache
        cache_key = self._get_cache_key(text)
        if cache_key in self._cache:
            return self._cache[cache_key]

        # Create embedding
        embeddings = await self._embedding_model.embed([text])
        embedding = embeddings[0]

        # Cache result
        self._add_to_cache(cache_key, embedding)

        return embedding

    async def embed_texts(self, texts: list[str]) -> list[list[float]]:
        """Create embeddings for multiple texts."""
        if not texts:
            return []

        # Check cache and identify uncached texts
        uncached_indices = []
        uncached_texts = []
        results = [None] * len(texts)

        for i, text in enumerate(texts):
            cache_key = self._get_cache_key(text)
            if cache_key in self._cache:
                results[i] = self._cache[cache_key]
            else:
                uncached_indices.append(i)
                uncached_texts.append(text)

        # Embed uncached texts
        if uncached_texts:
            new_embeddings = await self._embedding_model.embed(uncached_texts)

            for idx, embedding in zip(uncached_indices, new_embeddings):
                results[idx] = embedding
                cache_key = self._get_cache_key(texts[idx])
                self._add_to_cache(cache_key, embedding)

        return results

    async def embed_query(self, query: str) -> list[float]:
        """
        Embed a search query.

        May use different preprocessing than document embedding.
        """
        # For now, same as text embedding
        return await self.embed_text(query)

    def get_dimension(self) -> int:
        """Get embedding dimension."""
        return self._embedding_model.get_dimension()

    def get_model_name(self) -> str:
        """Get model name."""
        return self._embedding_model.get_model_name()

    def _get_cache_key(self, text: str) -> str:
        """Generate cache key for text."""
        import hashlib
        return hashlib.md5(text.encode()).hexdigest()

    def _add_to_cache(self, key: str, embedding: list[float]) -> None:
        """Add embedding to cache with LRU eviction."""
        if len(self._cache) >= self._cache_size:
            # Remove oldest entry (simple FIFO, could be improved to LRU)
            oldest_key = next(iter(self._cache))
            del self._cache[oldest_key]

        self._cache[key] = embedding

    def clear_cache(self) -> None:
        """Clear the embedding cache."""
        self._cache.clear()


__all__ = [
    "EmbeddingModel",
    "SentenceTransformerEmbedding",
    "EmbeddingService",
]