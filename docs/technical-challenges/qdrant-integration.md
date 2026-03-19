# Qdrant Vector Database Integration

## Overview

Qdrant is a high-performance vector database used in UTA Travel Agent for RAG (Retrieval-Augmented Generation) functionality. This document describes the integration architecture, API design, and implementation details.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      API Gateway (Go)                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  MainAgent ──────► Indexer Agent ──────► Qdrant Client      │
│                              │                               │
│  Guide Agent  ──────► RAG Query    ──────► Qdrant Client    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │     Qdrant      │
                    │   (gRPC:6334)   │
                    │   (REST:6333)   │
                    └─────────────────┘
```

## Client Implementation

### Configuration

```go
type Config struct {
    Host string  // Default: "localhost"
    Port int     // Default: 6334 (gRPC port)
}
```

### Core Methods

| Method | Description |
|--------|-------------|
| `CreateCollection` | Create a new vector collection |
| `DeleteCollection` | Delete a collection |
| `ListCollections` | List all collections |
| `CollectionInfo` | Get collection metadata |
| `Upsert` | Insert or update vectors |
| `DeletePoints` | Delete points by ID |
| `Search` | Vector similarity search |
| `SearchWithFilter` | Search with metadata filter |
| `Scroll` | Paginate through points |

### Usage Examples

#### Creating a Collection

```go
client, _ := qdrant.NewClient(qdrant.Config{
    Host: "localhost",
    Port: 6334,
})

err := client.CreateCollection(ctx, qdrant.CollectionConfig{
    Name:       "kyoto-attractions",
    VectorSize: 384,        // Match embedding model
    Distance:   "Cosine",   // Cosine similarity
    OnDisk:     false,      // In-memory for speed
})
```

#### Upserting Vectors

```go
points := []qdrant.Point{
    {
        ID:     "attraction-001",
        Vector: embedding,  // []float32 from embedding model
        Payload: map[string]interface{}{
            "name":     "Kinkaku-ji",
            "type":     "temple",
            "location": "Kyoto, Japan",
        },
    },
}

err := client.Upsert(ctx, "kyoto-attractions", points)
```

#### Searching Vectors

```go
queryVector := generateEmbedding("golden temple in kyoto")

results, err := client.Search(ctx, "kyoto-attractions", queryVector, 5)

for _, r := range results {
    fmt.Printf("Score: %.4f, Name: %s\n",
        r.Score,
        r.Payload["name"],
    )
}
```

## ID Handling

Qdrant requires valid UUID format for point IDs. The client automatically converts any string ID to a deterministic UUID using MD5 hash:

```go
// Input: "attraction-001"
// Output: "c5a8e1b2-4f3d-7a9c-2e1b-8f4d6a9c2e1b"
```

The original ID is stored in the `_id` payload field for reference.

## Distance Metrics

| Metric | Use Case |
|--------|----------|
| Cosine | General purpose, normalized vectors |
| Euclid | Geographic distances, absolute differences |
| Dot | When magnitude matters |

## Performance Considerations

### Indexing

- **HNSW**: Default, fast search, more memory
- **Flat**: Exact search, slower but less memory

### Batch Operations

For large datasets, use batch upsert:

```go
batchSize := 100
for i := 0; i < len(points); i += batchSize {
    end := i + batchSize
    if end > len(points) {
        end = len(points)
    }
    client.Upsert(ctx, collection, points[i:end])
}
```

## Docker Configuration

```yaml
# docker-compose.dev.yml
qdrant:
  image: qdrant/qdrant:latest
  ports:
    - "6333:6333"  # REST API
    - "6334:6334"  # gRPC API
  volumes:
    - qdrant_data:/qdrant/storage
```

## Error Handling

```go
result, err := client.Search(ctx, collection, vector, limit)
if err != nil {
    // Check for specific errors
    if strings.Contains(err.Error(), "not found") {
        // Collection doesn't exist
    }
    return err
}
```

## Testing

Run integration tests:

```bash
# Start Qdrant first
docker compose -f infra/docker/docker-compose.dev.yml up -d qdrant

# Run tests
go test -v ./internal/storage/qdrant/...
```

## Future Enhancements

1. **Filter Support**: Add metadata filtering for RAG queries
2. **Batch Operations**: Optimize large dataset indexing
3. **Caching**: Add Redis caching for frequent queries
4. **Quantization**: Enable vector quantization for memory efficiency

## References

- [Qdrant Documentation](https://qdrant.tech/documentation/)
- [Qdrant Go Client](https://github.com/qdrant/go-client)