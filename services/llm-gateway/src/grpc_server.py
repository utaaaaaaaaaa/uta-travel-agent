"""gRPC Server base module."""

import asyncio
import logging
from concurrent import futures
from typing import Optional

import grpc
from pydantic_settings import BaseSettings

logger = logging.getLogger(__name__)


class GRPCServerSettings(BaseSettings):
    """Settings for gRPC server."""

    grpc_host: str = "0.0.0.0"
    grpc_port: int = 50061
    max_workers: int = 10

    class Config:
        env_file = ".env"


class GRPCServer:
    """Base gRPC server with graceful shutdown."""

    def __init__(self, host: str, port: int, max_workers: int = 10):
        self.host = host
        self.port = port
        self.max_workers = max_workers
        self._server: Optional[grpc.aio.Server] = None

    async def start(self):
        """Start the gRPC server."""
        self._server = grpc.aio.server(
            futures.ThreadPoolExecutor(max_workers=self.max_workers)
        )
        await self._register_services()
        self._server.add_insecure_port(f"{self.host}:{self.port}")
        await self._server.start()
        logger.info(f"gRPC server started on {self.host}:{self.port}")

    async def stop(self, grace: float = 5.0):
        """Stop the gRPC server gracefully."""
        if self._server:
            await self._server.stop(grace)
            logger.info("gRPC server stopped")

    async def wait_for_termination(self):
        """Wait for server termination."""
        if self._server:
            await self._server.wait_for_termination()

    async def _register_services(self):
        """Override to register gRPC services."""
        raise NotImplementedError("Subclasses must implement _register_services")


def run_server(server: GRPCServer):
    """Run a gRPC server with graceful shutdown."""

    async def serve():
        await server.start()
        await server.wait_for_termination()

    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)

    try:
        loop.run_until_complete(serve())
    except KeyboardInterrupt:
        pass
    finally:
        loop.run_until_complete(server.stop())
        loop.close()