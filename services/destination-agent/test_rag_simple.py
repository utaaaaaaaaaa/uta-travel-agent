#!/usr/bin/env python3
"""
Simple RAG test script without external dependencies.
Tests the document processor and basic functionality.
"""

import sys
import os

# Add src to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'src'))


def test_document_processor_basic():
    """Test basic document processor functionality."""
    print("Testing Document Processor...")

    from rag.document_processor import Document, ChunkingConfig, DocumentProcessor

    # Create processor
    config = ChunkingConfig(
        chunk_size=512,
        chunk_overlap=50,
        min_chunk_size=100,
    )
    processor = DocumentProcessor(config)

    # Create test document
    doc = Document(
        id="test-1",
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
        restaurants and ochaya (teahouses), where geiko (Kyoto dialect for geisha)
        and maiko (geiko apprentices) entertain.
        """,
        source="https://example.com/kyoto",
        metadata={"destination": "Kyoto", "theme": "cultural"},
    )

    # Process document
    chunks = processor.process_document(doc)

    print(f"  - Document: {doc.title}")
    print(f"  - Content length: {len(doc.content)} chars")
    print(f"  - Chunks created: {len(chunks)}")

    for i, chunk in enumerate(chunks):
        print(f"    Chunk {i}: {chunk.token_count} tokens, {len(chunk.content)} chars")

    assert len(chunks) > 0, "Should create at least one chunk"
    assert all(c.document_id == doc.id for c in chunks), "All chunks should have document ID"

    print("  ✓ Document processor test passed!\n")
    return True


def test_text_cleaning():
    """Test text cleaning functionality."""
    print("Testing Text Cleaning...")

    from rag.document_processor import DocumentProcessor

    processor = DocumentProcessor()

    dirty_text = """
    This  has   extra   whitespace.


    And multiple newlines.
    'smart quotes' and "double quotes"
    —em dashes—
    """

    cleaned = processor._clean_text(dirty_text)

    assert "  " not in cleaned, "Should remove double spaces"
    assert "\n\n\n" not in cleaned, "Should remove triple newlines"

    print(f"  - Original length: {len(dirty_text)}")
    print(f"  - Cleaned length: {len(cleaned)}")
    print("  ✓ Text cleaning test passed!\n")
    return True


def test_token_counting():
    """Test token counting."""
    print("Testing Token Counting...")

    from rag.document_processor import DocumentProcessor

    processor = DocumentProcessor()

    text = "Hello, world! This is a test."
    count = processor.count_tokens(text)

    print(f"  - Text: '{text}'")
    print(f"  - Token count: {count}")

    assert count > 0, "Token count should be positive"
    assert count < len(text), "Token count should be less than character count"

    print("  ✓ Token counting test passed!\n")
    return True


def test_chunking_config():
    """Test chunking configuration."""
    print("Testing Chunking Config...")

    from rag.document_processor import ChunkingConfig

    config = ChunkingConfig()

    assert config.chunk_size == 512, "Default chunk size should be 512"
    assert config.chunk_overlap == 50, "Default overlap should be 50"

    custom_config = ChunkingConfig(chunk_size=256, chunk_overlap=30)
    assert custom_config.chunk_size == 256

    print(f"  - Default config: chunk_size={config.chunk_size}, overlap={config.chunk_overlap}")
    print("  ✓ Chunking config test passed!\n")
    return True


def main():
    """Run all tests."""
    print("=" * 60)
    print("RAG Core Tests")
    print("=" * 60 + "\n")

    tests = [
        test_chunking_config,
        test_text_cleaning,
        test_token_counting,
        test_document_processor_basic,
    ]

    passed = 0
    failed = 0

    for test in tests:
        try:
            if test():
                passed += 1
        except Exception as e:
            print(f"  ✗ Test failed: {e}\n")
            failed += 1

    print("=" * 60)
    print(f"Results: {passed} passed, {failed} failed")
    print("=" * 60)

    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())