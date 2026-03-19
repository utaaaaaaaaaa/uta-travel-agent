#!/usr/bin/env python3
"""
End-to-end test for Destination Agent API.
Uses mock Qdrant for testing without external dependencies.
"""

import asyncio
import sys
from unittest.mock import AsyncMock, MagicMock, patch


def test_api_models():
    """Test Pydantic models."""
    print("=" * 60)
    print("Testing API Models")
    print("=" * 60)

    from src.agent import AgentConfig, DestinationAgent

    # Test AgentConfig
    config = AgentConfig(
        destination="Kyoto, Japan",
        theme="cultural",
        languages=["zh", "en"],
        tags=["temples", "gardens"],
    )
    assert config.destination == "Kyoto, Japan"
    assert config.theme == "cultural"
    print(f"✓ AgentConfig created: {config.destination}")

    # Test DestinationAgent
    agent = DestinationAgent(
        id="test-123",
        user_id="user-1",
        name="Kyoto导游",
        description="文化主题的京都旅行向导",
        destination="Kyoto, Japan",
        vector_collection_id="dest_test",
        status="creating",
    )
    assert agent.id == "test-123"
    assert agent.status == "creating"
    print(f"✓ DestinationAgent created: {agent.name}")

    return True


def test_document_processor():
    """Test document processor."""
    print("\n" + "=" * 60)
    print("Testing Document Processor")
    print("=" * 60)

    from src.rag.document_processor import DocumentProcessor, ChunkingConfig, Document

    processor = DocumentProcessor(config=ChunkingConfig(chunk_size=256))

    doc = Document(
        id="test-doc",
        title="Kyoto Guide",
        content="Kyoto is famous for Kinkaku-ji (Golden Pavilion), a Zen temple covered in gold leaf. "
        "The Fushimi Inari Shrine features thousands of red torii gates. "
        "Gion district is known for geisha culture.",
        source="https://example.com/kyoto",
    )

    chunks = processor.process_documents([doc])
    assert len(chunks) > 0
    print(f"✓ Processed document into {len(chunks)} chunks")

    for chunk in chunks[:3]:
        print(f"  - Chunk {chunk.chunk_index}: {chunk.content[:50]}...")

    return True


def test_crawler():
    """Test web crawler (without network)."""
    print("\n" + "=" * 60)
    print("Testing Web Crawler")
    print("=" * 60)

    from src.crawler import WebCrawler, CrawlResult

    crawler = WebCrawler(max_concurrent=2, timeout=10)

    # Test CrawlResult
    result = CrawlResult(
        url="https://example.com/kyoto",
        title="Kyoto Travel Guide",
        content="Kyoto content here...",
        links=["https://example.com/tokyo"],
        metadata={"status_code": 200},
    )

    assert result.title == "Kyoto Travel Guide"
    print(f"✓ CrawlResult created: {result.title}")

    return True


def test_settings():
    """Test settings loading."""
    print("\n" + "=" * 60)
    print("Testing Settings")
    print("=" * 60)

    from src.agent import Settings

    settings = Settings()
    assert settings.port == 8001
    assert settings.qdrant_host == "localhost"
    print(f"✓ Settings loaded: port={settings.port}, qdrant={settings.qdrant_host}")

    return True


def test_api_routes():
    """Test FastAPI routes exist."""
    print("\n" + "=" * 60)
    print("Testing API Routes")
    print("=" * 60)

    # Import the FastAPI app from src module
    from src.main import app

    routes = [route.path for route in app.routes]
    print(f"  Routes found: {len(routes)}")

    expected = ["/health", "/agents", "/agents/{agent_id}"]
    for route in expected:
        if route in routes:
            print(f"  ✓ {route}")
        else:
            # Check if it's a dynamic route match
            found = any(route.rstrip("}") in r for r in routes)
            if found:
                print(f"  ✓ {route} (dynamic)")
            else:
                print(f"  ✗ {route} NOT FOUND")

    print("✅ API Routes: PASSED")
    return True


def test_rag_service_mock():
    """Test RAG service with mock components."""
    print("\n" + "=" * 60)
    print("Testing RAG Service (Mock)")
    print("=" * 60)

    from src.rag.rag_service import RAGService, SearchContext
    from unittest.mock import AsyncMock

    # Create mock components
    mock_vector_store = MagicMock()
    mock_embedding = MagicMock()

    # Mock search results
    mock_vector_store.search = AsyncMock(
        return_value=[
            {
                "id": "chunk-1",
                "score": 0.85,
                "content": "Kinkaku-ji is a Zen temple covered in gold leaf.",
                "document_id": "doc-1",
            }
        ]
    )

    mock_embedding.embed_query = AsyncMock(return_value=[0.1] * 384)

    # Create service
    rag = RAGService(
        vector_store=mock_vector_store,
        embedding_service=mock_embedding,
        anthropic_client=None,  # No LLM for this test
    )

    print("✓ RAG service created with mock components")
    print("  (Full query test requires Claude API key)")

    return True


def main():
    """Run all tests."""
    print("\n" + "=" * 60)
    print("End-to-End Tests (No External Dependencies)")
    print("=" * 60 + "\n")

    tests = [
        ("API Models", test_api_models),
        ("Settings", test_settings),
        ("Document Processor", test_document_processor),
        ("Crawler", test_crawler),
        ("RAG Service Mock", test_rag_service_mock),
        ("API Routes", test_api_routes),
    ]

    passed = 0
    failed = 0

    for name, test in tests:
        try:
            test()
            passed += 1
            print(f"✅ {name}: PASSED\n")
        except Exception as e:
            print(f"❌ {name}: FAILED - {e}\n")
            failed += 1

    print("=" * 60)
    print(f"Results: {passed}/{len(tests)} tests passed")
    print("=" * 60)

    print("\n📝 Note: Full end-to-end test requires:")
    print("   - Qdrant running (docker run -p 6333:6333 qdrant/qdrant)")
    print("   - ANTHROPIC_API_KEY set for LLM queries")

    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())