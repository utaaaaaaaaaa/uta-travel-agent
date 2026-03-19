# Curator Agent

## Overview

The Curator Agent is responsible for organizing and structuring collected travel information. It receives raw data from the Researcher Agent and transforms it into a structured knowledge base.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Curator Agent                           │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Memory     │  │    State     │  │    Tools     │       │
│  │   Manager    │  │   Machine    │  │   Registry   │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
├─────────────────────────────────────────────────────────────┤
│                     Core Functions                           │
│  • Information categorization                                │
│  • Duplicate removal                                         │
│  • Knowledge graph building                                  │
│  • Data quality validation                                   │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

The Curator Agent uses a YAML template for configuration:

```yaml
# configs/agents/curator.yaml
id: curator
name: "信息整理员"
type: curator
description: "整理和结构化旅游信息"
version: "1.0.0"

tools:
  - build_knowledge_base

capabilities:
  - categorization
  - deduplication
  - knowledge_graph
  - validation

settings:
  max_retries: 3
  timeout: 120s
```

## Execution Flow

```
Start → Thinking → Running (build_knowledge_base) → Completed
                    ↓
                  Error (if skill fails)
```

### State Transitions

| From | To | Trigger |
|------|-----|---------|
| Idle | Thinking | Run() called |
| Thinking | Running | After adding thought to memory |
| Running | Completed | Skill execution success |
| Running | Error | Skill execution failure |
| Completed/Error | Idle | Deferred cleanup |

## API

### Creation

```go
func NewCuratorAgent(id string, template *AgentTemplate) *CuratorAgent
```

### Execution

```go
func (a *CuratorAgent) Run(ctx context.Context, goal string) (*AgentResult, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeout
- `goal`: The curation goal/objective

**Returns:**
- `*AgentResult`: Contains organized output data
- `error`: Any execution error

### Lifecycle

```go
func (a *CuratorAgent) Stop() error
```

## Memory Management

The Curator Agent tracks its operations in memory:

| Type | Content | Source/Metadata |
|------|---------|-----------------|
| thought | 整理目标: {goal} | - |
| result | 知识库整理完成 | success: true |

### Memory Example

```go
// Thought recorded at start
Memory.AddThought("整理目标: 整理京都旅游信息")

// Result recorded on completion
Memory.AddResult("知识库整理完成", true, nil)
```

## Tool Dependencies

| Tool | Type | Purpose |
|------|------|---------|
| build_knowledge_base | Skill | Build structured knowledge from raw data |

### build_knowledge_base Skill

This skill is responsible for:
1. **Categorization**: Grouping information into categories (attractions, restaurants, transport, etc.)
2. **Deduplication**: Removing duplicate entries
3. **Knowledge Graph**: Building relationships between entities
4. **Validation**: Checking data quality and completeness

## Error Handling

| Error Type | Handling |
|------------|----------|
| Skill failure | Returns error result with failure message |
| Context cancellation | Propagates cancellation |
| Timeout | Returns timeout error |

## Usage Example

```go
// Create tool registry with skills
registry := agent.NewToolRegistry()
registry.RegisterSkill("build_knowledge_base", knowledgeBaseExecutor)

// Load template
template := &agent.AgentTemplate{
    ID:   "curator",
    Name: "信息整理员",
    Type: agent.AgentTypeCurator,
}

// Create agent
curator := agent.NewCuratorAgent("curator-001", template)
curator.SetToolRegistry(registry)

// Run curation
result, err := curator.Run(ctx, "整理京都旅游信息")
if err != nil {
    log.Fatalf("Curation failed: %v", err)
}
fmt.Printf("Curation completed: %+v\n", result)
```

## Testing

The Curator Agent includes comprehensive tests:

```bash
go test ./internal/agent/... -run "TestCurator" -v
```

### Test Cases

| Test | Description |
|------|-------------|
| TestCuratorAgentCreation | Verify agent creation and initialization |
| TestCuratorAgentRunWithSuccess | Test successful execution flow |
| TestCuratorAgentSkillFailure | Test error handling when skill fails |
| TestCuratorAgentMemoryTracking | Verify memory operations |
| TestCuratorAgentStop | Test stop functionality |

## Integration Points

### Input Sources
- **Researcher Agent**: Receives raw search results and documents

### Output Consumers
- **Indexer Agent**: Provides structured data for vector indexing

### Workflow Position

```
Researcher Agent → Curator Agent → Indexer Agent
     (search)        (organize)       (index)
```

## Performance Considerations

- **Memory Usage**: Uses bounded memory with automatic trimming
- **Concurrency**: Thread-safe memory access with RWMutex
- **Timeout**: Respects context deadline for long operations

## Future Enhancements

1. **Parallel Processing**: Process multiple categories concurrently
2. **Streaming Output**: Stream organized data as it's processed
3. **Quality Metrics**: Add data quality scoring
4. **Incremental Updates**: Support incremental knowledge base updates
