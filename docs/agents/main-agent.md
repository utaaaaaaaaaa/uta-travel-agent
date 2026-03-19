# Main Agent

## Overview

The Main Agent is the primary orchestrator in the UTA Travel Agent multi-agent system. It analyzes user goals, creates execution plans, and coordinates subagents to accomplish complex tasks.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Main Agent                                  │
├─────────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │   Memory     │  │    State     │  │    Tools     │               │
│  │   Manager    │  │   Machine    │  │   Registry   │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
├─────────────────────────────────────────────────────────────────────┤
│                       Subagent Management                           │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Researcher │ Curator │ Indexer │ Guide │ Planner │ ...     │   │
│  └─────────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────┤
│                       Core Functions                                │
│  • Goal analysis and planning                                       │
│  • Subagent coordination                                            │
│  • Execution monitoring                                              │
│  • Error handling and recovery                                      │
│  • LLM-powered chat and RAG queries                                 │
└─────────────────────────────────────────────────────────────────────┘
```

## Configuration

```yaml
# configs/agents/main.yaml
kind: AgentTemplate
apiVersion: v1
metadata:
  name: main
  version: 1.0.0
  description: Main orchestrator agent
spec:
  role: |
    你是 UTA Travel 的智能旅游助手。
    你的任务是为用户提供专业、友好的旅游建议和信息。
  capabilities:
    - planning
    - coordination
    - chat
    - rag_query
  tools:
    mcp: []
    skills: []
  decision:
    model: claude-sonnet-4-6
    temperature: 0.7
    max_iterations: 30
    timeout: 300s
  states:
    - idle
    - thinking
    - running
    - waiting
    - completed
    - error
```

## Execution Flow

### Destination Creation Workflow

```
User Request: "创建京都旅游Agent"
         │
         ▼
    ┌─────────┐
    │ Analyze │ ← Detect creation request
    │  Goal   │
    └────┬────┘
         │
         ▼
    ┌─────────┐
    │  Plan   │ Researcher → Curator → Indexer
    │Execution│
    └────┬────┘
         │
         ▼
    ┌─────────────┐
    │  Researcher │ Search for Kyoto travel info
    └──────┬──────┘
           │ (search results, documents)
           ▼
    ┌─────────────┐
    │   Curator   │ Organize and structure data
    └──────┬──────┘
           │ (structured knowledge)
           ▼
    ┌─────────────┐
    │   Indexer   │ Build vector index
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  Completed  │ Destination Agent ready
    └─────────────┘
```

### Planning Workflow

```
User Request: "规划京都三日游"
         │
         ▼
    ┌─────────┐
    │ Analyze │ ← Detect planning request
    │  Goal   │
    └────┬────┘
         │
         ▼
    ┌─────────────┐
    │   Planner   │ Generate itinerary
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  Completed  │ Return itinerary
    └─────────────┘
```

### Guide Workflow

```
User Request: "导游讲解金阁寺"
         │
         ▼
    ┌─────────┐
    │ Analyze │ ← Detect guide request
    │  Goal   │
    └────┬────┘
         │
         ▼
    ┌─────────────┐
    │    Guide    │ RAG query + narration
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  Completed  │ Return explanation
    └─────────────┘
```

## API

### Creation

```go
type MainAgentConfig struct {
    ID          string
    Template    *AgentTemplate
    LLMProvider llm.Provider
}

func NewMainAgent(config MainAgentConfig) *MainAgent
```

### Subagent Management

```go
// Register a subagent
func (a *MainAgent) RegisterSubagent(subagent Agent) error

// Get a subagent by type
func (a *MainAgent) GetSubagent(agentType AgentType) (Agent, bool)

// List all registered subagents (in registration order)
func (a *MainAgent) ListSubagents() []Agent
```

### Tool Management

```go
// Set tool registry for main agent
func (a *MainAgent) SetToolRegistry(registry ToolRegistry)

// Set tools for a specific subagent
func (a *MainAgent) SetSubagentTools(agentType AgentType, registry ToolRegistry) error

// Set tools for all subagents
func (a *MainAgent) SetAllSubagentTools(registry ToolRegistry)
```

### Execution

```go
// Run the main agent with a goal
func (a *MainAgent) Run(ctx context.Context, goal string) (*AgentResult, error)

// Simple chat interaction
func (a *MainAgent) Chat(ctx context.Context, message string) (string, error)

// Streaming chat
func (a *MainAgent) ChatStream(ctx context.Context, message string) (<-chan string, <-chan error)

// RAG-enhanced query
func (a *MainAgent) RAGQuery(ctx context.Context, query, context string) (string, error)

// Stop the agent and all subagents
func (a *MainAgent) Stop() error
```

## Request Type Detection

The Main Agent uses keyword-based detection to classify requests:

| Request Type | Keywords |
|--------------|----------|
| Creation | 创建, 建立, 生成, 制作, create, build, agent |
| Planning | 规划, 行程, 计划, plan, itinerary, 路线 |
| Guide | 导游, 讲解, 介绍, guide, explain, 带我 |

## Execution Plan

```go
type ExecutionStep struct {
    AgentType AgentType  // Subagent to execute
    Goal      string     // Goal for this step
    Required  bool       // Whether failure stops execution
}
```

### Default Plans

**Creation Request:**
```go
[]ExecutionStep{
    {AgentTypeResearcher, goal, true},
    {AgentTypeCurator, "整理研究信息", true},
    {AgentTypeIndexer, "构建知识索引", true},
}
```

**Planning Request:**
```go
[]ExecutionStep{
    {AgentTypePlanner, goal, true},
}
```

**Guide Request:**
```go
[]ExecutionStep{
    {AgentTypeGuide, goal, true},
}
```

## Memory Management

The Main Agent tracks its orchestration activities:

| Type | Content | When |
|------|---------|------|
| thought | 用户目标: {goal} | Start of Run |
| thought | 执行计划: {count} 个步骤 | After planning |
| thought | 步骤 {n}/{total}: 执行 {type} | Before each subagent |
| result | Subagent {type} completed/failed | After each subagent |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Required subagent fails | Return error, stop execution |
| Optional subagent fails | Log error, continue execution |
| No matching plan | Return nil plan (simple chat) |
| Subagent not registered | Log error, skip step |
| Context cancelled | Propagate cancellation |

## Usage Example

```go
// Create tool registry
registry := agent.NewToolRegistry(mcpClient, skillExec, svcClient)

// Create main agent
config := agent.MainAgentConfig{
    ID:       "main-001",
    Template: mainTemplate,
    LLMProvider: llmProvider,
}
mainAgent := agent.NewMainAgent(config)

// Create and register subagents
researcher := agent.NewResearcherAgent("researcher-001", researcherTemplate)
curator := agent.NewCuratorAgent("curator-001", curatorTemplate)
indexer := agent.NewIndexerAgent("indexer-001", indexerTemplate)

mainAgent.RegisterSubagent(researcher)
mainAgent.RegisterSubagent(curator)
mainAgent.RegisterSubagent(indexer)

// Set tools for all subagents
mainAgent.SetAllSubagentTools(registry)

// Run destination creation
ctx := context.Background()
result, err := mainAgent.Run(ctx, "创建京都旅游Agent")
if err != nil {
    log.Fatalf("Failed: %v", err)
}

fmt.Printf("Success: %v\n", result.Success)
fmt.Printf("Duration: %v\n", result.Duration)
```

## Testing

```bash
go test ./internal/agent/... -run "TestMain" -v
```

### Test Cases

| Test | Description |
|------|-------------|
| TestMainAgentCreation | Verify agent creation and initialization |
| TestMainAgentRegisterSubagent | Test subagent registration |
| TestMainAgentGetSubagent | Test subagent retrieval |
| TestMainAgentRunCreationRequest | Test destination creation workflow |
| TestMainAgentRunRequiredSubagentFailure | Test error handling |
| TestMainAgentRunPlanningRequest | Test planning workflow |
| TestMainAgentRunGuideRequest | Test guide workflow |
| TestMainAgentStop | Test stopping all subagents |
| TestMainAgentSetSubagentTools | Test tool assignment |
| TestMainAgentContextCancellation | Test context cancellation |
| TestIsCreationRequest | Test request type detection |
| TestIsPlanningRequest | Test request type detection |
| TestIsGuideRequest | Test request type detection |

## Thread Safety

The Main Agent is thread-safe:
- Uses `sync.RWMutex` for subagent map access
- State changes are protected by mutex
- Memory operations use internal locking

## Future Enhancements

1. **LLM-based Planning**: Use LLM for intelligent execution planning
2. **Parallel Execution**: Execute independent subagents concurrently
3. **Retry Logic**: Automatic retry for failed subagents
4. **Progress Streaming**: Stream execution progress to clients
5. **Dynamic Subagent Loading**: Load subagents based on needs
6. **Cost Tracking**: Track LLM token usage across subagents