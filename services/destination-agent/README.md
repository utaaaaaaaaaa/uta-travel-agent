# Destination Agent

AI-powered destination knowledge base builder using RAG (Retrieval-Augmented Generation).

## Features

- **Web Crawling**: Automatically gather destination information
- **Document Processing**: Intelligent chunking with token awareness
- **Vector Embeddings**: Multilingual support with sentence-transformers
- **RAG Query**: Context-aware answers using Claude API

## Quick Start

```bash
# Install dependencies
uv pip install -e ".[dev]"

# Run service
uvicorn src.main:app --reload

# Run tests
pytest tests/ -v
```

