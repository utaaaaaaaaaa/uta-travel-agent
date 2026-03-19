#!/usr/bin/env python3
"""
Minimal RAG test without external dependencies.
Tests core logic only.
"""

import hashlib
import re
import sys
from dataclasses import dataclass, field
from typing import Optional


@dataclass
class Document:
    id: str
    title: str
    content: str
    source: Optional[str] = None
    metadata: dict = field(default_factory=dict)


@dataclass
class Chunk:
    id: str
    document_id: str
    content: str
    token_count: int
    chunk_index: int
    metadata: dict = field(default_factory=dict)


@dataclass
class ChunkingConfig:
    chunk_size: int = 512
    chunk_overlap: int = 50
    min_chunk_size: int = 100
    max_chunk_size: int = 1024


class SimpleDocumentProcessor:
    """Simplified document processor for testing."""

    def __init__(self, config: Optional[ChunkingConfig] = None):
        self.config = config or ChunkingConfig()

    def process_document(self, document: Document) -> list[Chunk]:
        """Process a document into chunks."""
        content = self._clean_text(document.content)
        sections = self._split_into_sections(content)

        chunks = []
        chunk_index = 0

        for section in sections:
            section_chunks = self._chunk_section(section, document, chunk_index)
            chunks.extend(section_chunks)
            chunk_index += len(section_chunks)

        return chunks

    def _clean_text(self, text: str) -> str:
        """Clean and normalize text."""
        text = re.sub(r"\s+", " ", text)
        text = re.sub(r"\n{3,}", "\n\n", text)
        return text.strip()

    def _split_into_sections(self, text: str) -> list[str]:
        """Split text into sections."""
        paragraphs = text.split("\n\n")
        sections = []
        current = []

        for para in paragraphs:
            para = para.strip()
            if not para:
                continue
            current.append(para)

        if current:
            sections.append("\n\n".join(current))

        return sections

    def _chunk_section(self, section: str, document: Document, start_index: int) -> list[Chunk]:
        """Chunk a section into pieces."""
        # Simple word-based estimation (~1.3 words per token for English)
        words = section.split()
        estimated_tokens = len(words) // 1.3

        if estimated_tokens <= self.config.max_chunk_size:
            return [self._create_chunk(section, document, start_index)]

        # Split into multiple chunks
        chunks = []
        chunk_size_words = int(self.config.chunk_size * 1.3)
        overlap_words = int(self.config.chunk_overlap * 1.3)

        start = 0
        chunk_index = start_index

        while start < len(words):
            end = min(start + chunk_size_words, len(words))
            chunk_words = words[start:end]
            chunk_text = " ".join(chunk_words)

            if len(chunk_words) >= self.config.min_chunk_size // 2:
                chunks.append(self._create_chunk(chunk_text, document, chunk_index))
                chunk_index += 1

            start = end - overlap_words
            if start <= 0:
                break

        return chunks

    def _create_chunk(self, content: str, document: Document, chunk_index: int) -> Chunk:
        """Create a chunk."""
        token_count = len(content.split()) // 1.3
        chunk_id = hashlib.md5(f"{document.id}_{chunk_index}_{content[:50]}".encode()).hexdigest()[:16]

        return Chunk(
            id=chunk_id,
            document_id=document.id,
            content=content,
            token_count=int(token_count),
            chunk_index=chunk_index,
            metadata={
                "document_title": document.title,
                "document_source": document.source,
                **document.metadata,
            },
        )


def test_document_processor():
    """Test document processor."""
    print("=" * 60)
    print("Testing Document Processor")
    print("=" * 60)

    processor = SimpleDocumentProcessor()

    doc = Document(
        id="test-kyoto",
        title="Kyoto Travel Guide",
        content="""
        Kyoto is a city in Japan known for its classical Buddhist temples,
        gardens, imperial palaces, Shinto shrines and traditional wooden houses.

        The city is home to famous temples like Kiyomizu-dera, a wooden temple
        supported by pillars on a mountainside, and Kinkaku-ji (Golden Pavilion),
        a gold-leaf covered temple surrounded by gardens.

        The Fushimi Inari Shrine is famous for its thousands of vermilion torii gates.
        It is one of the most iconic images of Japan.

        Gion is Kyoto's most famous geisha district. It is filled with shops,
        restaurants and ochaya (teahouses), where geiko entertain guests.
        """,
        source="https://example.com/kyoto",
        metadata={"destination": "Kyoto", "theme": "cultural"},
    )

    chunks = processor.process_document(doc)

    print(f"\nDocument: {doc.title}")
    print(f"Content length: {len(doc.content)} characters")
    print(f"Chunks created: {len(chunks)}")
    print()

    for i, chunk in enumerate(chunks):
        print(f"Chunk {i}:")
        print(f"  ID: {chunk.id}")
        print(f"  Tokens: ~{chunk.token_count}")
        print(f"  Content preview: {chunk.content[:80]}...")
        print(f"  Metadata: destination={chunk.metadata.get('destination')}")
        print()

    # Assertions
    assert len(chunks) > 0, "Should create at least one chunk"
    assert all(c.document_id == doc.id for c in chunks), "All chunks should have document ID"
    assert all(c.metadata.get("destination") == "Kyoto" for c in chunks), "Metadata should be preserved"

    print("✓ All tests passed!")
    return True


def test_text_cleaning():
    """Test text cleaning."""
    print("\n" + "=" * 60)
    print("Testing Text Cleaning")
    print("=" * 60 + "\n")

    processor = SimpleDocumentProcessor()

    dirty_text = """
    This  has   extra   whitespace.


    And multiple newlines.
    """

    cleaned = processor._clean_text(dirty_text)

    print(f"Original: {repr(dirty_text[:50])}...")
    print(f"Cleaned: {repr(cleaned[:50])}...")

    assert "  " not in cleaned, "Should remove double spaces"
    print("\n✓ Text cleaning test passed!")


def test_chunking_config():
    """Test chunking config."""
    print("\n" + "=" * 60)
    print("Testing Chunking Config")
    print("=" * 60 + "\n")

    config = ChunkingConfig()
    print(f"Default config:")
    print(f"  chunk_size: {config.chunk_size}")
    print(f"  chunk_overlap: {config.chunk_overlap}")
    print(f"  min_chunk_size: {config.min_chunk_size}")
    print(f"  max_chunk_size: {config.max_chunk_size}")

    custom = ChunkingConfig(chunk_size=256, chunk_overlap=30)
    assert custom.chunk_size == 256
    print("\n✓ Chunking config test passed!")


def main():
    """Run all tests."""
    print("\n" + "=" * 60)
    print("RAG Core Functionality Tests")
    print("=" * 60 + "\n")

    tests = [
        test_chunking_config,
        test_text_cleaning,
        test_document_processor,
    ]

    passed = 0
    failed = 0

    for test in tests:
        try:
            test()
            passed += 1
        except Exception as e:
            print(f"\n✗ Test failed: {e}")
            failed += 1

    print("\n" + "=" * 60)
    print(f"Results: {passed}/{len(tests)} tests passed")
    print("=" * 60 + "\n")

    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())