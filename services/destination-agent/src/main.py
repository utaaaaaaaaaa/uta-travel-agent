"""
FastAPI main application for Destination Agent service.
"""

import logging
from contextlib import asynccontextmanager
from typing import AsyncGenerator

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

from agent import AgentConfig, AgentService, DestinationAgent, Settings

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Global service instance
service: AgentService | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    global service
    settings = Settings()
    service = AgentService(settings)
    logger.info("Destination Agent service started")
    yield
    logger.info("Destination Agent service stopped")


# Create FastAPI app
app = FastAPI(
    title="Destination Agent Service",
    description="Creates and manages travel destination knowledge bases with RAG",
    version="0.1.0",
    lifespan=lifespan,
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Configure appropriately for production
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# Request/Response models
class CreateAgentRequest(BaseModel):
    user_id: str
    destination: str
    theme: str = "cultural"
    languages: list[str] = ["zh"]
    tags: list[str] = []


class AgentResponse(BaseModel):
    id: str
    user_id: str
    name: str
    description: str
    destination: str
    status: str
    document_count: int
    chunk_count: int = 0


class QueryRequest(BaseModel):
    question: str
    top_k: int = 5


class QueryResponse(BaseModel):
    answer: str
    sources: list[dict]
    confidence: float
    question: str


# API endpoints
@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {
        "status": "healthy",
        "service": "destination-agent",
        "embedding_model": service._embedding_service.get_model_name() if service else None,
    }


@app.post("/agents", response_model=AgentResponse)
async def create_agent(request: CreateAgentRequest):
    """
    Create a new destination agent.

    This will:
    1. Research the destination online
    2. Process and chunk documents
    3. Create vector embeddings
    4. Store in Qdrant

    The agent will be in 'creating' status initially.
    Poll the GET endpoint to check when it's ready.
    """
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    config = AgentConfig(
        destination=request.destination,
        theme=request.theme,
        languages=request.languages,
        tags=request.tags,
    )

    agent = await service.create_agent(request.user_id, config)

    return AgentResponse(
        id=agent.id,
        user_id=agent.user_id,
        name=agent.name,
        description=agent.description,
        destination=agent.destination,
        status=agent.status,
        document_count=agent.document_count,
        chunk_count=agent.chunk_count,
    )


@app.get("/agents/{agent_id}", response_model=AgentResponse)
async def get_agent(agent_id: str):
    """Get an agent by ID."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    agent = await service.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    return AgentResponse(
        id=agent.id,
        user_id=agent.user_id,
        name=agent.name,
        description=agent.description,
        destination=agent.destination,
        status=agent.status,
        document_count=agent.document_count,
        chunk_count=agent.chunk_count,
    )


@app.get("/agents")
async def list_agents(user_id: str):
    """List all agents for a user."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    agents = await service.list_agents(user_id)
    return {
        "agents": [
            AgentResponse(
                id=a.id,
                user_id=a.user_id,
                name=a.name,
                description=a.description,
                destination=a.destination,
                status=a.status,
                document_count=a.document_count,
                chunk_count=a.chunk_count,
            )
            for a in agents
        ]
    }


@app.delete("/agents/{agent_id}")
async def delete_agent(agent_id: str):
    """Delete an agent and its knowledge base."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    deleted = await service.delete_agent(agent_id)
    if not deleted:
        raise HTTPException(status_code=404, detail="Agent not found")

    return {"status": "deleted"}


@app.post("/agents/{agent_id}/query", response_model=QueryResponse)
async def query_agent(agent_id: str, request: QueryRequest):
    """
    Query an agent's knowledge base using RAG.

    The agent must be in 'ready' status to accept queries.
    """
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    try:
        response = await service.query_agent(
            agent_id=agent_id,
            question=request.question,
            top_k=request.top_k,
        )

        return QueryResponse(
            answer=response.answer,
            sources=response.sources,
            confidence=response.confidence,
            question=response.query,
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


@app.post("/agents/{agent_id}/query/stream")
async def query_agent_stream(agent_id: str, request: QueryRequest):
    """
    Query an agent's knowledge base with streaming response.

    Returns the answer as it's generated for real-time display.
    """
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    async def generate() -> AsyncGenerator[str, None]:
        try:
            # For now, return non-streaming response
            # TODO: Implement actual streaming with Claude
            response = await service.query_agent(
                agent_id=agent_id,
                question=request.question,
                top_k=request.top_k,
            )
            yield f"data: {response.answer}\n\n"
            yield "data: [DONE]\n\n"
        except ValueError as e:
            yield f"data: [ERROR] {str(e)}\n\n"

    return StreamingResponse(
        generate(),
        media_type="text/event-stream",
    )


@app.get("/agents/{agent_id}/stats")
async def get_agent_stats(agent_id: str):
    """Get statistics about an agent's knowledge base."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    agent = await service.get_agent(agent_id)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")

    # Get collection stats from Qdrant
    stats = await service._vector_store.get_collection_stats(
        agent.vector_collection_id
    )

    return {
        "agent_id": agent.id,
        "destination": agent.destination,
        "status": agent.status,
        "document_count": agent.document_count,
        "chunk_count": agent.chunk_count,
        "vector_store": stats,
    }


if __name__ == "__main__":
    import uvicorn

    settings = Settings()
    uvicorn.run(app, host=settings.host, port=settings.port)