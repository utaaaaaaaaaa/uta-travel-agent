"""
FastAPI main application for Planner Agent service.
"""

import logging
from contextlib import asynccontextmanager
from datetime import date

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

from agent import Itinerary, PlannerService, TravelPreferences, Settings

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Global service instance
service: PlannerService | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    global service
    settings = Settings()
    service = PlannerService(settings)
    logger.info("Planner Agent service started")
    yield
    logger.info("Planner Agent service stopped")


app = FastAPI(
    title="Planner Agent Service",
    description="Trip planning and itinerary generation",
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
class CreateItineraryRequest(BaseModel):
    user_id: str
    agent_id: str
    destination: str
    start_date: date
    end_date: date
    preferences: TravelPreferences


# Endpoints
@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy", "service": "planner-agent"}


@app.post("/itineraries")
async def create_itinerary(request: CreateItineraryRequest):
    """Create a new travel itinerary."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    itinerary = await service.create_itinerary(
        user_id=request.user_id,
        agent_id=request.agent_id,
        destination=request.destination,
        start_date=request.start_date,
        end_date=request.end_date,
        preferences=request.preferences,
    )

    return {
        "id": itinerary.id,
        "destination": itinerary.destination,
        "status": itinerary.status,
        "days_count": len(itinerary.days),
    }


@app.get("/itineraries/{itinerary_id}")
async def get_itinerary(itinerary_id: str):
    """Get an itinerary by ID."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    itinerary = await service.get_itinerary(itinerary_id)
    if not itinerary:
        raise HTTPException(status_code=404, detail="Itinerary not found")

    return itinerary.model_dump()


@app.post("/itineraries/{itinerary_id}/optimize/{day_index}")
async def optimize_day(itinerary_id: str, day_index: int):
    """Optimize the route for a specific day."""
    if not service:
        raise HTTPException(status_code=503, detail="Service not initialized")

    activities = await service.optimize_route(itinerary_id, day_index)

    return {
        "itinerary_id": itinerary_id,
        "day_index": day_index,
        "activities": [a.model_dump() for a in activities],
    }


if __name__ == "__main__":
    import uvicorn

    settings = Settings()
    uvicorn.run(app, host=settings.host, port=settings.port)