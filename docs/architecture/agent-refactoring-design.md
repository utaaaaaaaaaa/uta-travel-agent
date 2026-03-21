# Agent 架构重构设计

## 背景

当前 MainAgent 和 GuideAgent 应该是长期存在的会话型 Agent，需要：
- 持久化内存
- 上下文工程
- 会话管理

而 ResearcherAgent、CuratorAgent、IndexerAgent 是短期的任务型 Agent，只需要简单内存。

## 当前问题

### 1. 内存系统不一致

| Agent | 内存实现 | 问题 |
|-------|----------|------|
| MainAgent | BaseAgent.memory (AgentMemory) | ✅ 正确 |
| GuideAgent | LLMAgent.memory | ✅ 正确但无持久化 |
| ResearcherAgent | roundSummaries (自己实现) | ❌ 不使用 AgentMemory |
| CuratorAgent | LLMAgent.memory | ⚠️ 临时使用 |
| IndexerAgent | LLMAgent.memory | ⚠️ 临时使用 |

### 2. 缺少上下文工程

- 无上下文窗口管理
- 无上下文压缩/摘要
- 无优先级排序

### 3. 缺少会话管理

- 无会话持久化
- 无会话隔离
- 无会话恢复

## 设计方案

### 架构分层

```
SessionAgent (Base for MainAgent, GuideAgent)
├── SessionManager      # 会话生命周期
├── PersistentMemory    # 持久化内存
├── ContextEngineer     # 上下文工程
└── LLMBrain            # LLM 调用

TaskAgent (Base for ResearcherAgent, etc.)
├── SimpleMemory        # 简单内存
└── LLMBrain            # LLM 调用
```

### 新增接口

```go
// SessionAgent 会话型 Agent 接口
type SessionAgent interface {
    Agent

    // Session 管理
    SessionID() string
    SaveSession() (*SessionSnapshot, error)
    LoadSession(snapshot *SessionSnapshot) error

    // 上下文工程
    GetContextWindow() int
    SetContextWindow(maxTokens int)
    CompressContext() error

    // 持久化内存
    GetLongTermMemory() *PersistentMemory
    Remember(key string, value any) error
    Recall(key string) (any, error)
}

// TaskAgent 任务型 Agent 接口
type TaskAgent interface {
    Agent

    // 简单任务执行
    ExecuteTask(ctx context.Context, task string) (*AgentResult, error)
}
```

### 核心组件

#### 1. SessionManager

```go
type SessionManager struct {
    sessionID    string
    createdAt    time.Time
    lastActiveAt time.Time
    metadata     map[string]any
    state        SessionState
}

type SessionState string

const (
    SessionStateActive   SessionState = "active"
    SessionStatePaused   SessionState = "paused"
    SessionStateClosed   SessionState = "closed"
)

type SessionSnapshot struct {
    ID           string         `json:"id"`
    CreatedAt    time.Time      `json:"created_at"`
    Memory       []MemoryItem   `json:"memory"`
    Conversation []Message      `json:"conversation"`
    Metadata     map[string]any `json:"metadata"`
}
```

#### 2. PersistentMemory

```go
type PersistentMemory struct {
    *AgentMemory                    // 继承短期内存

    // 长期记忆
    longTerm     []MemoryItem      // 重要信息的长期存储
    embeddings   map[string][]float32  // 向量索引

    // 持久化
    storage      MemoryStorage     // 存储后端
}

func (m *PersistentMemory) Remember(key string, value any) error
func (m *PersistentMemory) Recall(key string) (any, error)
func (m *PersistentMemory) Search(query string, k int) []MemoryItem
func (m *PersistentMemory) Save() error
func (m *PersistentMemory) Load(sessionID string) error
```

#### 3. ContextEngineer

```go
type ContextEngineer struct {
    maxTokens      int
    currentTokens  int
    compressionFn  func([]MemoryItem) string

    // 优先级队列
    priorities    map[string]int
}

func (e *ContextEngineer) BuildContext(memory *AgentMemory, maxTokens int) []llm.Message
func (e *ContextEngineer) Compress(items []MemoryItem) string
func (e *ContextEngineer) Prioritize(itemType string, priority int)
func (e *ContextEngineer) EstimateTokens(content string) int
```

### 重构后的 Agent 实现

#### BaseSessionAgent

```go
type BaseSessionAgent struct {
    id             string
    agentType      AgentType
    state          AgentState

    // 新增组件
    session        *SessionManager
    memory         *PersistentMemory
    contextEngine  *ContextEngineer

    // LLM
    llmProvider    llm.Provider
    systemPrompt   string
    tools          ToolRegistry
}

func NewBaseSessionAgent(config SessionAgentConfig) *BaseSessionAgent {
    return &BaseSessionAgent{
        id:            config.ID,
        agentType:     config.AgentType,
        session:       NewSessionManager(),
        memory:        NewPersistentMemory(config.Storage),
        contextEngine: NewContextEngineer(config.MaxContextTokens),
        llmProvider:   config.LLMProvider,
        systemPrompt:  config.SystemPrompt,
        tools:         config.Tools,
    }
}
```

#### 重构后的 MainAgent

```go
type MainAgent struct {
    *BaseSessionAgent  // 使用新的 BaseSessionAgent

    // 子 Agent 管理
    subagents      map[AgentType]Agent
}

func (a *MainAgent) Chat(ctx context.Context, message string) (string, error) {
    // 1. 更新会话状态
    a.session.Touch()

    // 2. 添加到内存
    a.memory.AddMessage("user", message)

    // 3. 构建上下文 (使用 ContextEngineer)
    messages := a.contextEngine.BuildContext(a.memory, a.contextEngine.maxTokens)

    // 4. 调用 LLM
    response, err := a.llmProvider.CompleteWithSystem(ctx, a.systemPrompt, messages)

    // 5. 保存响应
    a.memory.AddMessage("assistant", response.Content)

    // 6. 自动保存会话
    a.SaveSession()

    return response.Content, nil
}
```

#### 重构后的 GuideAgent

```go
type GuideAgent struct {
    *BaseSessionAgent  // 使用相同的 BaseSessionAgent

    // 导游特有
    destination      string
    collectionID     string
    ragService       *rag.Service
}

func (a *GuideAgent) Guide(ctx context.Context, query string) (string, error) {
    // 1. 更新会话
    a.session.Touch()

    // 2. 检索相关知识
    context, sources := a.ragService.Query(ctx, a.collectionID, query)

    // 3. 构建上下文 (包含 RAG 结果)
    messages := a.contextEngine.BuildContext(a.memory, a.contextEngine.maxTokens)
    messages = append(messages, llm.Message{
        Role:    "system",
        Content: fmt.Sprintf("参考知识:\n%s", context),
    })

    // 4. 调用 LLM
    response, _ := a.llmProvider.CompleteWithSystem(ctx, a.systemPrompt, messages)

    // 5. 记住这次对话
    a.memory.AddMessage("user", query)
    a.memory.AddMessage("assistant", response.Content)

    // 6. 如果是重要信息，记住到长期内存
    if isImportant(query) {
        a.Remember("last_query", query)
    }

    return response.Content, nil
}
```

### 存储后端

```go
type MemoryStorage interface {
    Save(sessionID string, snapshot *SessionSnapshot) error
    Load(sessionID string) (*SessionSnapshot, error)
    Delete(sessionID string) error
}

// PostgreSQL 实现
type PostgreSQLStorage struct {
    db *sql.DB
}

// Redis 实现 (可选，用于缓存)
type RedisStorage struct {
    client *redis.Client
}
```

## 迁移计划

### Phase 1: 创建共享组件

1. 创建 `SessionManager`
2. 创建 `PersistentMemory`
3. 创建 `ContextEngineer`
4. 创建 `BaseSessionAgent`

### Phase 2: 重构 MainAgent

1. 继承 `BaseSessionAgent`
2. 实现会话保存/恢复
3. 实现上下文压缩

### Phase 3: 重构 GuideAgent

1. 继承 `BaseSessionAgent`
2. 添加 RAG 集成
3. 实现长期记忆

### Phase 4: 简化 TaskAgent

1. ResearcherAgent 使用简单内存
2. CuratorAgent 简化
3. IndexerAgent 简化

## 文件结构

```
internal/agent/
├── agent.go              # Agent 接口定义
├── session_agent.go      # SessionAgent 接口 + BaseSessionAgent (新)
├── task_agent.go         # TaskAgent 接口 (新)
├── main_agent.go         # MainAgent (重构)
├── guide_agent.go        # GuideAgent (新，独立实现)
├── researcher_agent.go   # ResearcherAgent (简化)
├── curator_agent.go      # CuratorAgent (简化)
├── indexer_agent.go      # IndexerAgent (简化)
├── memory.go             # AgentMemory
├── persistent_memory.go  # PersistentMemory (新)
├── context_engineer.go   # ContextEngineer (新)
├── session_manager.go    # SessionManager (新)
├── factory.go            # Agent 工厂
└── registry.go           # Agent 注册表
```

## 收益

| 改进 | Before | After |
|------|--------|-------|
| 会话持久化 | ❌ | ✅ PostgreSQL 存储 |
| 上下文管理 | ❌ 手动 | ✅ 自动压缩 |
| 内存共享 | ❌ 各自实现 | ✅ 共享 BaseSessionAgent |
| 长期记忆 | ❌ | ✅ 向量索引 |
| 会话恢复 | ❌ | ✅ Load/Save |

## 注意事项

1. **向后兼容**: 保持 Agent 接口不变
2. **渐进迁移**: 先添加新组件，再逐步重构
3. **测试覆盖**: 每个新组件都需要测试
4. **性能考虑**: 上下文压缩需要高效实现