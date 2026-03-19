"""
LLM Gateway Service for UTA Travel Agent.

This service provides a unified interface for LLM operations.
Supports multiple providers: Anthropic (Claude), OpenAI, GLM (智谱 AI), DeepSeek, and Mock.
"""

import base64
import logging
from abc import ABC, abstractmethod
from typing import AsyncIterator, Optional

from anthropic import AsyncAnthropic
from openai import AsyncOpenAI
from pydantic import BaseModel
from pydantic_settings import BaseSettings

logger = logging.getLogger(__name__)


class Settings(BaseSettings):
    """Service settings."""

    host: str = "0.0.0.0"
    port: int = 8001

    # LLM Provider: "anthropic", "openai", "glm", "deepseek", "mock"
    llm_provider: str = "mock"

    # Anthropic Claude API
    anthropic_api_key: str = ""
    anthropic_model: str = "claude-sonnet-4-20250514"

    # OpenAI API
    openai_api_key: str = ""
    openai_base_url: str = "https://api.openai.com/v1"
    openai_model: str = "gpt-4o-mini"

    # GLM (智谱 AI) API
    glm_api_key: str = ""
    glm_base_url: str = "https://open.bigmodel.cn/api/paas/v4"
    glm_model: str = "glm-5"

    # DeepSeek API
    deepseek_api_key: str = ""
    deepseek_base_url: str = "https://api.deepseek.com/v1"
    deepseek_model: str = "deepseek-chat"

    # Common settings
    max_tokens: int = 4096
    temperature: float = 0.7

    # Rate limiting
    requests_per_minute: int = 60

    class Config:
        env_file = ".env"


class ChatMessage(BaseModel):
    """A chat message."""

    role: str = "user"
    content: str


class ChatRequest(BaseModel):
    """Request for chat completion."""

    messages: list[ChatMessage]
    model: Optional[str] = None
    system: Optional[str] = None
    max_tokens: Optional[int] = None
    temperature: float = 0.7
    stream: bool = False


class ChatResponse(BaseModel):
    """Response from chat completion."""

    content: str
    model: str
    usage: dict
    stop_reason: str


class RAGRequest(BaseModel):
    """Request for RAG-enhanced query."""

    query: str
    context: str
    system_prompt: Optional[str] = None
    model: Optional[str] = None
    temperature: float = 0.7


class LLMProvider(ABC):
    """Abstract base class for LLM providers."""

    @abstractmethod
    async def complete(self, request: ChatRequest) -> ChatResponse:
        """Generate a chat completion."""
        pass

    @abstractmethod
    async def stream(self, request: ChatRequest) -> AsyncIterator[str]:
        """Stream a chat completion."""
        pass

    @abstractmethod
    async def rag_query(self, request: RAGRequest, system_prompt: str) -> ChatResponse:
        """Execute a RAG-enhanced query."""
        pass

    @abstractmethod
    async def rag_stream(self, request: RAGRequest, system_prompt: str) -> AsyncIterator[str]:
        """Stream a RAG-enhanced response."""
        pass

    @abstractmethod
    async def analyze_image(
        self, image_data: bytes, media_type: str, prompt: str, model: Optional[str] = None
    ) -> ChatResponse:
        """Analyze an image using vision."""
        pass

    @abstractmethod
    async def health_check(self) -> dict:
        """Check provider health."""
        pass


class MockProvider(LLMProvider):
    """Mock provider for testing without real LLM."""

    def __init__(self, response: str = "这是一个模拟回复。"):
        self.default_response = response

    async def complete(self, request: ChatRequest) -> ChatResponse:
        last_message = request.messages[-1].content if request.messages else ""
        return ChatResponse(
            content=f"【Mock回复】你说: {last_message}\n\n{self.default_response}",
            model="mock-model",
            usage={"input_tokens": 10, "output_tokens": 20},
            stop_reason="end_turn",
        )

    async def stream(self, request: ChatRequest) -> AsyncIterator[str]:
        last_message = request.messages[-1].content if request.messages else ""
        response = f"【Mock回复】你说: {last_message}\n\n{self.default_response}"
        for char in response:
            yield char

    async def rag_query(self, request: RAGRequest, system_prompt: str) -> ChatResponse:
        return ChatResponse(
            content=f"【Mock RAG回复】关于 '{request.query}'，根据上下文信息，这是一个模拟的回答。",
            model="mock-model",
            usage={"input_tokens": 20, "output_tokens": 30},
            stop_reason="end_turn",
        )

    async def rag_stream(self, request: RAGRequest, system_prompt: str) -> AsyncIterator[str]:
        response = f"【Mock RAG回复】关于 '{request.query}'，根据上下文信息，这是一个模拟的回答。"
        for char in response:
            yield char

    async def analyze_image(
        self, image_data: bytes, media_type: str, prompt: str, model: Optional[str] = None
    ) -> ChatResponse:
        return ChatResponse(
            content="【Mock Vision】这是一张模拟的图片分析结果。",
            model="mock-vision",
            usage={"input_tokens": 100, "output_tokens": 50},
            stop_reason="end_turn",
        )

    async def health_check(self) -> dict:
        return {"status": "healthy", "provider": "mock"}


class OpenAICompatibleProvider(LLMProvider):
    """
    OpenAI-compatible provider.
    Works with: OpenAI, GLM (智谱), DeepSeek, and any OpenAI-compatible API.
    """

    RAG_SYSTEM_PROMPT = """你是一位专业的旅行导游助手。你的任务是根据提供的目的地知识库信息，为用户提供准确、有用的旅行建议。

规则:
1. 只使用提供的上下文信息回答问题
2. 如果信息不足，坦诚告知用户
3. 保持回答简洁但有价值
4. 适当添加文化背景和有趣的事实
5. 使用友好的语气，像一位本地导游一样交流"""

    def __init__(
        self,
        api_key: str,
        base_url: str,
        default_model: str,
        vision_model: str = None,
        provider_name: str = "openai",
        max_tokens: int = 4096,
    ):
        self.client = AsyncOpenAI(api_key=api_key, base_url=base_url)
        self.default_model = default_model
        self.vision_model = vision_model or default_model
        self.provider_name = provider_name
        self.max_tokens = max_tokens

    async def complete(self, request: ChatRequest) -> ChatResponse:
        model = request.model or self.default_model
        max_tokens = request.max_tokens or self.max_tokens
        messages = []

        if request.system:
            messages.append({"role": "system", "content": request.system})

        for m in request.messages:
            messages.append({"role": m.role, "content": m.content})

        response = await self.client.chat.completions.create(
            model=model,
            messages=messages,
            max_tokens=max_tokens,
            temperature=request.temperature,
        )

        choice = response.choices[0]
        return ChatResponse(
            content=choice.message.content,
            model=response.model,
            usage={
                "input_tokens": response.usage.prompt_tokens,
                "output_tokens": response.usage.completion_tokens,
            },
            stop_reason=choice.finish_reason,
        )

    async def stream(self, request: ChatRequest) -> AsyncIterator[str]:
        model = request.model or self.default_model
        max_tokens = request.max_tokens or self.max_tokens
        messages = []

        if request.system:
            messages.append({"role": "system", "content": request.system})

        for m in request.messages:
            messages.append({"role": m.role, "content": m.content})

        stream = await self.client.chat.completions.create(
            model=model,
            messages=messages,
            max_tokens=max_tokens,
            temperature=request.temperature,
            stream=True,
        )

        async for chunk in stream:
            if chunk.choices[0].delta.content:
                yield chunk.choices[0].delta.content

    async def rag_query(self, request: RAGRequest, system_prompt: str) -> ChatResponse:
        model = request.model or self.default_model
        user_message = f"""用户问题: {request.query}

参考信息:
{request.context}

请根据以上信息回答用户的问题。如果参考信息中没有相关内容，请坦诚告知。"""

        response = await self.client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_message},
            ],
            max_tokens=self.max_tokens,
            temperature=request.temperature,
        )

        choice = response.choices[0]
        return ChatResponse(
            content=choice.message.content,
            model=response.model,
            usage={
                "input_tokens": response.usage.prompt_tokens,
                "output_tokens": response.usage.completion_tokens,
            },
            stop_reason=choice.finish_reason,
        )

    async def rag_stream(self, request: RAGRequest, system_prompt: str) -> AsyncIterator[str]:
        model = request.model or self.default_model
        user_message = f"""用户问题: {request.query}

参考信息:
{request.context}

请根据以上信息回答用户的问题。"""

        stream = await self.client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_message},
            ],
            max_tokens=self.max_tokens,
            temperature=request.temperature,
            stream=True,
        )

        async for chunk in stream:
            if chunk.choices[0].delta.content:
                yield chunk.choices[0].delta.content

    async def analyze_image(
        self, image_data: bytes, media_type: str, prompt: str, model: Optional[str] = None
    ) -> ChatResponse:
        """Analyze an image using vision model."""
        model = model or self.vision_model
        image_base64 = base64.b64encode(image_data).decode("utf-8")
        image_url = f"data:{media_type};base64,{image_base64}"

        response = await self.client.chat.completions.create(
            model=model,
            messages=[
                {
                    "role": "user",
                    "content": [
                        {"type": "image_url", "image_url": {"url": image_url}},
                        {"type": "text", "text": prompt},
                    ],
                }
            ],
            max_tokens=1024,
        )

        choice = response.choices[0]
        return ChatResponse(
            content=choice.message.content,
            model=response.model,
            usage={
                "input_tokens": response.usage.prompt_tokens,
                "output_tokens": response.usage.completion_tokens,
            },
            stop_reason=choice.finish_reason,
        )

    async def health_check(self) -> dict:
        return {
            "status": "healthy",
            "provider": self.provider_name,
            "model": self.default_model,
        }


class AnthropicProvider(LLMProvider):
    """Anthropic Claude provider."""

    def __init__(self, api_key: str, default_model: str, max_tokens: int = 4096):
        self.client = AsyncAnthropic(api_key=api_key)
        self.default_model = default_model
        self.max_tokens = max_tokens

    async def complete(self, request: ChatRequest) -> ChatResponse:
        model = request.model or self.default_model
        max_tokens = request.max_tokens or self.max_tokens
        messages = [{"role": m.role, "content": m.content} for m in request.messages]

        kwargs = {"model": model, "max_tokens": max_tokens, "messages": messages}
        if request.system:
            kwargs["system"] = request.system

        response = await self.client.messages.create(**kwargs)

        return ChatResponse(
            content=response.content[0].text,
            model=response.model,
            usage={
                "input_tokens": response.usage.input_tokens,
                "output_tokens": response.usage.output_tokens,
            },
            stop_reason=response.stop_reason,
        )

    async def stream(self, request: ChatRequest) -> AsyncIterator[str]:
        model = request.model or self.default_model
        max_tokens = request.max_tokens or self.max_tokens
        messages = [{"role": m.role, "content": m.content} for m in request.messages]

        kwargs = {"model": model, "max_tokens": max_tokens, "messages": messages}
        if request.system:
            kwargs["system"] = request.system

        async with self.client.messages.stream(**kwargs) as stream:
            async for text in stream.text_stream:
                yield text

    async def rag_query(self, request: RAGRequest, system_prompt: str) -> ChatResponse:
        model = request.model or self.default_model
        user_message = f"""用户问题: {request.query}

参考信息:
{request.context}

请根据以上信息回答用户的问题。如果参考信息中没有相关内容，请坦诚告知。"""

        response = await self.client.messages.create(
            model=model,
            max_tokens=self.max_tokens,
            system=system_prompt,
            messages=[{"role": "user", "content": user_message}],
            temperature=request.temperature,
        )

        return ChatResponse(
            content=response.content[0].text,
            model=response.model,
            usage={
                "input_tokens": response.usage.input_tokens,
                "output_tokens": response.usage.output_tokens,
            },
            stop_reason=response.stop_reason,
        )

    async def rag_stream(self, request: RAGRequest, system_prompt: str) -> AsyncIterator[str]:
        model = request.model or self.default_model
        user_message = f"""用户问题: {request.query}

参考信息:
{request.context}

请根据以上信息回答用户的问题。"""

        async with self.client.messages.stream(
            model=model,
            max_tokens=self.max_tokens,
            system=system_prompt,
            messages=[{"role": "user", "content": user_message}],
            temperature=request.temperature,
        ) as stream:
            async for text in stream.text_stream:
                yield text

    async def analyze_image(
        self, image_data: bytes, media_type: str, prompt: str, model: Optional[str] = None
    ) -> ChatResponse:
        model = model or self.default_model
        image_base64 = base64.b64encode(image_data).decode("utf-8")

        response = await self.client.messages.create(
            model=model,
            max_tokens=1024,
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
                        {"type": "text", "text": prompt},
                    ],
                }
            ],
        )

        return ChatResponse(
            content=response.content[0].text,
            model=response.model,
            usage={
                "input_tokens": response.usage.input_tokens,
                "output_tokens": response.usage.output_tokens,
            },
            stop_reason=response.stop_reason,
        )

    async def health_check(self) -> dict:
        return {
            "status": "healthy",
            "provider": "anthropic",
            "model": self.default_model,
        }


class LLMGateway:
    """
    Gateway for LLM operations.

    Supports multiple providers:
    - anthropic: Claude (Anthropic)
    - openai: GPT models
    - glm: 智谱 AI
    - deepseek: DeepSeek
    - mock: Testing without real LLM
    """

    RAG_SYSTEM_PROMPT = """你是一位专业的旅行导游助手。你的任务是根据提供的目的地知识库信息，为用户提供准确、有用的旅行建议。

规则:
1. 只使用提供的上下文信息回答问题
2. 如果信息不足，坦诚告知用户
3. 保持回答简洁但有价值
4. 适当添加文化背景和有趣的事实
5. 使用友好的语气，像一位本地导游一样交流"""

    def __init__(self, settings: Settings):
        self.settings = settings
        self._provider: Optional[LLMProvider] = None

    @property
    def provider(self) -> LLMProvider:
        """Lazy load LLM provider based on settings."""
        if self._provider is None:
            provider_type = self.settings.llm_provider.lower()

            if provider_type == "openai":
                if not self.settings.openai_api_key:
                    raise ValueError("OPENAI_API_KEY is required for OpenAI provider")
                logger.info(f"Using OpenAI provider with model {self.settings.openai_model}")
                self._provider = OpenAICompatibleProvider(
                    api_key=self.settings.openai_api_key,
                    base_url=self.settings.openai_base_url,
                    default_model=self.settings.openai_model,
                    vision_model="gpt-4o",  # GPT-4o for vision
                    provider_name="openai",
                    max_tokens=self.settings.max_tokens,
                )
            elif provider_type == "glm":
                if not self.settings.glm_api_key:
                    raise ValueError("GLM_API_KEY is required for GLM provider")
                logger.info(f"Using GLM provider with model {self.settings.glm_model}")
                self._provider = OpenAICompatibleProvider(
                    api_key=self.settings.glm_api_key,
                    base_url=self.settings.glm_base_url,
                    default_model=self.settings.glm_model,
                    vision_model="glm-4v-flash",  # GLM vision model
                    provider_name="glm",
                    max_tokens=self.settings.max_tokens,
                )
            elif provider_type == "deepseek":
                if not self.settings.deepseek_api_key:
                    raise ValueError("DEEPSEEK_API_KEY is required for DeepSeek provider")
                logger.info(f"Using DeepSeek provider with model {self.settings.deepseek_model}")
                self._provider = OpenAICompatibleProvider(
                    api_key=self.settings.deepseek_api_key,
                    base_url=self.settings.deepseek_base_url,
                    default_model=self.settings.deepseek_model,
                    provider_name="deepseek",
                    max_tokens=self.settings.max_tokens,
                )
            elif provider_type == "anthropic":
                if not self.settings.anthropic_api_key:
                    raise ValueError("ANTHROPIC_API_KEY is required for Anthropic provider")
                logger.info(f"Using Anthropic provider with model {self.settings.anthropic_model}")
                self._provider = AnthropicProvider(
                    api_key=self.settings.anthropic_api_key,
                    default_model=self.settings.anthropic_model,
                    max_tokens=self.settings.max_tokens,
                )
            else:
                logger.info("Using Mock provider")
                self._provider = MockProvider()

        return self._provider

    async def complete(self, request: ChatRequest) -> ChatResponse:
        """Generate a chat completion."""
        return await self.provider.complete(request)

    async def stream(self, request: ChatRequest) -> AsyncIterator[str]:
        """Stream a chat completion."""
        async for chunk in self.provider.stream(request):
            yield chunk

    async def rag_query(self, request: RAGRequest) -> ChatResponse:
        """Execute a RAG-enhanced query."""
        system_prompt = request.system_prompt or self.RAG_SYSTEM_PROMPT
        return await self.provider.rag_query(request, system_prompt)

    async def rag_stream(self, request: RAGRequest) -> AsyncIterator[str]:
        """Stream a RAG-enhanced response."""
        system_prompt = request.system_prompt or self.RAG_SYSTEM_PROMPT
        async for chunk in self.provider.rag_stream(request, system_prompt):
            yield chunk

    async def analyze_image(
        self,
        image_data: bytes,
        media_type: str,
        prompt: str,
        model: Optional[str] = None,
    ) -> ChatResponse:
        """Analyze an image using vision."""
        return await self.provider.analyze_image(image_data, media_type, prompt, model)

    async def health_check(self) -> dict:
        """Check service health."""
        result = await self.provider.health_check()
        result["api_key_configured"] = self._check_api_key()
        return result

    def _check_api_key(self) -> bool:
        """Check if API key is configured for the current provider."""
        provider_type = self.settings.llm_provider.lower()
        return (
            (provider_type == "openai" and bool(self.settings.openai_api_key))
            or (provider_type == "glm" and bool(self.settings.glm_api_key))
            or (provider_type == "deepseek" and bool(self.settings.deepseek_api_key))
            or (provider_type == "anthropic" and bool(self.settings.anthropic_api_key))
            or provider_type == "mock"
        )


# Singleton instance
_gateway: Optional[LLMGateway] = None


def get_gateway() -> LLMGateway:
    """Get the LLM gateway singleton."""
    global _gateway
    if _gateway is None:
        _gateway = LLMGateway(Settings())
    return _gateway