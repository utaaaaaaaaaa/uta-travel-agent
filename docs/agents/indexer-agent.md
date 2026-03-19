# Indexer Agent

## Overview

The Indexer Agent is responsible for building vector indexes from structured travel information. It takes the curated knowledge base and creates embeddings for efficient RAG (Retrieval-Augmented Generation) queries.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Indexer Agent                           │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Memory     │  │    State     │  │    Tools     │       │
│  │   Manager    │  │   Machine    │  │   Registry   │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
├─────────────────────────────────────────────────────────────┤
│                     Core Functions                           │
│  • Document embedding                                        │
│  • Vector index creation                                     │
│  • Qdrant collection management                              │
│  • Index optimization                                        │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

The Indexer Agent uses a YAML template for configuration:

```yaml
# configs/agents/indexer.yaml
id: indexer
name: "索引构建员"
type: indexer
description: "构建向量索引用于RAG检索"
version: "1.0.0"

tools:
  - build_knowledge_index

capabilities:
  - embedding
  - vector_indexing
  - collection_management

settings:
  max_retries: 3
  timeout: 300s  # Indexing may take longer
  batch_size: 100
```

## Execution Flow

```
Start → Thinking → Running (build_knowledge_index) → Completed
                    ↓
                  Error (if skill fails)
```

### State Transitions

| From | To | Trigger |
|------|-----|---------|
| Idle | Thinking | Run() called |
| Thinking | Running | After adding thought to memory |
| Running | Completed | Index building success |
| Running | Error | Index building failure |
| Completed/Error | Idle | Deferred cleanup |

## API

### Creation

```go
func NewIndexerAgent(id string, template *AgentTemplate) *IndexerAgent
```

### Execution

```go
func (a *IndexerAgent) Run(ctx context.Context, goal string) (*AgentResult, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeout
- `goal`: The indexing goal/objective (typically includes collection ID)

**Returns:**
- `*AgentResult`: Contains indexing metadata (collection ID, document count, etc.)
- `error`: Any execution error

### Lifecycle

```go
func (a *IndexerAgent) Stop() error
```

## Memory Management

The Indexer Agent tracks its operations in memory:

| Type | Content | Source/Metadata |
|------|---------|-----------------|
| thought | 索引目标: {goal} | - |
| result | 向量索引构建完成 | success: true |

### Memory Example

```go
// Thought recorded at start
Memory.AddThought("索引目标: 为京都旅游知识库构建向量索引")

// Result recorded on completion
Memory.AddResult("向量索引构建完成", true, nil)
```

## Tool Dependencies

| Tool | Type | Purpose |
|------|------|---------|
| build_knowledge_index | Skill | Create vector embeddings and Qdrant collection |

### build_knowledge_index Skill

This skill is responsible for:
1. **Document Chunking**: Split documents into appropriate chunk sizes
2. **Embedding Generation**: Create vector embeddings using the embedding service
3. **Qdrant Upload**: Upload vectors to Qdrant collection
4. **Index Optimization**: Configure optimal index settings

## Qdrant Integration

### Collection Structure

```
Collection: destination_{id}
├── Vectors
│   ├── Dimension: 1536 (OpenAI) or 768 (sentence-transformers)
│   └── Distance: Cosine
├── Payload
│   ├── text: Original document text
│   ├── source: Document source URL
│   ├── category: Information category
│   └── metadata: Additional metadata
└── Index
    └── HNSW configuration
```

### Vector Operations

```go
// Create collection
client.CreateCollection(ctx, &qdrant.CreateCollection{
    CollectionName: collectionID,
    VectorsConfig: &qdrant.VectorsConfig{
        Config: &qdrant.VectorsConfig_Params{
            Params: &qdrant.VectorParams{
                Size:     1536,
                Distance: qdrant.Distance_Cosine,
            },
        },
    },
})

// Upsert points
client.Upsert(ctx, &qdrant.UpsertPoints{
    CollectionName: collectionID,
    Points: []*qdrant.PointStruct{
        {
            Id:      pointID,
            Vectors: vectors,
            Payload: payload,
        },
    },
})
```

## Error Handling

| Error Type | Handling |
|------------|----------|
| Skill failure | Returns error result with failure message |
| Qdrant connection error | Retry with exponential backoff |
| Embedding failure | Log and skip problematic documents |
| Context cancellation | Propagates cancellation |

## Usage Example

```go
// Create tool registry with skills
registry := agent.NewToolRegistry()
registry.RegisterSkill("build_knowledge_index", indexExecutor)

// Load template
template := &agent.AgentTemplate{
    ID:   "indexer",
    Name: "索引构建员",
    Type: agent.AgentTypeIndexer,
}

// Create agent
indexer := agent.NewIndexerAgent("indexer-001", template)
indexer.SetToolRegistry(registry)

// Run indexing
result, err := indexer.Run(ctx, "为京都旅游知识库构建向量索引")
if err != nil {
    log.Fatalf("Indexing failed: %v", err)
}
fmt.Printf("Indexing completed: %+v\n", result)
```

## Testing

The Indexer Agent includes comprehensive tests:

```bash
go test ./internal/agent/... -run "TestIndexer" -v
```

### Test Cases

| Test | Description |
|------|-------------|
| TestIndexerAgentCreation | Verify agent creation and initialization |
| TestIndexerAgentRunWithSuccess | Test successful execution flow |
| TestIndexerAgentSkillFailure | Test error handling when skill fails |
| TestIndexerAgentMemoryTracking | Verify memory operations |
| TestIndexerAgentStop | Test stop functionality |

## Integration Points

### Input Sources
- **Curator Agent**: Receives structured, organized travel data

### Output Consumers
- **Guide Agent**: Uses the vector index for RAG queries
- **Planner Agent**: Queries indexed data for itinerary planning

### Workflow Position

```
Researcher Agent → Curator Agent → Indexer Agent → Guide/Planner
     (search)        (organize)       (index)        (query)
```

## Performance Considerations

### Batch Processing
- Documents are processed in batches for efficiency
- Default batch size: 100 documents
- Configurable via template settings

### Memory Management
- Streaming processing for large document sets
- Bounded memory with automatic trimming
- Thread-safe access with RWMutex

### Index Optimization
- HNSW parameters tuned for tourism queries
- Appropriate M and ef_construct values
- Payload indexes for common filter fields

## Monitoring

### Key Metrics
- Documents indexed per second
- Average embedding generation time
- Qdrant upload latency
- Index size and memory usage

### Logging
```
[Indexer] Starting index build for collection: destination_kyoto
[Indexer] Processing batch 1/10 (100 documents)
[Indexer] Generated 100 embeddings in 2.5s
[Indexer] Uploaded batch 1 to Qdrant in 0.3s
[Indexer] Index build completed: 1000 documents, 15.2s total
```

## Future Enhancements

1. **Incremental Indexing**: Support adding new documents without full rebuild
2. **Multi-language Support**: Language-specific embedding models
3. **Hybrid Search**: Combine vector and keyword search
4. **Index Versioning**: Track index versions for rollback
5. **Parallel Processing**: Distribute embedding generation across workers
