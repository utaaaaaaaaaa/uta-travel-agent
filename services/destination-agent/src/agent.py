"""
Destination Agent - Creates and manages travel destination knowledge bases.

This agent is responsible for:
1. Researching destination information from web sources
2. Building RAG knowledge bases for specific destinations
3. Persisting agent instances for later use
"""

import asyncio
import hashlib
import logging
import time
from typing import Optional

from anthropic import AsyncAnthropic
from pydantic import BaseModel, Field
from pydantic_settings import BaseSettings

from .crawler import CrawlResult, WebCrawler
from .rag import (
    ChunkingConfig,
    DestinationKnowledgeBase,
    Document,
    DocumentProcessor,
    EmbeddingService,
    QdrantVectorStore,
    RAGService,
)

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

    # Embedding
    embedding_model: str = "multilingual"

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
    chunk_count: int = 0
    language: str = "zh"
    tags: list[str] = Field(default_factory=list)
    theme: str = "cultural"
    status: str = "creating"


class AgentService:
    """Service for managing destination agents with full RAG pipeline."""

    def __init__(self, settings: Settings):
        self.settings = settings
        self._agents: dict[str, DestinationAgent] = {}

        # Initialize components
        self._init_components()

    def _init_components(self) -> None:
        """Initialize RAG components."""
        # Embedding service
        self._embedding_service = EmbeddingService(model=self.settings.embedding_model)

        # Vector store
        self._vector_store = QdrantVectorStore(
            host=self.settings.qdrant_host,
            port=self.settings.qdrant_port,
            embedding_service=self._embedding_service,
        )

        # Document processor
        self._document_processor = DocumentProcessor(
            config=ChunkingConfig(
                chunk_size=512,
                chunk_overlap=50,
            )
        )

        # Web crawler
        self._crawler = WebCrawler(max_concurrent=5, timeout=30)

        # Anthropic client (if API key provided)
        self._anthropic_client = None
        if self.settings.anthropic_api_key:
            self._anthropic_client = AsyncAnthropic(
                api_key=self.settings.anthropic_api_key
            )

        # RAG service
        self._rag_service = RAGService(
            vector_store=self._vector_store,
            embedding_service=self._embedding_service,
            anthropic_client=self._anthropic_client,
        )

        logger.info("AgentService initialized with RAG components")

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
        collection_name = f"dest_{agent_id}"

        logger.info(f"Creating agent for {config.destination}")

        agent = DestinationAgent(
            id=agent_id,
            user_id=user_id,
            name=f"{config.destination}导游",
            description=f"{config.theme}主题的{config.destination}旅行向导",
            destination=config.destination,
            vector_collection_id=collection_name,
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
        agent = self._agents.get(agent_id)
        if not agent:
            return False

        # Delete from Qdrant
        try:
            await self._vector_store.delete_collection(agent.vector_collection_id)
        except Exception as e:
            logger.error(f"Failed to delete collection: {e}")

        del self._agents[agent_id]
        return True

    async def query_agent(
        self,
        agent_id: str,
        question: str,
        top_k: int = 5,
    ):
        """
        Query an agent's knowledge base.

        Returns RAG response with answer and sources.
        """
        agent = self._agents.get(agent_id)
        if not agent:
            raise ValueError(f"Agent {agent_id} not found")

        if agent.status != "ready":
            raise ValueError(f"Agent {agent_id} is not ready (status: {agent.status})")

        # Create knowledge base interface
        kb = DestinationKnowledgeBase(
            rag_service=self._rag_service,
            vector_store=self._vector_store,
        )

        return await kb.ask(
            collection_name=agent.vector_collection_id,
            question=question,
            top_k=top_k,
        )

    async def _process_agent_creation(
        self,
        agent: DestinationAgent,
        config: AgentConfig,
    ) -> None:
        """Background task to process agent creation."""
        try:
            # Step 1: Research destination
            logger.info(f"[{agent.id}] Step 1: Researching {config.destination}")
            crawl_results = await self._research_destination(config.destination, config.theme)

            # Step 2: Convert to documents
            documents = self._crawl_to_documents(crawl_results, config.destination)
            agent.document_count = len(documents)
            logger.info(f"[{agent.id}] Found {len(documents)} documents")

            # Step 3: Process and chunk documents
            logger.info(f"[{agent.id}] Step 2: Processing documents")
            chunks = self._document_processor.process_documents(documents)
            agent.chunk_count = len(chunks)
            logger.info(f"[{agent.id}] Created {len(chunks)} chunks")

            # Step 4: Create collection and index
            logger.info(f"[{agent.id}] Step 3: Creating vector collection")
            await self._vector_store.create_collection(
                collection_name=agent.vector_collection_id,
                vector_size=self._embedding_service.get_dimension(),
            )

            logger.info(f"[{agent.id}] Step 4: Indexing chunks")
            indexed = await self._vector_store.index_chunks(
                collection_name=agent.vector_collection_id,
                chunks=chunks,
                batch_size=50,
            )
            logger.info(f"[{agent.id}] Indexed {indexed} chunks")

            # Update agent status
            agent.status = "ready"
            logger.info(f"[{agent.id}] Agent creation complete!")

        except Exception as e:
            logger.error(f"[{agent.id}] Agent creation failed: {e}", exc_info=True)
            agent.status = "failed"

    async def _research_destination(
        self,
        destination: str,
        theme: str,
    ) -> list[CrawlResult]:
        """
        Research destination information from web sources.

        For MVP, uses curated URLs. In production, would use search API.
        """
        # Build search URLs based on destination and theme
        # For demo, we'll use Wikipedia and travel sites
        search_queries = self._build_search_queries(destination, theme)
        start_urls = self._get_start_urls(destination, theme)

        logger.info(f"Crawling {len(start_urls)} starting URLs for {destination}")

        # Crawl with limits
        results = await self._crawler.crawl(
            start_urls=start_urls,
            max_pages=20,
            allowed_domains=None,  # Allow all domains for now
        )

        return results

    def _build_search_queries(self, destination: str, theme: str) -> list[str]:
        """Build search queries for the destination."""
        theme_keywords = {
            "cultural": ["历史", "文化", "寺庙", "博物馆", "古迹"],
            "food": ["美食", "餐厅", "小吃", "料理"],
            "adventure": ["户外", "徒步", "探险", "自然"],
            "art": ["美术馆", "艺术", "画廊", "设计"],
        }

        keywords = theme_keywords.get(theme, [])
        queries = [f"{destination} {kw}" for kw in keywords]
        queries.insert(0, f"{destination} 旅游攻略")
        queries.insert(0, f"{destination} 简介")

        return queries

    def _get_start_urls(self, destination: str, theme: str) -> list[str]:
        """Get starting URLs for crawling."""
        # For MVP, use Wikipedia and travel sites
        # In production, would integrate with search APIs
        encoded_dest = destination.replace(" ", "_")

        urls = [
            f"https://zh.wikipedia.org/wiki/{encoded_dest}",
            f"https://en.wikipedia.org/wiki/{encoded_dest}",
        ]

        # Add travel site URLs (example structure)
        # urls.append(f"https://www.tripadvisor.com/Search?q={destination}")

        return urls

    def _crawl_to_documents(
        self,
        crawl_results: list[CrawlResult],
        destination: str,
    ) -> list[Document]:
        """Convert crawl results to documents."""
        documents = []

        for i, result in enumerate(crawl_results):
            if not result.content or len(result.content) < 100:
                continue

            doc = Document(
                id=hashlib.md5(result.url.encode()).hexdigest()[:12],
                title=result.title or f"{destination} 文档 {i+1}",
                content=result.content,
                source=result.url,
                metadata={
                    "destination": destination,
                    "crawl_status": result.metadata.get("status_code"),
                },
            )
            documents.append(doc)

        return documents

    def _generate_agent_id(self, destination: str) -> str:
        """Generate a unique agent ID."""
        hash_input = f"{destination}_{time.time()}"
        return hashlib.md5(hash_input.encode()).hexdigest()[:12]


# FastAPI app will be defined in main.py
__all__ = [
    "Settings",
    "AgentConfig",
    "DestinationAgent",
    "AgentService",
]