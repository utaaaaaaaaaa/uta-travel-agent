"""
Destination Agent - Creates and manages travel destination knowledge bases.

This agent is responsible for:
1. Researching destination information from web sources
2. Building RAG knowledge bases for specific destinations
3. Persisting agent instances for later use
"""

import asyncio
import logging
from typing import Optional

from pydantic import BaseModel, Field
from pydantic_settings import BaseSettings

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    # Server
    host: str = "0.0.0.0"
    port: int = 8001
    debug: bool = False

    # gRPC
    grpc_host: str = "localhost"
    grpc_port: int = 50051

    # Qdrant
    qdrant_host: str = "localhost"
    qdrant_port: int = 6333

    # Claude API
    anthropic_api_key: str = ""

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"


class AgentConfig(BaseModel):
    """Configuration for creating a new destination agent."""

    destination: str = Field(..., description="Destination name, e.g., 'Kyoto, Japan'")
    theme: str = Field(default="cultural", description="Theme: cultural, food, adventure, art")
    languages: list[str] = Field(default=["zh"], description="Supported languages")
    tags: list[str] = Field(default_factory=list, description="Additional tags")


class DestinationAgent(BaseModel):
    """A persisted destination agent with RAG knowledge."""

    id: str
    user_id: str
    name: str
    description: str
    destination: str
    vector_collection_id: str
    document_count: int = 0
    language: str = "zh"
    tags: list[str] = Field(default_factory=list)
    theme: str = "cultural"
    status: str = "creating"


class AgentService:
    """Service for managing destination agents."""

    def __init__(self, settings: Settings):
        self.settings = settings
        self._agents: dict[str, DestinationAgent] = {}

    async def create_agent(
        self,
        user_id: str,
        config: AgentConfig,
    ) -> DestinationAgent:
        """
        Create a new destination agent.

        This involves:
        1. Researching destination information
        2. Processing and chunking documents
        3. Creating vector embeddings
        4. Storing in Qdrant
        5. Persisting metadata
        """
        agent_id = self._generate_agent_id(config.destination)

        logger.info(f"Creating agent for {config.destination}")

        agent = DestinationAgent(
            id=agent_id,
            user_id=user_id,
            name=f"{config.destination}导游",
            description=f"{config.theme}主题的{config.destination}旅行向导",
            destination=config.destination,
            vector_collection_id=f"dest_{agent_id}",
            language=config.languages[0] if config.languages else "zh",
            tags=config.tags,
            theme=config.theme,
            status="creating",
        )

        # Store in memory (will be persisted to PostgreSQL later)
        self._agents[agent_id] = agent

        # Start async creation process
        asyncio.create_task(self._process_agent_creation(agent, config))

        return agent

    async def get_agent(self, agent_id: str) -> Optional[DestinationAgent]:
        """Retrieve an agent by ID."""
        return self._agents.get(agent_id)

    async def list_agents(self, user_id: str) -> list[DestinationAgent]:
        """List all agents for a user."""
        return [a for a in self._agents.values() if a.user_id == user_id]

    async def delete_agent(self, agent_id: str) -> bool:
        """Delete an agent and its knowledge base."""
        if agent_id in self._agents:
            del self._agents[agent_id]
            # TODO: Delete from Qdrant and PostgreSQL
            return True
        return False

    async def _process_agent_creation(
        self,
        agent: DestinationAgent,
        config: AgentConfig,
    ) -> None:
        """Background task to process agent creation."""
        try:
            # Step 1: Research
            logger.info(f"[{agent.id}] Step 1: Researching {config.destination}")
            documents = await self._research_destination(config.destination, config.theme)

            # Step 2: Process documents
            logger.info(f"[{agent.id}] Step 2: Processing {len(documents)} documents")
            chunks = await self._process_documents(documents)

            # Step 3: Create vector embeddings
            logger.info(f"[{agent.id}] Step 3: Creating embeddings for {len(chunks)} chunks")
            await self._create_embeddings(agent.vector_collection_id, chunks)

            # Update agent status
            agent.document_count = len(documents)
            agent.status = "ready"
            logger.info(f"[{agent.id}] Agent creation complete")

        except Exception as e:
            logger.error(f"[{agent.id}] Agent creation failed: {e}")
            agent.status = "failed"

    async def _research_destination(
        self,
        destination: str,
        theme: str,
    ) -> list[dict]:
        """Research destination information from web sources."""
        # TODO: Implement actual web scraping
        # This would use trafilatura/BeautifulSoup to scrape travel sites
        logger.info(f"Researching {destination} with theme {theme}")
        return [
            {"title": f"{destination}简介", "content": "..."},
            {"title": f"{destination}景点", "content": "..."},
        ]

    async def _process_documents(self, documents: list[dict]) -> list[dict]:
        """Process and chunk documents for embedding."""
        # TODO: Implement document chunking with tiktoken
        return documents

    async def _create_embeddings(
        self,
        collection_id: str,
        chunks: list[dict],
    ) -> None:
        """Create vector embeddings and store in Qdrant."""
        # TODO: Implement embedding creation and Qdrant storage
        logger.info(f"Creating embeddings in collection {collection_id}")

    def _generate_agent_id(self, destination: str) -> str:
        """Generate a unique agent ID."""
        import hashlib
        import time

        hash_input = f"{destination}_{time.time()}"
        return hashlib.md5(hash_input.encode()).hexdigest()[:12]


# FastAPI app will be defined in main.py
__all__ = ["Settings", "AgentConfig", "DestinationAgent", "AgentService"]