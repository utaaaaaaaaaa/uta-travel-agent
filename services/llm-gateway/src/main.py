"""
LLM Gateway FastAPI Application.
"""

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse

from .gateway import (
    ChatRequest,
    ChatResponse,
    LLMGateway,
    RAGRequest,
    Settings,
)

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Global gateway instance
gateway: LLMGateway = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    global gateway
    settings = Settings()
    gateway = LLMGateway(settings)
    logger.info(f"LLM Gateway started on {settings.host}:{settings.port}")
    yield
    logger.info("LLM Gateway shutting down")


app = FastAPI(
    title="UTA LLM Gateway",
    description="LLM Gateway service for UTA Travel Agent",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/health")
async def health():
    """Health check endpoint."""
    return await gateway.health_check()


@app.post("/v1/chat", response_model=ChatResponse)
async def chat(request: ChatRequest):
    """
    Generate a chat completion.
    """
    try:
        return await gateway.complete(request)
    except Exception as e:
        logger.error(f"Chat completion failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/v1/chat/stream")
async def chat_stream(request: ChatRequest):
    """
    Stream a chat completion.
    """

    async def generate():
        async for chunk in gateway.stream(request):
            yield f"data: {chunk}\n\n"
        yield "data: [DONE]\n\n"

    return StreamingResponse(generate(), media_type="text/event-stream")


@app.post("/v1/rag", response_model=ChatResponse)
async def rag_query(request: RAGRequest):
    """
    Execute a RAG-enhanced query.
    """
    try:
        return await gateway.rag_query(request)
    except Exception as e:
        logger.error(f"RAG query failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/v1/rag/stream")
async def rag_stream(request: RAGRequest):
    """
    Stream a RAG-enhanced response.
    """

    async def generate():
        async for chunk in gateway.rag_stream(request):
            yield f"data: {chunk}\n\n"
        yield "data: [DONE]\n\n"

    return StreamingResponse(generate(), media_type="text/event-stream")


@app.post("/v1/vision/analyze", response_model=ChatResponse)
async def analyze_image(
    image_data: bytes,
    prompt: str,
    media_type: str = "image/jpeg",
):
    """
    Analyze an image using Claude Vision.
    """
    try:
        return await gateway.analyze_image(image_data, media_type, prompt)
    except Exception as e:
        logger.error(f"Image analysis failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    import uvicorn

    settings = Settings()
    uvicorn.run(
        "llm_gateway.main:app",
        host=settings.host,
        port=settings.port,
        reload=True,
    )
