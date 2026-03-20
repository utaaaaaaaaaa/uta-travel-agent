#!/bin/bash

# UTA Travel Agent - Development Environment Setup
# This script starts all required services for local development

set -e

echo "🚀 Starting UTA Travel Agent Development Environment..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check for .env file
if [ ! -f .env ]; then
    echo -e "${YELLOW}No .env file found, creating from .env.example${NC}"
    cp .env.example .env
    echo -e "${YELLOW}Please edit .env and add your API keys${NC}"
fi

# Load environment variables
source .env 2>/dev/null || true

# Start infrastructure services
echo -e "${GREEN}Starting infrastructure services (PostgreSQL, Qdrant, Redis)...${NC}"
docker-compose -f docker-compose.dev.yml up -d

# Wait for services to be healthy
echo -e "${GREEN}Waiting for services to be ready...${NC}"
sleep 5

# Check PostgreSQL
until docker exec uta-postgres pg_isready -U postgres; do
    echo -e "${YELLOW}Waiting for PostgreSQL...${NC}"
    sleep 2
done
echo -e "${GREEN}✓ PostgreSQL is ready${NC}"

# Check Qdrant
until curl -s http://localhost:6333/ > /dev/null; do
    echo -e "${YELLOW}Waiting for Qdrant...${NC}"
    sleep 2
done
echo -e "${GREEN}✓ Qdrant is ready${NC}"

# Check Redis
until docker exec uta-redis redis-cli ping | grep -q PONG; do
    echo -e "${YELLOW}Waiting for Redis...${NC}"
    sleep 2
done
echo -e "${GREEN}✓ Redis is ready${NC}"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Infrastructure services are running!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Services:"
echo "  - PostgreSQL: localhost:5432"
echo "  - Qdrant:    localhost:6333 (REST), localhost:6334 (gRPC)"
echo "  - Redis:     localhost:6379"
echo ""
echo "To start the application services:"
echo ""
echo "  1. Start Python Embedding Service:"
echo "     cd services/embedding && source .venv/bin/activate && python main.py"
echo ""
echo "  2. Start Go Orchestrator:"
echo "     cd cmd/orchestrator && go run ."
echo ""
echo "  3. Start Next.js Frontend:"
echo "     cd apps/web && npm run dev"
echo ""
echo "To stop infrastructure:"
echo "  docker-compose -f docker-compose.dev.yml down"
echo ""
