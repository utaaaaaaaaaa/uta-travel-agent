"""LLM gRPC Service Implementation."""

import asyncio
import logging
from typing import AsyncIterator

import grpc

# Import generated protobuf code
from agent import llm_pb2, llm_pb2_grpc

from .gateway import LLMGateway, Settings

logger = logging.getLogger(__name__)


class LLMServiceServicer(llm_pb2_grpc.LLMServiceServicer):
    """gRPC servicer for LLM operations."""

    def __init__(self, gateway: LLMGateway):
        self.gateway = gateway

    async def Complete(self, request: llm_pb2.ChatRequest, context) -> llm_pb2.ChatResponse:
        """Generate a chat completion."""
        try:
            messages = [{"role": m.role, "content": m.content} for m in request.messages]

            from .gateway import ChatRequest as PyChatRequest
            py_request = PyChatRequest(
                messages=messages,
                model=request.model if request.HasField("model") else None,
                system=request.system if request.HasField("system") else None,
                max_tokens=request.max_tokens if request.HasField("max_tokens") else None,
                temperature=request.temperature,
            )

            response = await self.gateway.complete(py_request)

            return llm_pb2.ChatResponse(
                content=response.content,
                model=response.model,
                usage=llm_pb2.UsageInfo(
                    input_tokens=response.usage["input_tokens"],
                    output_tokens=response.usage["output_tokens"],
                ),
                stop_reason=response.stop_reason,
            )
        except Exception as e:
            logger.error(f"Complete failed: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))

    async def Stream(
        self, request: llm_pb2.ChatRequest, context
    ) -> AsyncIterator[llm_pb2.StreamChunk]:
        """Stream a chat completion."""
        try:
            messages = [{"role": m.role, "content": m.content} for m in request.messages]

            from .gateway import ChatRequest as PyChatRequest
            py_request = PyChatRequest(
                messages=messages,
                model=request.model if request.HasField("model") else None,
                system=request.system if request.HasField("system") else None,
                max_tokens=request.max_tokens if request.HasField("max_tokens") else None,
                temperature=request.temperature,
            )

            async for chunk in self.gateway.stream(py_request):
                yield llm_pb2.StreamChunk(content=chunk, done=False)

            yield llm_pb2.StreamChunk(content="", done=True)
        except Exception as e:
            logger.error(f"Stream failed: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))

    async def RAGQuery(self, request: llm_pb2.RAGRequest, context) -> llm_pb2.ChatResponse:
        """Execute a RAG-enhanced query."""
        try:
            from .gateway import RAGRequest as PyRAGRequest
            py_request = PyRAGRequest(
                query=request.query,
                context=request.context,
                system_prompt=request.system_prompt if request.HasField("system_prompt") else None,
                model=request.model if request.HasField("model") else None,
                temperature=request.temperature,
            )

            response = await self.gateway.rag_query(py_request)

            return llm_pb2.ChatResponse(
                content=response.content,
                model=response.model,
                usage=llm_pb2.UsageInfo(
                    input_tokens=response.usage["input_tokens"],
                    output_tokens=response.usage["output_tokens"],
                ),
                stop_reason=response.stop_reason,
            )
        except Exception as e:
            logger.error(f"RAGQuery failed: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))

    async def RAGStream(
        self, request: llm_pb2.RAGRequest, context
    ) -> AsyncIterator[llm_pb2.StreamChunk]:
        """Stream a RAG-enhanced response."""
        try:
            from .gateway import RAGRequest as PyRAGRequest
            py_request = PyRAGRequest(
                query=request.query,
                context=request.context,
                system_prompt=request.system_prompt if request.HasField("system_prompt") else None,
                model=request.model if request.HasField("model") else None,
                temperature=request.temperature,
            )

            async for chunk in self.gateway.rag_stream(py_request):
                yield llm_pb2.StreamChunk(content=chunk, done=False)

            yield llm_pb2.StreamChunk(content="", done=True)
        except Exception as e:
            logger.error(f"RAGStream failed: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))

    async def AnalyzeImage(
        self, request: llm_pb2.AnalyzeImageRequest, context
    ) -> llm_pb2.AnalyzeImageResponse:
        """Analyze an image using vision."""
        try:
            response = await self.gateway.analyze_image(
                image_data=request.image_data,
                media_type=request.media_type,
                prompt=request.prompt,
                model=request.model if request.HasField("model") else None,
            )

            return llm_pb2.AnalyzeImageResponse(
                description=response.content,
                model=response.model,
                usage=llm_pb2.UsageInfo(
                    input_tokens=response.usage["input_tokens"],
                    output_tokens=response.usage["output_tokens"],
                ),
            )
        except Exception as e:
            logger.error(f"AnalyzeImage failed: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))

    async def HealthCheck(
        self, request: llm_pb2.HealthCheckRequest, context
    ) -> llm_pb2.HealthCheckResponse:
        """Check service health."""
        result = await self.gateway.health_check()
        return llm_pb2.HealthCheckResponse(
            status=result["status"],
            model=result["model"],
            api_key_configured=result["api_key_configured"],
        )


class LLMGRPCServer:
    """LLM gRPC Server wrapper."""

    def __init__(self, host: str = "0.0.0.0", port: int = 50061):
        self.host = host
        self.port = port
        self.gateway = LLMGateway(Settings())
        self._server = None

    async def start(self):
        """Start the gRPC server."""
        self._server = grpc.aio.server()
        llm_pb2_grpc.add_LLMServiceServicer_to_server(
            LLMServiceServicer(self.gateway), self._server
        )
        self._server.add_insecure_port(f"{self.host}:{self.port}")
        await self._server.start()
        logger.info(f"LLM gRPC server started on {self.host}:{self.port}")

    async def stop(self, grace: float = 5.0):
        """Stop the gRPC server."""
        if self._server:
            await self._server.stop(grace)
            logger.info("LLM gRPC server stopped")

    async def wait_for_termination(self):
        """Wait for server termination."""
        if self._server:
            await self._server.wait_for_termination()


async def serve():
    """Run the LLM gRPC server."""
    server = LLMGRPCServer()
    await server.start()
    await server.wait_for_termination()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    asyncio.run(serve())