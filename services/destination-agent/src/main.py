"""
FastAPI main application for Destination Agent service.
"""

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
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
    description="Creates and manages travel destination knowledge bases",
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


# API endpoints
@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy", "service": "destination-agent"}


@app.post("/agents", response_model=AgentResponse)
async def create_agent(request: CreateAgentRequest):
    """Create a new destination agent."""
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
            )
            for a in agents
        ]
    }


@app.delete("/agents/{agent_id}")
async def delete_agent(agent_id: str):
    """Delete an agent."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    deleted = await service.delete_agent(agent_id)
    if not deleted:
        raise HTTPException(status_code=404, detail="Agent not found")

    return {"status": "deleted"}


if __name__ == "__main__":
    import uvicorn

    settings = Settings()
    uvicorn.run(app, host=settings.host, port=settings.port)