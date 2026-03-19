"""
Embedding Service FastAPI Application.
"""

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException

from .service import (
    BatchEmbedRequest,
    EmbedRequest,
    EmbedResponse,
    EmbeddingService,
    Settings,
)

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Global service instance
service: EmbeddingService = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    global service
    settings = Settings()
    service = EmbeddingService(settings)
    logger.info(f"Embedding Service started on {settings.host}:{settings.port}")
    logger.info(f"Model: {service.get_model_name()}, Dimension: {service.get_dimension()}")
    yield
    logger.info("Embedding Service shutting down")


app = FastAPI(
    title="UTA Embedding Service",
    description="Text embedding service for UTA Travel Agent",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/health")
async def health():
    """Health check endpoint."""
    return await service.health_check()


@app.post("/v1/embed", response_model=EmbedResponse)
async def embed(request: EmbedRequest):
    """
    Create embeddings for texts.

    Supports caching for repeated texts.
    """
    try:
        embeddings, cached_count = await service.embed(
            request.texts,
            use_cache=request.use_cache,
        )
        return EmbedResponse(
            embeddings=embeddings,
            model=service.get_model_name(),
            dimension=service.get_dimension(),
            cached_count=cached_count,
        )
    except Exception as e:
        logger.error(f"Embedding failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/v1/embed/batch", response_model=EmbedResponse)
async def embed_batch(request: BatchEmbedRequest):
    """
    Create embeddings for a large batch of texts.

    Processes in smaller batches internally.
    """
    try:
        embeddings, cached_count = await service.embed(
            request.texts,
            use_cache=True,
        )
        return EmbedResponse(
            embeddings=embeddings,
            model=service.get_model_name(),
            dimension=service.get_dimension(),
            cached_count=cached_count,
        )
    except Exception as e:
        logger.error(f"Batch embedding failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/v1/model/info")
async def model_info():
    """Get model information."""
    return {
        "model": service.get_model_name(),
        "dimension": service.get_dimension(),
    }


@app.post("/v1/cache/clear")
async def clear_cache():
    """Clear the embedding cache."""
    count = service.clear_cache()
    return {"cleared": count}


if __name__ == "__main__":
    import uvicorn

    settings = Settings()
    uvicorn.run(
        "embedding.main:app",
        host=settings.host,
        port=settings.port,
        reload=True,
    )