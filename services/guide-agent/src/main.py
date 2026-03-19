"""
FastAPI main application for Guide Agent service.
"""

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

from agent import GuideService, Location, Settings

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Global service instance
service: GuideService | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    global service
    settings = Settings()
    service = GuideService(settings)
    logger.info("Guide Agent service started")
    yield
    logger.info("Guide Agent service stopped")


app = FastAPI(
    title="Guide Agent Service",
    description="Real-time tour guide with vision and narration",
    version="0.1.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# Request/Response models
class StartSessionRequest(BaseModel):
    agent_id: str
    user_id: str
    location: Location | None = None


class UpdateLocationRequest(BaseModel):
    latitude: float
    longitude: float
    accuracy: float | None = None


class QuestionRequest(BaseModel):
    question: str
    context: str | None = None


# REST endpoints
@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy", "service": "guide-agent"}


@app.post("/sessions")
async def start_session(request: StartSessionRequest):
    """Start a new guide session."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    session = await service.start_session(
        agent_id=request.agent_id,
        user_id=request.user_id,
        initial_location=request.location,
    )

    return {
        "session_id": session.id,
        "agent_id": session.agent_id,
        "status": session.status,
    }


@app.post("/sessions/{session_id}/location")
async def update_location(session_id: str, request: UpdateLocationRequest):
    """Update session location and get nearby landmarks."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    location = Location(
        latitude=request.latitude,
        longitude=request.longitude,
        accuracy=request.accuracy,
    )

    landmarks = await service.update_location(session_id, location)

    return {
        "session_id": session_id,
        "landmarks": [
            {
                "id": lm.id,
                "name": lm.name,
                "description": lm.description,
                "confidence": lm.confidence,
            }
            for lm in landmarks
        ],
    }


@app.post("/sessions/{session_id}/question")
async def ask_question(session_id: str, request: QuestionRequest):
    """Ask a question during the tour."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    answer = await service.answer_question(
        session_id=session_id,
        question=request.question,
        context=request.context,
    )

    return {"answer": answer}


@app.delete("/sessions/{session_id}")
async def end_session(session_id: str):
    """End a guide session."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    ended = await service.end_session(session_id)
    if not ended:
        raise HTTPException(status_code=404, detail="Session not found")

    return {"status": "ended"}


# WebSocket for real-time communication
@app.websocket("/ws/{session_id}")
async def websocket_endpoint(websocket: WebSocket, session_id: str):
    """WebSocket endpoint for real-time guide communication."""
    await websocket.accept()
    logger.info(f"WebSocket connected for session {session_id}")

    try:
        while True:
            data = await websocket.receive_json()

            # Handle different message types
            message_type = data.get("type")

            if message_type == "location":
                # Location update
                location = Location(
                    latitude=data["latitude"],
                    longitude=data["longitude"],
                )
                landmarks = await service.update_location(session_id, location)
                await websocket.send_json({
                    "type": "landmarks",
                    "data": [{"name": lm.name, "description": lm.description} for lm in landmarks],
                })

            elif message_type == "question":
                # User question
                answer = await service.answer_question(
                    session_id=session_id,
                    question=data["question"],
                )
                await websocket.send_json({
                    "type": "answer",
                    "data": answer,
                })

            elif message_type == "ping":
                await websocket.send_json({"type": "pong"})

    except WebSocketDisconnect:
        logger.info(f"WebSocket disconnected for session {session_id}")
    except Exception as e:
        logger.error(f"WebSocket error: {e}")
        await websocket.close()


if __name__ == "__main__":
    import uvicorn

    settings = Settings()
    uvicorn.run(app, host=settings.host, port=settings.port)