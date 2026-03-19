"""
RAG (Retrieval-Augmented Generation) module for destination knowledge.

This module provides:
- Document processing and chunking
- Vector embeddings
- Qdrant vector storage
- RAG query service
"""

from .document_processor import Chunk, ChunkingConfig, Document, DocumentProcessor
from .embeddings import EmbeddingModel, EmbeddingService, SentenceTransformerEmbedding
from .rag_service import DestinationKnowledgeBase, RAGResponse, RAGService, SearchContext
from .vector_store import QdrantVectorStore

__all__ = [
    # Document Processing
    "Document",
    "Chunk",
    "ChunkingConfig",
    "DocumentProcessor",
    # Embeddings
    "EmbeddingModel",
    "SentenceTransformerEmbedding",
    "EmbeddingService",
    # Vector Store
    "QdrantVectorStore",
    # RAG Service
    "RAGResponse",
    "SearchContext",
    "RAGService",
    "DestinationKnowledgeBase",
]