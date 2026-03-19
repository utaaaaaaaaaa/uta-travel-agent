"""
Vision Service for UTA Travel Agent.

Provides image analysis capabilities using Claude Vision.
"""

import base64
import logging
from io import BytesIO
from typing import Optional

from anthropic import AsyncAnthropic
from pydantic import BaseModel, Field
from pydantic_settings import BaseSettings

logger = logging.getLogger(__name__)


class Settings(BaseSettings):
    """Service settings."""

    host: str = "0.0.0.0"
    port: int = 8003

    # Claude API
    anthropic_api_key: str = ""
    default_model: str = "claude-sonnet-4-20250514"
    max_tokens: int = 1024

    # Image processing
    max_image_size: int = 5 * 1024 * 1024  # 5MB
    supported_formats: list[str] = ["image/jpeg", "image/png", "image/webp", "image/gif"]

    class Config:
        env_file = ".env"


class AnalyzeRequest(BaseModel):
    """Request for image analysis."""

    prompt: str = "请描述这张图片的内容。"
    detail_level: str = "normal"  # low, normal, high


class LandmarkRecognitionRequest(BaseModel):
    """Request for landmark recognition."""

    destination: Optional[str] = None  # Hint for the destination
    language: str = "zh"


class AnalyzeResponse(BaseModel):
    """Response from image analysis."""

    description: str
    model: str
    usage: dict


class LandmarkInfo(BaseModel):
    """Information about a recognized landmark."""

    name: str
    confidence: float = Field(..., ge=0.0, le=1.0)
    description: str
    category: Optional[str] = None  # temple, museum, nature, etc.
    historical_period: Optional[str] = None


class LandmarkResponse(BaseModel):
    """Response from landmark recognition."""

    recognized: bool
    landmark: Optional[LandmarkInfo] = None
    raw_analysis: str
    model: str


class VisionService:
    """
    Vision analysis service using Claude Vision.
    """

    LANDMARK_PROMPT_ZH = """你是一位专业的旅游向导和图像识别专家。
请分析这张图片，识别其中的地标、景点或名胜古迹。

请按以下格式回答：
1. 如果能识别出地标，请提供：
   - 名称
   - 简介（50字以内）
   - 类别（寺庙/博物馆/自然景观/历史建筑等）

2. 如果无法识别，请：
   - 描述图片内容
   - 猜测可能的地点或类型

请用中文回答。"""

    LANDMARK_PROMPT_EN = """You are a professional tour guide and image recognition expert.
Please analyze this image and identify any landmarks, attractions, or famous sites.

Please respond in the following format:
1. If you can identify the landmark, please provide:
   - Name
   - Brief description (within 50 words)
   - Category (temple/museum/nature/historical building, etc.)

2. If you cannot identify it, please:
   - Describe the image content
   - Guess possible location or type

Please respond in English."""

    def __init__(self, settings: Settings):
        self.settings = settings
        self._client: Optional[AsyncAnthropic] = None

    @property
    def client(self) -> AsyncAnthropic:
        """Lazy load Anthropic client."""
        if self._client is None:
            if not self.settings.anthropic_api_key:
                raise ValueError("ANTHROPIC_API_KEY is required")
            self._client = AsyncAnthropic(
                api_key=self.settings.anthropic_api_key
            )
        return self._client

    async def analyze_image(
        self,
        image_data: bytes,
        media_type: str,
        prompt: str,
    ) -> AnalyzeResponse:
        """
        Analyze an image with a custom prompt.
        """
        # Validate media type
        if media_type not in self.settings.supported_formats:
            raise ValueError(f"Unsupported media type: {media_type}")

        # Validate size
        if len(image_data) > self.settings.max_image_size:
            raise ValueError(f"Image too large: {len(image_data)} bytes")

        # Encode to base64
        image_base64 = base64.b64encode(image_data).decode("utf-8")

        # Call Claude Vision
        response = await self.client.messages.create(
            model=self.settings.default_model,
            max_tokens=self.settings.max_tokens,
            messages=[
                {
                    "role": "user",
                    "content": [
                        {
                            "type": "image",
                            "source": {
                                "type": "base64",
                                "media_type": media_type,
                                "data": image_base64,
                            },
                        },
                        {
                            "type": "text",
                            "text": prompt,
                        },
                    ],
                }
            ],
        )

        return AnalyzeResponse(
            description=response.content[0].text,
            model=response.model,
            usage={
                "input_tokens": response.usage.input_tokens,
                "output_tokens": response.usage.output_tokens,
            },
        )

    async def recognize_landmark(
        self,
        image_data: bytes,
        media_type: str,
        destination: Optional[str] = None,
        language: str = "zh",
    ) -> LandmarkResponse:
        """
        Recognize a landmark in an image.

        Uses specialized prompts for landmark identification.
        """
        # Select prompt based on language
        if language == "zh":
            base_prompt = self.LANDMARK_PROMPT_ZH
        else:
            base_prompt = self.LANDMARK_PROMPT_EN

        # Add destination hint if provided
        if destination:
            if language == "zh":
                prompt = f"提示：这张照片可能拍摄于 {destination}。\n\n{base_prompt}"
            else:
                prompt = f"Hint: This photo might be taken in {destination}.\n\n{base_prompt}"
        else:
            prompt = base_prompt

        # Analyze
        result = await self.analyze_image(image_data, media_type, prompt)

        # Parse result to extract landmark info
        landmark_info = self._parse_landmark_result(result.description, language)

        return LandmarkResponse(
            recognized=landmark_info is not None,
            landmark=landmark_info,
            raw_analysis=result.description,
            model=result.model,
        )

    def _parse_landmark_result(self, text: str, language: str) -> Optional[LandmarkInfo]:
        """
        Parse the analysis result to extract landmark information.

        This is a simple heuristic parser. In production, you might use
        structured output or a more sophisticated parsing approach.
        """
        # Look for key indicators
        lines = text.strip().split("\n")

        # Simple extraction - look for name patterns
        name = None
        description = ""
        category = None

        for i, line in enumerate(lines):
            line = line.strip()
            if not line:
                continue

            # Look for name indicators
            if language == "zh":
                if "名称" in line or "这是" in line:
                    # Extract name
                    if "：" in line:
                        name = line.split("：", 1)[1].strip()
                    elif "是" in line:
                        name = line.split("是", 1)[1].strip().split("，")[0]
                elif "类别" in line and "：" in line:
                    category = line.split("：", 1)[1].strip()
            else:
                if "name:" in line.lower() or "this is" in line.lower():
                    if ":" in line:
                        name = line.split(":", 1)[1].strip()
                elif "category:" in line.lower():
                    category = line.split(":", 1)[1].strip()

            # Collect description
            if len(line) > 20 and not line.startswith(("-", "*", "#")):
                description += line + " "

        if name:
            # Clean up
            description = description.strip()[:200]  # Limit length

            # Estimate confidence based on text content
            confidence = 0.8 if "确定" in text or "certain" in text.lower() else 0.6

            return LandmarkInfo(
                name=name,
                confidence=confidence,
                description=description,
                category=category,
            )

        return None

    async def health_check(self) -> dict:
        """Check service health."""
        return {
            "status": "healthy",
            "model": self.settings.default_model,
            "api_key_configured": bool(self.settings.anthropic_api_key),
            "supported_formats": self.settings.supported_formats,
        }


# Singleton instance
_service: Optional[VisionService] = None


def get_service() -> VisionService:
    """Get the vision service singleton."""
    global _service
    if _service is None:
        _service = VisionService(Settings())
    return _service