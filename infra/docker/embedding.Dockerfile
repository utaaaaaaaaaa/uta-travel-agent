# Python Embedding Service Dockerfile
FROM python:3.11-slim AS builder

WORKDIR /app

RUN pip install --no-cache-dir uv

COPY services/embedding/pyproject.toml ./

RUN uv pip install --system --no-cache-dir .

FROM python:3.11-slim

WORKDIR /app

COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

COPY services/embedding/src ./src
COPY services/gen/python ./gen

ENV PYTHONPATH=/app:/app/gen
ENV PYTHONUNBUFFERED=1

EXPOSE 50052

CMD ["python", "-m", "src.grpc_service"]