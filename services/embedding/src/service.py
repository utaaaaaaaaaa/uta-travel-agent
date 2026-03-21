"""
Embedding Service for UTA Travel Agent.

Provides text embedding capabilities using Sentence Transformers.
"""

import asyncio
import hashlib
import logging
import os
from abc import ABC, abstractmethod
from typing import Optional

# Set offline mode before importing sentence_transformers
os.environ['HF_HUB_OFFLINE'] = '1'
os.environ['TRANSFORMERS_OFFLINE'] = '1'

from pydantic import BaseModel, Field
from pydantic_settings import BaseSettings
from sentence_transformers import SentenceTransformer

logger = logging.getLogger(__name__)


class Settings(BaseSettings):
    """Service settings."""

    host: str = "0.0.0.0"
    port: int = 8002

    # Embedding model settings
    model_name: str = "multilingual"  # preset or full model name or local path
    device: Optional[str] = None  # auto-detect if None
    batch_size: int = 32

    # Cache settings
    cache_size: int = 10000

    class Config:
        env_file = ".env"
        env_prefix = "EMBEDDING_"  # Allow EMBEDDING_MODEL_NAME env var


class EmbedRequest(BaseModel):
    """Request for text embedding."""

    texts: list[str] = Field(..., min_length=1)
    use_cache: bool = True


class EmbedResponse(BaseModel):
    """Response from embedding."""

    embeddings: list[list[float]]
    model: str
    dimension: int
    cached_count: int = 0


class BatchEmbedRequest(BaseModel):
    """Request for batch embedding."""

    texts: list[str] = Field(..., min_length=1)
    batch_size: int = 32


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

    Supports multilingual models for travel content.
    """

    MODEL_PRESETS = {
        "multilingual": "paraphrase-multilingual-MiniLM-L12-v2",  # 384 dims
        "english": "all-MiniLM-L6-v2",  # 384 dims, fast
        "quality": "all-mpnet-base-v2",  # 768 dims, best quality
        "asian": "distiluse-base-multilingual-cased-v1",  # 768 dims
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
        self._model: Optional[SentenceTransformer] = None

    @property
    def model(self) -> SentenceTransformer:
        """Lazy load the model."""
        if self._model is None:
            logger.info(f"Loading embedding model: {self.model_name}")
            self._model = SentenceTransformer(self.model_name, device=self.device)
        return self._model

    async def embed(self, texts: list[str]) -> list[list[float]]:
        """Create embeddings for texts using batching."""
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
    High-level embedding service with caching.
    """

    def __init__(self, settings: Settings):
        self.settings = settings
        self._model = SentenceTransformerEmbedding(
            model_name=settings.model_name,
            device=settings.device,
            batch_size=settings.batch_size,
        )
        self._cache: dict[str, list[float]] = {}
        self._cache_size = settings.cache_size

    async def embed(self, texts: list[str], use_cache: bool = True) -> tuple[list[list[float]], int]:
        """
        Create embeddings with optional caching.

        Returns (embeddings, cached_count).
        """
        if not texts:
            return [], 0

        if not use_cache:
            embeddings = await self._model.embed(texts)
            return embeddings, 0

        # Check cache and identify uncached texts
        uncached_indices = []
        uncached_texts = []
        results = [None] * len(texts)
        cached_count = 0

        for i, text in enumerate(texts):
            cache_key = self._get_cache_key(text)
            if cache_key in self._cache:
                results[i] = self._cache[cache_key]
                cached_count += 1
            else:
                uncached_indices.append(i)
                uncached_texts.append(text)

        # Embed uncached texts
        if uncached_texts:
            new_embeddings = await self._model.embed(uncached_texts)

            for idx, embedding in zip(uncached_indices, new_embeddings):
                results[idx] = embedding
                cache_key = self._get_cache_key(texts[idx])
                self._add_to_cache(cache_key, embedding)

        return results, cached_count

    async def embed_single(self, text: str, use_cache: bool = True) -> list[float]:
        """Embed a single text."""
        embeddings, _ = await self.embed([text], use_cache)
        return embeddings[0]

    def get_dimension(self) -> int:
        """Get embedding dimension."""
        return self._model.get_dimension()

    def get_model_name(self) -> str:
        """Get model name."""
        return self._model.get_model_name()

    def _get_cache_key(self, text: str) -> str:
        """Generate cache key for text."""
        return hashlib.md5(text.encode()).hexdigest()

    def _add_to_cache(self, key: str, embedding: list[float]) -> None:
        """Add embedding to cache with LRU eviction."""
        if len(self._cache) >= self._cache_size:
            # Remove oldest entry (simple FIFO)
            oldest_key = next(iter(self._cache))
            del self._cache[oldest_key]

        self._cache[key] = embedding

    def clear_cache(self) -> int:
        """Clear the embedding cache. Returns number of entries cleared."""
        count = len(self._cache)
        self._cache.clear()
        return count

    async def health_check(self) -> dict:
        """Check service health."""
        return {
            "status": "healthy",
            "model": self.get_model_name(),
            "dimension": self.get_dimension(),
            "cache_size": len(self._cache),
        }


# Singleton instance
_service: Optional[EmbeddingService] = None


def get_service() -> EmbeddingService:
    """Get the embedding service singleton."""
    global _service
    if _service is None:
        _service = EmbeddingService(Settings())
    return _service
