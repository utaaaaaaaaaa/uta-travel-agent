# Proto Generation

This directory contains Protocol Buffer definitions for the UTA Travel Agent project.

## Structure

```
proto/
├── buf.yaml           # Buf configuration
├── buf.gen.yaml       # Code generation config
└── agent/
    ├── common.proto       # Common types
    ├── destination.proto  # Destination Agent service (Go)
    ├── guide.proto        # Guide Agent service (Go)
    ├── llm.proto          # LLM Gateway service (Python)
    ├── embedding.proto    # Embedding service (Python)
    └── vision.proto       # Vision service (Python)
```

## Services

| Service | Language | Port | Description |
|---------|----------|------|-------------|
| DestinationAgentService | Go | 50051 | Destination agent management |
| GuideService | Go | 50052 | Real-time guide operations |
| LLMService | Python | 8001 | LLM completions & RAG |
| EmbeddingService | Python | 8002 | Text embeddings |
| VisionService | Python | 8003 | Image analysis |

## Prerequisites

- [Buf](https://buf.build/docs/installation)
- Go protoc plugins:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```
- Python packages:
  ```bash
  pip install grpcio grpcio-tools
  ```

## Generate Code

```bash
# Generate all
buf generate

# Generate for specific paths
buf generate --path agent/llm.proto

# Lint check
buf lint

# Format
buf format -w
```

## Generated Code

Generated code will be placed in:
- Go: `internal/gen/go/`
- Python: `services/gen/python/`