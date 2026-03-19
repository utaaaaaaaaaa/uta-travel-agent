"""
"""Tests for Embedding Service.

Unit tests that don't require model loading.
Integration tests that load models are marked with @pytest.mark.integration.
"""

import hashlib
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from src.service import (
    BatchEmbedRequest,
    EmbedRequest,
    SentenceTransformerEmbedding,
    Settings,
)


class TestSentenceTransformerEmbedding:
    """Tests for the SentenceTransformerEmbedding model wrapper."""

    def test_model_presets(self):
        """Test that model presets are defined correctly."""
        assert "multilingual" in SentenceTransformerEmbedding.MODEL_PRESETS
        assert "english" in SentenceTransformerEmbedding.MODEL_PRESETS
        assert "quality" in SentenceTransformerEmbedding.MODEL_PRESETS
        assert "asian" in SentenceTransformerEmbedding.MODEL_PRESETS

    def test_preset_resolution(self):
        """Test that preset names are resolved to actual model names."""
        model = SentenceTransformerEmbedding(model_name="multilingual")
        # Should be resolved to the full model name
        assert "paraphrase" in model.model_name.lower()


class TestRequestModels:
    """Tests for request/response models."""

    def test_embed_request_validation(self):
        """Test EmbedRequest validation."""
        req = EmbedRequest(texts=["Test"])
        assert req.texts == ["Test"]
        assert req.use_cache is True

    def test_embed_request_empty_texts(self):
        """Test EmbedRequest with empty texts raises error."""
        with pytest.raises(ValueError):
            EmbedRequest(texts=[])

    def test_batch_embed_request(self):
        """Test BatchEmbedRequest."""
        req = BatchEmbedRequest(texts=["Test"], batch_size=64)
        assert req.texts == ["Test"]
        assert req.batch_size == 64


class TestEmbeddingServiceCaching:
    """Tests for the caching mechanism."""

    @pytest.fixture
    def mock_settings(self):
        """Create mock settings."""
        return Settings(
            model_name="english",
            cache_size=100,
        )

    def test_cache_key_generation(self, mock_settings):
        """Test cache key generation uses MD5."""
        text = "Test text"
        expected = hashlib.md5(text.encode()).hexdigest()
        # Create service with mock model
        with patch.object(SentenceTransformerEmbedding, "__init__", return_value=None):
            service = EmbeddingService.__new__(mock_settings)
            service._model = MagicMock()
            key = service._get_cache_key(text)
            assert key == expected

    def test_cache_eviction(self, mock_settings):
        """Test cache eviction when full."""
        with patch.object(SentenceTransformerEmbedding, "__init__", return_value=None):
            service = EmbeddingService.__new__(mock_settings)
            service._model = MagicMock()
            service._cache_size = 2

            # Add items manually
            service._cache["key1"] = [0.1, 0.2]
            service._cache["key2"] = [0.3, 0.4]

            # Adding third item should evict first
            service._add_to_cache("key3", [0.5, 0.6])
            assert "key1" not in service._cache
            assert len(service._cache) == 2

    def test_clear_cache(self, mock_settings):
        """Test clearing cache."""
        with patch.object(SentenceTransformerEmbedding, "__init__", return_value=None):
            service = EmbeddingService.__new__(mock_settings)
            service._model = MagicMock()

            service._cache["test_key"] = [0.1, 0.2, 0.3]
            count = service.clear_cache()
            assert count == 1
            assert len(service._cache) == 0


class TestEmbeddingServiceSettings:
    """Tests for Settings."""

    def test_default_settings(self):
        """Test default settings values."""
        settings = Settings()
        assert settings.host == "0.0.0.0"
        assert settings.port == 8002
        assert settings.model_name == "multilingual"
        assert settings.cache_size == 10000

    def test_custom_settings(self):
        """Test custom settings values."""
        settings = Settings(
            host="127.0.0.1",
            port=9000,
            model_name="quality",
            cache_size=5000,
        )
        assert settings.host == "127.0.0.1"
        assert settings.port == 9000
        assert settings.model_name == "quality"
        assert settings.cache_size == 5000


