# Python LLM Gateway Dockerfile
FROM python:3.11-slim AS builder

WORKDIR /app

# Install build dependencies
RUN pip install --no-cache-dir uv

# Copy project files
COPY services/llm-gateway/pyproject.toml ./

# Install dependencies
RUN uv pip install --system --no-cache-dir .

# Final stage
FROM python:3.11-slim

WORKDIR /app

# Copy installed packages
COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Copy source code
COPY services/llm-gateway/src ./src
COPY services/gen/python ./gen

# Set Python path
ENV PYTHONPATH=/app:/app/gen
ENV PYTHONUNBUFFERED=1

# Expose gRPC port
EXPOSE 50051

# Run gRPC server
CMD ["python", "-m", "src.grpc_service"]