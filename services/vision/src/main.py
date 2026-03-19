"""
Vision Service FastAPI Application.
"""

import base64
import logging
from contextlib import asynccontextmanager
from io import BytesIO

import PIL.Image
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import JSONResponse

from .service import (
    AnalyzeResponse,
    LandmarkResponse,
    Settings,
    VisionService,
)

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Global service instance
service: VisionService = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    global service
    settings = Settings()
    service = VisionService(settings)
    logger.info(f"Vision Service started on {settings.host}:{settings.port}")
    yield
    logger.info("Vision Service shutting down")


app = FastAPI(
    title="UTA Vision Service",
    description="Image analysis service for UTA Travel Agent",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/health")
async def health():
    """Health check endpoint."""
    return await service.health_check()


@app.post("/v1/analyze", response_model=AnalyzeResponse)
async def analyze_image(
    file: UploadFile = File(...),
    prompt: str = Form(default="请描述这张图片的内容。"),
):
    """
    Analyze an uploaded image with a custom prompt.
    """
    try:
        # Read image data
        image_data = await file.read()

        # Detect media type
        media_type = file.content_type
        if not media_type or media_type == "application/octet-stream":
            # Try to detect from image content
            try:
                img = PIL.Image.open(BytesIO(image_data))
                format_map = {
                    "JPEG": "image/jpeg",
                    "PNG": "image/png",
                    "WEBP": "image/webp",
                    "GIF": "image/gif",
                }
                media_type = format_map.get(img.format, "image/jpeg")
            except Exception:
                media_type = "image/jpeg"

        return await service.analyze_image(image_data, media_type, prompt)

    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Image analysis failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/v1/landmark", response_model=LandmarkResponse)
async def recognize_landmark(
    file: UploadFile = File(...),
    destination: str = Form(default=None),
    language: str = Form(default="zh"),
):
    """
    Recognize a landmark in an uploaded image.

    Optionally provide a destination hint to improve recognition.
    """
    try:
        # Read image data
        image_data = await file.read()

        # Detect media type
        media_type = file.content_type
        if not media_type or media_type == "application/octet-stream":
            try:
                img = PIL.Image.open(BytesIO(image_data))
                format_map = {
                    "JPEG": "image/jpeg",
                    "PNG": "image/png",
                    "WEBP": "image/webp",
                    "GIF": "image/gif",
                }
                media_type = format_map.get(img.format, "image/jpeg")
            except Exception:
                media_type = "image/jpeg"

        return await service.recognize_landmark(
            image_data,
            media_type,
            destination=destination,
            language=language,
        )

    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Landmark recognition failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/v1/analyze/base64", response_model=AnalyzeResponse)
async def analyze_image_base64(
    image_base64: str,
    media_type: str = "image/jpeg",
    prompt: str = "请描述这张图片的内容。",
):
    """
    Analyze an image provided as base64.
    """
    try:
        image_data = base64.b64decode(image_base64)
        return await service.analyze_image(image_data, media_type, prompt)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Image analysis failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    import uvicorn

    settings = Settings()
    uvicorn.run(
        "vision.main:app",
        host=settings.host,
        port=settings.port,
        reload=True,
    )