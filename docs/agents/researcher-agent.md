# Researcher Agent

## Overview

The Researcher Agent is a specialized subagent responsible for collecting travel information about destinations. It autonomously searches the web, crawls relevant pages, and extracts structured travel information to build a knowledge base for the UTA Travel Agent system.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Researcher Agent                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │   Memory    │    │   State     │    │   Tools     │     │
│  │  Management │    │   Machine   │    │  Registry   │     │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘     │
│         │                  │                  │             │
│         └──────────────────┼──────────────────┘             │
│                            ▼                                │
│              ┌─────────────────────────┐                   │
│              │     Run() Execution     │                   │
│              │  1. Search Web          │                   │
│              │  2. Crawl Pages         │                   │
│              │  3. Extract Info        │                   │
│              └─────────────────────────┘                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## How It Works

### Execution Flow

```
1. Receive Goal (e.g., "Research Kyoto travel information")
   ↓
2. Set State: Thinking
   ↓
3. Add goal to Memory
   ↓
4. Set State: Running
   ↓
5. Execute brave_search tool with goal as query
   ↓
6. Parse search results, extract URLs
   ↓
7. For each URL (up to 5):
   ├── Execute web_reader tool
   ├── Store document content
   └── Add observation to memory
   ↓
8. Execute extract_travel_info skill
   ├── Process collected documents
   ├── Extract structured information
   └── Categorize by type (attractions, food, etc.)
   ↓
9. Set State: Completed
   ↓
10. Return AgentResult with collected data
```

### State Machine

| State | Description |
|-------|-------------|
| `idle` | Agent is not running |
| `thinking` | Agent is analyzing the goal |
| `running` | Agent is executing tools |
| `completed` | Agent finished successfully |
| `error` | Agent encountered an error |

## Configuration

### Template Definition

The Researcher Agent is configured via `agent-templates/researcher.yaml`:

```yaml
kind: AgentTemplate
apiVersion: agent.uta/v1

metadata:
  name: researcher
  version: v1.0.0

spec:
  role: |
    Professional travel information researcher...

  capabilities:
    - search_web
    - crawl_pages
    - extract_content
    - summarize
    - validate_info

  tools:
    mcp:
      - name: brave_search
        required: true
      - name: web_reader
        required: true
    skills:
      - extract_travel_info
    services:
      - llm_gateway

  decision:
    model: claude-sonnet-4-6
    temperature: 0.2
    max_iterations: 30
    timeout: 600s
```

### Decision Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `model` | claude-sonnet-4-6 | LLM model for decision making |
| `temperature` | 0.2 | Low temperature for consistent results |
| `max_iterations` | 30 | Maximum tool execution iterations |
| `timeout` | 600s | Total execution timeout |

## API

### Creating a Researcher Agent

```go
import "github.com/utaaa/uta-travel-agent/internal/agent"

// Load template
template := &agent.AgentTemplate{
    Metadata: agent.TemplateMetadata{
        Name:    "researcher",
        Version: "v1.0.0",
    },
    Spec: agent.TemplateSpec{
        Decision: agent.DecisionConfig{
            MaxIterations: 30,
            Timeout:       600 * time.Second,
        },
    },
}

// Create agent
researcher := agent.NewResearcherAgent("researcher-001", template)

// Set tools
toolRegistry := agent.NewToolRegistry(mcpClient, skillExecutor, serviceClient)
researcher.SetTools(toolRegistry)
```

### Running the Agent

```go
ctx := context.Background()
result, err := researcher.Run(ctx, "Research Kyoto travel information")

if err != nil {
    log.Fatalf("Research failed: %v", err)
}

if result.Success {
    fmt.Printf("Collected %d documents\n", result.Metadata["urls_processed"])

    // Access collected data
    output := result.Output.(map[string]any)
    documents := output["documents"].([]any)
    extracted := output["extracted"]
}
```

### Output Structure

```go
type AgentResult struct {
    AgentID    string
    AgentType  AgentType
    Goal       string
    Success    bool
    Output     map[string]any  // Contains collected data
    Duration   time.Duration
    Metadata   map[string]any  // Contains execution stats
}

// Output map structure:
// {
//     "search_results": {...},
//     "documents": [...],
//     "extracted": {
//         "attractions": [...],
//         "food": [...],
//         "transport": [...],
//         ...
//     }
// }
```

## Memory Management

The Researcher Agent uses working memory to track execution:

```go
// Memory types used:
// - thought: Agent's reasoning
// - action: Tool execution attempts
// - observation: Tool results
// - result: Final outcomes

// Access memory
thoughts := researcher.Memory().GetByType("thought")
actions := researcher.Memory().GetByType("action")
observations := researcher.Memory().GetByType("observation")
```

## Tool Dependencies

### Required MCP Tools

| Tool | Purpose | Parameters |
|------|---------|------------|
| `brave_search` | Web search | `query`, `count`, `search_lang` |
| `web_reader` | Page crawling | `url` |

### Optional Skills

| Skill | Purpose |
|-------|---------|
| `extract_travel_info` | Extract structured travel data from documents |

## Error Handling

```go
result, err := researcher.Run(ctx, goal)
if err != nil {
    // Check error type
    if errors.Is(err, ErrNoToolsAvailable) {
        // Tools not configured
    }

    // Agent returned error result
    if !result.Success {
        log.Printf("Agent error: %s", result.Error)
    }
}
```

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `no tools available` | Tools not set | Call `SetTools()` before running |
| `搜索失败` | Brave Search API failure | Check API key, rate limits |
| `tool not found` | Missing tool in registry | Register required tools |

## Integration with Main Agent

The Researcher Agent is typically orchestrated by the Main Agent:

```go
// In Main Agent orchestration
func (a *MainAgent) createDestinationAgent(ctx context.Context, destination string) error {
    // Step 1: Research
    researcher := a.GetSubagent(AgentTypeResearcher)
    result, err := researcher.Run(ctx, fmt.Sprintf("Research %s travel information", destination))
    if err != nil {
        return err
    }

    // Step 2: Pass to Curator for organization
    curator := a.GetSubagent(AgentTypeCurator)
    // ... pass research results to curator
}
```

## Performance Considerations

1. **URL Limit**: Agent processes at most 5 URLs per search to prevent timeout
2. **Concurrent Execution**: Can run multiple researcher agents in parallel for different topics
3. **Memory Size**: Default memory size is 100 items, configurable via template
4. **Timeout**: 600s default timeout may need adjustment for slow networks

## Testing

Run the Researcher Agent tests:

```bash
go test -v ./internal/agent/... -run TestResearcher
```

### Test Coverage

- Agent creation and configuration
- Successful execution flow
- Search failure handling
- Web reader failure handling
- Memory tracking
- URL extraction helper
- Context cancellation
- Missing tools handling

## Future Enhancements

1. **Intelligent Search Strategy**: Use LLM to plan search keywords
2. **Source Validation**: Validate source credibility
3. **Incremental Updates**: Support updating existing research
4. **Multi-language Support**: Search in multiple languages
5. **Caching**: Cache search results to avoid redundant API calls