"""Embedding gRPC Service Implementation."""

import asyncio
import logging

import grpc

# Import generated protobuf code
from agent import embedding_pb2, embedding_pb2_grpc

from .service import EmbeddingService, Settings

logger = logging.getLogger(__name__)


class EmbeddingServiceServicer(embedding_pb2_grpc.EmbeddingServiceServicer):
    """gRPC servicer for Embedding operations."""

    def __init__(self, service: EmbeddingService):
        self.service = service

    async def Embed(self, request: embedding_pb2.EmbedRequest, context) -> embedding_pb2.EmbedResponse:
        """Create embeddings for texts."""
        try:
            embeddings, cached_count = await self.service.embed(
                list(request.texts),
                use_cache=request.use_cache,
            )

            proto_embeddings = [
                embedding_pb2.Embedding(values=list(e)) for e in embeddings
            ]

            return embedding_pb2.EmbedResponse(
                embeddings=proto_embeddings,
                model=self.service.get_model_name(),
                dimension=self.service.get_dimension(),
                cached_count=cached_count,
            )
        except Exception as e:
            logger.error(f"Embed failed: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))

    async def BatchEmbed(
        self, request: embedding_pb2.BatchEmbedRequest, context
    ) -> embedding_pb2.EmbedResponse:
        """Create embeddings for a large batch."""
        try:
            embeddings, cached_count = await self.service.embed(
                list(request.texts),
                use_cache=True,
            )

            proto_embeddings = [
                embedding_pb2.Embedding(values=list(e)) for e in embeddings
            ]

            return embedding_pb2.EmbedResponse(
                embeddings=proto_embeddings,
                model=self.service.get_model_name(),
                dimension=self.service.get_dimension(),
                cached_count=cached_count,
            )
        except Exception as e:
            logger.error(f"BatchEmbed failed: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))

    async def GetModelInfo(
        self, request: embedding_pb2.ModelInfoRequest, context
    ) -> embedding_pb2.ModelInfoResponse:
        """Get model information."""
        from .service import SentenceTransformerEmbedding

        presets = {
            k: v for k, v in SentenceTransformerEmbedding.MODEL_PRESETS.items()
        }

        return embedding_pb2.ModelInfoResponse(
            model=self.service.get_model_name(),
            dimension=self.service.get_dimension(),
            presets=presets,
        )

    async def ClearCache(
        self, request: embedding_pb2.ClearCacheRequest, context
    ) -> embedding_pb2.ClearCacheResponse:
        """Clear the embedding cache."""
        count = self.service.clear_cache()
        return embedding_pb2.ClearCacheResponse(cleared=count)

    async def HealthCheck(
        self, request: embedding_pb2.HealthCheckRequest, context
    ) -> embedding_pb2.HealthCheckResponse:
        """Check service health."""
        result = await self.service.health_check()
        return embedding_pb2.HealthCheckResponse(
            status=result["status"],
            model=result["model"],
            dimension=result["dimension"],
            cache_size=result["cache_size"],
        )


class EmbeddingGRPCServer:
    """Embedding gRPC Server wrapper."""

    def __init__(self, host: str = "0.0.0.0", port: int = 50062):
        self.host = host
        self.port = port
        self.service = EmbeddingService(Settings())
        self._server = None

    async def start(self):
        """Start the gRPC server."""
        self._server = grpc.aio.server()
        embedding_pb2_grpc.add_EmbeddingServiceServicer_to_server(
            EmbeddingServiceServicer(self.service), self._server
        )
        self._server.add_insecure_port(f"{self.host}:{self.port}")
        await self._server.start()
        logger.info(f"Embedding gRPC server started on {self.host}:{self.port}")

    async def stop(self, grace: float = 5.0):
        """Stop the gRPC server."""
        if self._server:
            await self._server.stop(grace)
            logger.info("Embedding gRPC server stopped")

    async def wait_for_termination(self):
        """Wait for server termination."""
        if self._server:
            await self._server.wait_for_termination()


async def serve():
    """Run the Embedding gRPC server."""
    server = EmbeddingGRPCServer()
    await server.start()
    await server.wait_for_termination()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    asyncio.run(serve())