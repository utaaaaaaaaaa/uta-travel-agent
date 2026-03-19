"""Vision gRPC Service Implementation."""

import asyncio
import logging
from typing import Optional

import grpc
from google.protobuf import wrappers_pb2

# Import generated protobuf code
from agent import vision_pb2, vision_pb2_grpc

from .service import VisionService, Settings

logger = logging.getLogger(__name__)


def optional_string(value: Optional[str]) -> wrappers_pb2.StringValue:
    """Convert optional string to protobuf wrapper."""
    if value is None:
        return wrappers_pb2.StringValue()
    return wrappers_pb2.StringValue(value=value)


class VisionServiceServicer(vision_pb2_grpc.VisionServiceServicer):
    """gRPC servicer for Vision operations."""

    def __init__(self, service: VisionService):
        self.service = service

    async def AnalyzeImage(
        self, request: vision_pb2.AnalyzeRequest, context
    ) -> vision_pb2.AnalyzeResponse:
        """Analyze an image with a custom prompt."""
        try:
            result = await self.service.analyze_image(
                image_data=request.image_data,
                media_type=request.media_type,
                prompt=request.prompt,
            )

            return vision_pb2.AnalyzeResponse(
                description=result.description,
                model=result.model,
                usage=vision_pb2.UsageInfo(
                    input_tokens=result.usage["input_tokens"],
                    output_tokens=result.usage["output_tokens"],
                ),
            )
        except ValueError as e:
            logger.error(f"AnalyzeImage validation error: {e}")
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(e))
        except Exception as e:
            logger.error(f"AnalyzeImage failed: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))

    async def RecognizeLandmark(
        self, request: vision_pb2.RecognizeLandmarkRequest, context
    ) -> vision_pb2.RecognizeLandmarkResponse:
        """Recognize a landmark in an image."""
        try:
            destination = None
            if request.HasField("destination"):
                destination = request.destination

            result = await self.service.recognize_landmark(
                image_data=request.image_data,
                media_type=request.media_type,
                destination=destination,
                language=request.language,
            )

            response = vision_pb2.RecognizeLandmarkResponse(
                recognized=result.recognized,
                raw_analysis=result.raw_analysis,
                model=result.model,
            )

            if result.landmark:
                response.landmark = vision_pb2.LandmarkInfo(
                    name=result.landmark.name,
                    confidence=result.landmark.confidence,
                    description=result.landmark.description,
                )
                if result.landmark.category:
                    response.landmark.category = optional_string(result.landmark.category)
                if result.landmark.historical_period:
                    response.landmark.historical_period = optional_string(result.landmark.historical_period)

            return response
        except ValueError as e:
            logger.error(f"RecognizeLandmark validation error: {e}")
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(e))
        except Exception as e:
            logger.error(f"RecognizeLandmark failed: {e}")
            await context.abort(grpc.StatusCode.INTERNAL, str(e))

    async def HealthCheck(
        self, request: vision_pb2.HealthCheckRequest, context
    ) -> vision_pb2.HealthCheckResponse:
        """Check service health."""
        result = await self.service.health_check()
        return vision_pb2.HealthCheckResponse(
            status=result["status"],
            model=result["model"],
            api_key_configured=result["api_key_configured"],
            supported_formats=result["supported_formats"],
        )


class VisionGRPCServer:
    """Vision gRPC Server wrapper."""

    def __init__(self, host: str = "0.0.0.0", port: int = 50063):
        self.host = host
        self.port = port
        self.service = VisionService(Settings())
        self._server = None

    async def start(self):
        """Start the gRPC server."""
        self._server = grpc.aio.server()
        vision_pb2_grpc.add_VisionServiceServicer_to_server(
            VisionServiceServicer(self.service), self._server
        )
        self._server.add_insecure_port(f"{self.host}:{self.port}")
        await self._server.start()
        logger.info(f"Vision gRPC server started on {self.host}:{self.port}")

    async def stop(self, grace: float = 5.0):
        """Stop the gRPC server."""
        if self._server:
            await self._server.stop(grace)
            logger.info("Vision gRPC server stopped")

    async def wait_for_termination(self):
        """Wait for server termination."""
        if self._server:
            await self._server.wait_for_termination()


async def serve():
    """Run the Vision gRPC server."""
    server = VisionGRPCServer()
    await server.start()
    await server.wait_for_termination()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    asyncio.run(serve())