# Embedding Service

## Overview

The Embedding Service provides text embedding capabilities for the UTA Travel Agent system. It transforms text into vector representations that can be used for semantic search, similarity comparison, and RAG (Retrieval-Augmented Generation).

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Embedding Service                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────────┐    ┌─────────────────────────────┐     │
│  │   FastAPI REST  │    │      gRPC Service           │     │
│  │   Port: 8002    │    │      Port: 50052            │     │
│  └────────┬────────┘    └─────────────┬───────────────┘     │
│           │                           │                      │
│           └───────────┬───────────────┘                      │
│                       ▼                                      │
│           ┌─────────────────────────┐                       │
│           │   EmbeddingService      │                       │
│           │   - Model Management    │                       │
│           │   - Caching Layer       │                       │
│           └────────────┬────────────┘                       │
│                        ▼                                     │
│           ┌─────────────────────────┐                       │
│           │ SentenceTransformer     │                       │
│           │ (HuggingFace Models)    │                       │
│           └─────────────────────────┘                       │
└─────────────────────────────────────────────────────────────┘
```

## Model Presets

The service supports several pre-configured model presets optimized for different use cases:

| Preset | Model | Dimensions | Description |
|--------|-------|------------|-------------|
| `multilingual` | paraphrase-multilingual-MiniLM-L12-v2 | 384 | 50+ languages, balanced performance |
| `english` | all-MiniLM-L6-v2 | 384 | Fast, English-only |
| `quality` | all-mpnet-base-v2 | 768 | Best quality, slower |
| `asian` | distiluse-base-multilingual-cased-v1 | 768 | Better for Asian languages |

## API Endpoints

### REST API

#### POST /v1/embed

Create embeddings for texts with optional caching.

```json
// Request
{
  "texts": ["Hello world", "Travel to Japan"],
  "use_cache": true
}

// Response
{
  "embeddings": [[0.1, 0.2, ...], [0.3, 0.4, ...]],
  "model": "all-MiniLM-L6-v2",
  "dimension": 384,
  "cached_count": 0
}
```

#### POST /v1/embed/batch

Create embeddings for a large batch of texts (always uses cache).

```json
// Request
{
  "texts": ["Text 1", "Text 2", ...],
  "batch_size": 32
}

// Response
{
  "embeddings": [[...], [...], ...],
  "model": "all-MiniLM-L6-v2",
  "dimension": 384,
  "cached_count": 5
}
```

#### GET /v1/model/info

Get current model information.

```json
// Response
{
  "model": "all-MiniLM-L6-v2",
  "dimension": 384
}
```

#### POST /v1/cache/clear

Clear the embedding cache.

```json
// Response
{
  "cleared": 150
}
```

#### GET /health

Health check endpoint.

```json
// Response
{
  "status": "healthy",
  "model": "all-MiniLM-L6-v2",
  "dimension": 384,
  "cache_size": 25
}
```

### gRPC API

The service exposes the following gRPC methods on port 50052:

```protobuf
service EmbeddingService {
    rpc Embed(EmbedRequest) returns (EmbedResponse);
    rpc BatchEmbed(BatchEmbedRequest) returns (EmbedResponse);
    rpc GetModelInfo(ModelInfoRequest) returns (ModelInfoResponse);
    rpc ClearCache(ClearCacheRequest) returns (ClearCacheResponse);
    rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HOST` | `0.0.0.0` | Server host |
| `PORT` | `8002` | REST API port |
| `MODEL_NAME` | `multilingual` | Model preset or name |
| `DEVICE` | auto-detect | Device for inference (cpu/cuda) |
| `BATCH_SIZE` | `32` | Batch size for embedding |
| `CACHE_SIZE` | `10000` | Maximum cache entries |

## Caching Mechanism

The service implements an in-memory LRU cache for embeddings:

- **Cache Key**: MD5 hash of the text
- **Eviction**: FIFO when cache is full
- **Benefit**: Avoids recomputing embeddings for repeated texts

```python
# Example: First call computes, second uses cache
await service.embed(["Hello"])  # Computes embedding
await service.embed(["Hello"])  # Returns cached embedding
```

## Go Client Usage

```go
import "github.com/utaaa/uta-travel-agent/internal/grpc/clients"

// Create client
conn, _ := grpc.Dial("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
client := clients.NewEmbeddingClient(conn)

// Create embeddings
resp, err := client.Embed(ctx, clients.EmbedRequest{
    Texts:    []string{"Hello", "World"},
    UseCache: true,
})

// Access embeddings
for i, emb := range resp.Embeddings {
    fmt.Printf("Text %d: %d dimensions\n", i, len(emb))
}
```

## Docker Configuration

```yaml
# docker-compose.yml
embedding:
  build:
    context: .
    dockerfile: infra/docker/embedding.Dockerfile
  ports:
    - "50052:50052"
  environment:
    - GRPC_PORT=50052
```

## Performance Considerations

1. **Batch Processing**: Use batch endpoints for multiple texts
2. **Caching**: Enable caching for repeated texts
3. **Model Selection**: Use smaller models for faster inference
4. **Device**: Use GPU (cuda) for better performance with large batches

## Error Handling

```go
resp, err := client.Embed(ctx, req)
if err != nil {
    if strings.Contains(err.Error(), "INTERNAL") {
        // Service error
    }
    return err
}
```

## Testing

Run tests:

```bash
cd services/embedding
pytest tests/ -v
```

## References

- [Sentence Transformers Documentation](https://www.sbert.net/)
- [HuggingFace Models](https://huggingface.co/models?library=sentence-transformers)