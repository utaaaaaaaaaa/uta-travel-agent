# UTA Embedding Service - gRPC Server

# Build stage
FROM python:3.11-slim AS builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements and install dependencies
COPY services/embedding/requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

# Runtime stage
FROM python:3.11-slim

WORKDIR /app

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy installed packages from builder
COPY --from=builder /root/.local /root/.local
ENV PATH=/root/.local/bin:$PATH

# Copy generated protobuf code
COPY services/gen/python/agent /app/agent

# Copy application code
COPY services/embedding/src /app/src
COPY services/embedding/main.py /app/main.py

# Set PYTHONPATH
ENV PYTHONPATH=/app:/app/src

# Create models directory for caching
RUN mkdir -p /app/models

# Expose gRPC port
EXPOSE 50052

# Environment variables
ENV GRPC_PORT=50052
ENV EMBEDDING_MODEL=sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2

# Health check
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD curl -f http://localhost:50052/health || exit 1

# Run the gRPC service
CMD ["python", "main.py"]