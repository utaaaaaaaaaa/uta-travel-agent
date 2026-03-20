# UTA Travel Agent - Makefile

.PHONY: all dev infra up down build test clean

# Default target
all: dev

# Start infrastructure services (PostgreSQL, Qdrant, Redis)
infra:
	@echo "Starting infrastructure services..."
	docker-compose -f docker-compose.dev.yml up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@docker exec uta-postgres pg_isready -U postgres || (echo "PostgreSQL not ready" && exit 1)
	@curl -s http://localhost:6333/ > /dev/null || (echo "Qdrant not ready" && exit 1)
	@echo "Infrastructure services are ready!"

# Stop all services
down:
	@echo "Stopping services..."
	docker-compose -f docker-compose.dev.yml down
	docker-compose down 2>/dev/null || true

# Start embedding service (Python)
embedding:
	@echo "Starting embedding service..."
	cd services/embedding && \
	if [ ! -d ".venv" ]; then \
		python3 -m venv .venv; \
		. .venv/bin/activate && pip install -r requirements.txt; \
	fi && \
	. .venv/bin/activate && PYTHONPATH=/app:/app/src python main.py

# Start Go orchestrator
orchestrator:
	@echo "Starting Go orchestrator..."
	cd cmd/orchestrator && GO111MODULE=on go run .

# Start frontend
web:
	@echo "Starting Next.js frontend..."
	cd apps/web && npm run dev

# Build all services
build:
	@echo "Building all services..."
	GO111MODULE=on go build ./...
	cd apps/web && npm run build

# Run tests
test:
	@echo "Running tests..."
	GO111MODULE=on go test ./... -v -count=1

# Clean up
clean:
	@echo "Cleaning up..."
	rm -rf apps/web/.next apps/web/node_modules
	rm -rf services/embedding/.venv services/embedding/__pycache__
	rm -rf services/embedding/src/__pycache__
	docker-compose -f docker-compose.dev.yml down -v
	docker-compose down -v 2>/dev/null || true

# Full development setup
dev: infra
	@echo ""
	@echo "Infrastructure is running. Now start the services:"
	@echo ""
	@echo "  Terminal 1: make orchestrator"
	@echo "  Terminal 2: make web"
	@echo ""
	@echo "Optional: make embedding (if using embedding service)"

# Quick start with mock LLM
quick-start:
	@echo "Quick starting with mock LLM..."
	$(MAKE) infra
	@echo ""
	@echo "Starting orchestrator with mock LLM in background..."
	@cd cmd/orchestrator && LLM_PROVIDER=mock GO111MODULE=on go run . &
	@sleep 3
	@echo "Starting frontend..."
	@cd apps/web && npm run dev

# Show status
status:
	@echo "Service Status:"
	@echo "==============="
	@docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "Docker not running"
	@echo ""
	@echo "Endpoints:"
	@echo "  - Frontend:    http://localhost:3000"
	@echo "  - API:         http://localhost:8080"
	@echo "  - Qdrant:      http://localhost:6333"
	@echo "  - PostgreSQL:  localhost:5432"
