"""
Qdrant vector store for document embeddings.

Provides efficient vector storage and retrieval using Qdrant.
"""

import logging
from typing import Optional

from qdrant_client import QdrantClient, models
from qdrant_client.http.exceptions import UnexpectedResponse

from .document_processor import Chunk
from .embeddings import EmbeddingService

logger = logging.getLogger(__name__)


class QdrantVectorStore:
    """
    Vector store using Qdrant.

    Manages collections, indexing, and retrieval.
    """

    def __init__(
        self,
        host: str = "localhost",
        port: int = 6333,
        grpc_port: int = 6334,
        prefer_grpc: bool = True,
        embedding_service: Optional[EmbeddingService] = None,
    ):
        self.host = host
        self.port = port
        self.grpc_port = grpc_port

        # Initialize Qdrant client
        self._client = QdrantClient(
            host=host,
            port=port,
            grpc_port=grpc_port,
            prefer_grpc=prefer_grpc,
        )

        self._embedding_service = embedding_service

    def set_embedding_service(self, service: EmbeddingService) -> None:
        """Set the embedding service."""
        self._embedding_service = service

    async def create_collection(
        self,
        collection_name: str,
        vector_size: Optional[int] = None,
        distance: str = "Cosine",
    ) -> bool:
        """
        Create a new collection for storing vectors.

        Args:
            collection_name: Name of the collection
            vector_size: Dimension of vectors (default: from embedding service)
            distance: Distance metric (Cosine, Euclid, Dot)

        Returns:
            True if created successfully
        """
        if vector_size is None:
            if self._embedding_service is None:
                raise ValueError("Either vector_size or embedding_service must be provided")
            vector_size = self._embedding_service.get_dimension()

        distance_map = {
            "Cosine": models.Distance.COSINE,
            "Euclid": models.Distance.EUCLID,
            "Dot": models.Distance.DOT,
        }

        try:
            self._client.create_collection(
                collection_name=collection_name,
                vectors_config=models.VectorParams(
                    size=vector_size,
                    distance=distance_map.get(distance, models.Distance.COSINE),
                ),
            )
            logger.info(f"Created collection: {collection_name}")
            return True
        except UnexpectedResponse as e:
            if "already exists" in str(e):
                logger.info(f"Collection already exists: {collection_name}")
                return True
            raise

    async def delete_collection(self, collection_name: str) -> bool:
        """
        Delete a collection.

        Args:
            collection_name: Name of the collection to delete

        Returns:
            True if deleted successfully
        """
        try:
            self._client.delete_collection(collection_name=collection_name)
            logger.info(f"Deleted collection: {collection_name}")
            return True
        except Exception as e:
            logger.error(f"Failed to delete collection {collection_name}: {e}")
            return False

    async def collection_exists(self, collection_name: str) -> bool:
        """Check if a collection exists."""
        try:
            self._client.get_collection(collection_name=collection_name)
            return True
        except Exception:
            return False

    async def index_chunks(
        self,
        collection_name: str,
        chunks: list[Chunk],
        batch_size: int = 100,
    ) -> int:
        """
        Index chunks into the collection.

        Args:
            collection_name: Target collection
            chunks: Chunks to index
            batch_size: Number of chunks to index per batch

        Returns:
            Number of chunks indexed
        """
        if not chunks:
            return 0

        if self._embedding_service is None:
            raise ValueError("Embedding service not configured")

        indexed_count = 0

        # Process in batches
        for i in range(0, len(chunks), batch_size):
            batch = chunks[i : i + batch_size]

            # Create embeddings for batch
            texts = [chunk.content for chunk in batch]
            embeddings = await self._embedding_service.embed_texts(texts)

            # Prepare points for Qdrant
            points = []
            for chunk, embedding in zip(batch, embeddings):
                points.append(
                    models.PointStruct(
                        id=chunk.id,
                        vector=embedding,
                        payload={
                            "document_id": chunk.document_id,
                            "content": chunk.content,
                            "chunk_index": chunk.chunk_index,
                            "token_count": chunk.token_count,
                            **chunk.metadata,
                        },
                    )
                )

            # Upsert to Qdrant
            self._client.upsert(
                collection_name=collection_name,
                points=points,
            )

            indexed_count += len(points)
            logger.debug(f"Indexed {indexed_count}/{len(chunks)} chunks")

        logger.info(f"Indexed {indexed_count} chunks into {collection_name}")
        return indexed_count

    async def search(
        self,
        collection_name: str,
        query: str,
        limit: int = 5,
        score_threshold: Optional[float] = None,
        filter_conditions: Optional[dict] = None,
    ) -> list[dict]:
        """
        Search for similar documents.

        Args:
            collection_name: Collection to search
            query: Search query
            limit: Maximum results
            score_threshold: Minimum similarity score
            filter_conditions: Optional filter conditions

        Returns:
            List of search results with scores and payloads
        """
        if self._embedding_service is None:
            raise ValueError("Embedding service not configured")

        # Embed query
        query_vector = await self._embedding_service.embed_query(query)

        # Build filter if provided
        query_filter = None
        if filter_conditions:
            query_filter = models.Filter(**filter_conditions)

        # Search
        results = self._client.search(
            collection_name=collection_name,
            query_vector=query_vector,
            limit=limit,
            score_threshold=score_threshold,
            query_filter=query_filter,
        )

        return [
            {
                "id": str(result.id),
                "score": result.score,
                "content": result.payload.get("content", ""),
                "document_id": result.payload.get("document_id"),
                "metadata": {
                    k: v
                    for k, v in result.payload.items()
                    if k not in ["content", "document_id"]
                },
            }
            for result in results
        ]

    async def search_by_vector(
        self,
        collection_name: str,
        vector: list[float],
        limit: int = 5,
        score_threshold: Optional[float] = None,
    ) -> list[dict]:
        """
        Search using a pre-computed vector.

        Useful when you already have an embedding.
        """
        results = self._client.search(
            collection_name=collection_name,
            query_vector=vector,
            limit=limit,
            score_threshold=score_threshold,
        )

        return [
            {
                "id": str(result.id),
                "score": result.score,
                "content": result.payload.get("content", ""),
                "document_id": result.payload.get("document_id"),
                "metadata": result.payload,
            }
            for result in results
        ]

    async def get_collection_stats(self, collection_name: str) -> dict:
        """Get statistics about a collection."""
        try:
            info = self._client.get_collection(collection_name=collection_name)
            return {
                "name": collection_name,
                "vectors_count": info.vectors_count,
                "points_count": info.points_count,
                "status": info.status.value,
                "config": {
                    "vector_size": info.config.params.vectors.size,
                    "distance": info.config.params.vectors.distance.value,
                },
            }
        except Exception as e:
            logger.error(f"Failed to get collection stats: {e}")
            return {}

    def close(self) -> None:
        """Close the Qdrant client connection."""
        self._client.close()


__all__ = ["QdrantVectorStore"]