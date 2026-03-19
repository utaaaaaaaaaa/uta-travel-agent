"""
Tests for RAG document processing module.
"""

import pytest

from rag import (
    Chunk,
    ChunkingConfig,
    Document,
    DocumentProcessor,
)


class TestDocumentProcessor:
    """Tests for DocumentProcessor."""

    @pytest.fixture
    def processor(self):
        """Create a document processor with default config."""
        return DocumentProcessor()

    @pytest.fixture
    def small_processor(self):
        """Create a processor with small chunk size."""
        return DocumentProcessor(
            config=ChunkingConfig(
                chunk_size=100,
                chunk_overlap=20,
                min_chunk_size=20,
                max_chunk_size=200,
            )
        )

    def test_process_simple_document(self, processor):
        """Test processing a simple document."""
        doc = Document(
            id="test-1",
            title="Test Document",
            content="This is a test document. " * 50,
            source="https://example.com",
        )

        chunks = processor.process_document(doc)

        assert len(chunks) > 0
        assert all(c.document_id == doc.id for c in chunks)
        assert all(c.content for c in chunks)

    def test_process_short_document(self, processor):
        """Test processing a short document."""
        doc = Document(
            id="test-2",
            title="Short Doc",
            content="This is a short document.",
        )

        chunks = processor.process_document(doc)

        # Short doc should be single chunk
        assert len(chunks) == 1
        assert chunks[0].content == doc.content

    def test_chunk_metadata_preserved(self, processor):
        """Test that document metadata is preserved in chunks."""
        doc = Document(
            id="test-3",
            title="Metadata Test",
            content="Content " * 100,
            source="https://example.com",
            metadata={"destination": "Kyoto", "theme": "cultural"},
        )

        chunks = processor.process_document(doc)

        for chunk in chunks:
            assert chunk.metadata.get("document_title") == doc.title
            assert chunk.metadata.get("document_source") == doc.source
            assert chunk.metadata.get("destination") == "Kyoto"

    def test_text_cleaning(self, processor):
        """Test text cleaning functionality."""
        dirty_text = """
        This  has   extra   whitespace.


        And multiple newlines.
        \u2018smart quotes\u2019 and \u201cdouble quotes\u201d
        \u2014em dashes\u2014
        """

        cleaned = processor._clean_text(dirty_text)

        assert "  " not in cleaned  # No double spaces
        assert "\n\n\n" not in cleaned  # No triple newlines
        assert "'" in cleaned  # Normal single quotes
        assert '"' in cleaned  # Normal double quotes (smart quotes converted to these)
        assert "\u201c" not in cleaned  # No left smart quotes
        assert "\u201d" not in cleaned  # No right smart quotes
        assert "\u2014" not in cleaned  # No em dashes (converted to regular dash)

    def test_section_splitting(self, processor):
        """Test document splitting into sections."""
        text = """Introduction

This is the introduction paragraph.

History

This is about history.

Culture

This is about culture."""

        sections = processor._split_into_sections(text)

        assert len(sections) >= 1

    def test_token_counting(self, processor):
        """Test token counting."""
        text = "Hello, world!"
        count = processor.count_tokens(text)

        assert count > 0
        assert count < len(text)  # Tokens < chars usually

    def test_chunk_ids_unique(self, processor):
        """Test that chunk IDs are unique."""
        doc = Document(
            id="test-unique",
            title="Unique Test",
            content="Content " * 200,
        )

        chunks = processor.process_document(doc)
        chunk_ids = [c.id for c in chunks]

        assert len(chunk_ids) == len(set(chunk_ids))  # All unique

    def test_empty_document(self, processor):
        """Test processing an empty document."""
        doc = Document(
            id="empty",
            title="Empty",
            content="",
        )

        chunks = processor.process_document(doc)
        assert len(chunks) == 0

    def test_process_multiple_documents(self, processor):
        """Test processing multiple documents."""
        docs = [
            Document(
                id=f"doc-{i}",
                title=f"Document {i}",
                content=f"Content for document {i}. " * 50,
            )
            for i in range(3)
        ]

        all_chunks = processor.process_documents(docs)

        assert len(all_chunks) > 0
        doc_ids = {c.document_id for c in all_chunks}
        assert len(doc_ids) == 3


class TestChunkingConfig:
    """Tests for ChunkingConfig."""

    def test_default_config(self):
        """Test default configuration values."""
        config = ChunkingConfig()

        assert config.chunk_size == 512
        assert config.chunk_overlap == 50
        assert config.min_chunk_size == 100
        assert config.max_chunk_size == 1024

    def test_custom_config(self):
        """Test custom configuration."""
        config = ChunkingConfig(
            chunk_size=256,
            chunk_overlap=30,
        )

        assert config.chunk_size == 256
        assert config.chunk_overlap == 30


class TestDocument:
    """Tests for Document dataclass."""

    def test_document_creation(self):
        """Test creating a document."""
        doc = Document(
            id="test",
            title="Test",
            content="Content",
        )

        assert doc.id == "test"
        assert doc.metadata == {}

    def test_document_with_metadata(self):
        """Test document with metadata."""
        doc = Document(
            id="test",
            title="Test",
            content="Content",
            metadata={"key": "value"},
        )

        assert doc.metadata["key"] == "value"


class TestChunk:
    """Tests for Chunk dataclass."""

    def test_chunk_creation(self):
        """Test creating a chunk."""
        chunk = Chunk(
            id="chunk-1",
            document_id="doc-1",
            content="Chunk content",
            token_count=10,
            chunk_index=0,
        )

        assert chunk.id == "chunk-1"
        assert chunk.embedding is None

    def test_chunk_with_embedding(self):
        """Test chunk with embedding."""
        embedding = [0.1] * 384  # Example embedding
        chunk = Chunk(
            id="chunk-2",
            document_id="doc-1",
            content="Content",
            token_count=5,
            chunk_index=1,
            embedding=embedding,
        )

        assert len(chunk.embedding) == 384
