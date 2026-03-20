# Phase 2 技术文档

## 概述

Phase 2 实现了真正的 Agent 创建流程，包括：
- 前端连接真实 API
- SSE 进度流推送
- Scheduler 任务调度改进

## API 端点

### Agent API

#### POST /api/v1/agents

创建新的目的地 Agent。

**请求体**:
```json
{
  "destination": "京都",
  "name": "京都旅游助手",      // 可选
  "description": "...",        // 可选
  "theme": "cultural",         // 可选，默认 cultural
  "languages": ["zh"],         // 可选
  "user_id": "user-123"        // 可选
}
```

**响应**:
```json
{
  "agent_id": "b01cfed0-17a5-4ff1-8611-3294a9fda395",
  "task_id": "0c57dd88-c7da-43f0-9e8c-2ceef816bb35",
  "status": "creating",
  "message": "正在创建 京都 导游 Agent"
}
```

#### GET /api/v1/agents

列出用户的所有 Agent。

**查询参数**:
- `user_id` (可选): 用户 ID

**响应**:
```json
{
  "agents": [
    {
      "id": "b01cfed0-...",
      "user_id": "default-user",
      "name": "京都旅游助手",
      "destination": "京都",
      "status": "ready",
      "document_count": 42,
      "created_at": "2026-03-19T22:52:44Z"
    }
  ],
  "count": 1
}
```

#### GET /api/v1/agents/{id}

获取单个 Agent 详情。

#### DELETE /api/v1/agents/{id}

删除 Agent。

### Task API

#### GET /api/v1/tasks/{id}

获取任务详情。

**响应**:
```json
{
  "id": "0c57dd88-...",
  "agent_id": "b01cfed0-...",
  "status": "completed",
  "duration_seconds": 6.5,
  "total_tokens": 5000,
  "exploration_log": [
    {
      "timestamp": "2026-03-19T22:52:45Z",
      "direction": "景点",
      "thought": "搜索京都著名景点...",
      "action": "researching",
      "tokens_in": 150,
      "tokens_out": 200
    }
  ]
}
```

#### GET /api/v1/tasks/{id}/stream

SSE 进度流。

**事件类型**:

1. `progress` - 进度更新
```
event: progress
data: {"stage":"researching","step":{"direction":"景点","thought":"搜索..."},"message":"正在搜索..."}
```

2. `complete` - 任务完成
```
event: complete
data: {"task_id":"xxx","status":"completed","agent_id":"xxx","duration_sec":6.5,"tokens":5000}
```

## SSE 进度流实现

### 后端实现 (router.go)

```go
func (r *Router) handleTaskStream(w http.ResponseWriter, req *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        writeError(w, http.StatusInternalServerError, "streaming not supported")
        return
    }

    // Poll for updates
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            task, _ := r.scheduler.Get(taskID)

            // Send progress update
            if len(task.ExplorationLog) > 0 {
                r.sendSSE(w, flusher, "progress", progressData)
            }

            // Check completion
            if task.Status == "completed" {
                r.sendSSE(w, flusher, "complete", completeData)
                return
            }
        }
    }
}

func (r *Router) sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
    dataJSON, _ := json.Marshal(data)
    fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(dataJSON))
    flusher.Flush()
}
```

### 前端实现 (create/page.tsx)

```typescript
const handleCreate = async () => {
  // 1. Create agent via API
  const response = await fetch(`${API_BASE_URL}/api/v1/agents`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ destination, theme, languages }),
  });

  const { agent_id, task_id } = await response.json();

  // 2. Connect to SSE
  const eventSource = new EventSource(`${API_BASE_URL}/api/v1/tasks/${task_id}/stream`);

  eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);
    handleProgressUpdate(data);
  };

  eventSource.addEventListener('complete', (event) => {
    const data = JSON.parse(event.data);
    setStatus('completed');
    eventSource.close();
  });

  eventSource.onerror = () => {
    eventSource.close();
    // Fall back to polling
    pollTaskStatus(task_id, agent_id);
  };
};
```

## Scheduler 架构

### 架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    Scheduler v2                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────┐                                            │
│  │   Router    │                                            │
│  │  (HTTP API) │                                            │
│  └──────┬──────┘                                            │
│         │ Submit(task, priority)                            │
│         ▼                                                    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Priority Queues                         │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐             │    │
│  │  │  High   │  │ Normal  │  │   Low   │             │    │
│  │  │  chan   │  │  chan   │  │  chan   │             │    │
│  │  └────┬────┘  └────┬────┘  └────┬────┘             │    │
│  └───────┼────────────┼────────────┼───────────────────┘    │
│          │            │            │                         │
│          ▼            ▼            ▼                         │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Worker Pool (N workers)                 │    │
│  │  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐     │    │
│  │  │Worker 0│  │Worker 1│  │Worker 2│  │Worker 3│     │    │
│  │  └────────┘  └────────┘  └────────┘  └────────┘     │    │
│  └─────────────────────────────────────────────────────┘    │
│          │                                                    │
│          ▼                                                    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Task Handler                            │    │
│  │  • executeTask()                                     │    │
│  │  • Retry logic (3 attempts)                          │    │
│  │  • Exponential backoff                               │    │
│  │  • Status updates                                    │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 核心数据结构

```go
// TaskPriority 任务优先级
type TaskPriority int

const (
    PriorityLow    TaskPriority = 0
    PriorityNormal TaskPriority = 1
    PriorityHigh   TaskPriority = 2
)

// ScheduledTask 带调度元数据的任务
type ScheduledTask struct {
    *agent.AgentTask           // 嵌入 AgentTask
    Priority    TaskPriority   // 优先级
    RetryCount  int            // 已重试次数
    MaxRetries  int            // 最大重试次数
    SubmittedAt time.Time      // 提交时间
    StartedAt   *time.Time     // 开始时间
}

// Scheduler 调度器
type Scheduler struct {
    tasks       map[string]*ScheduledTask
    queues      map[TaskPriority]chan *ScheduledTask
    workerCount int
    handlers    map[string]TaskHandler
    completedCount int64
    failedCount    int64
}
```

### Worker 流程

```go
func (s *Scheduler) worker(id int) {
    for {
        select {
        case <-s.ctx.Done():
            return
        case task := <-s.queues[PriorityHigh]:  // 优先处理高优先级
            s.executeTask(id, task)
        case task := <-s.queues[PriorityNormal]:
            s.executeTask(id, task)
        case task := <-s.queues[PriorityLow]:
            s.executeTask(id, task)
        }
    }
}

func (s *Scheduler) executeTask(workerID int, task *ScheduledTask) {
    // 1. 更新状态为 running
    task.Status = agent.TaskStatusRunning

    // 2. 执行任务
    err := handler(ctx, task)

    // 3. 处理结果
    if err != nil {
        if task.RetryCount < task.MaxRetries {
            // 重试
            task.RetryCount++
            time.Sleep(backoff)
            queue <- task
        } else {
            // 标记失败
            task.Status = agent.TaskStatusFailed
        }
    } else {
        // 成功
        task.Status = agent.TaskStatusCompleted
    }
}
```

### 重试机制

- 默认最多重试 3 次
- 指数退避: 1s, 2s, 3s
- 队列满时丢弃任务

## 前端 API Client

### 完整 API 列表

```typescript
export const api = {
  // Agent CRUD
  listAgents(userId?: string): Promise<{ agents: Agent[]; count: number }>
  getAgent(id: string): Promise<Agent>
  createAgent(data: CreateAgentRequest): Promise<CreateAgentResponse>
  deleteAgent(id: string): Promise<void>

  // Task operations
  getTask(id: string): Promise<AgentTask>
  createTask(agentId: string, goal: string): Promise<{ task_id: string }>
  streamTaskProgress(taskId: string, callbacks): () => void

  // Chat
  chat(agentId: string, data: ChatRequest): Promise<ChatResponse>
  chatStream(data: ChatRequest): AsyncGenerator<string>
}
```

### SSE 连接管理

```typescript
// 返回清理函数，支持组件卸载时自动关闭
const cleanup = api.streamTaskProgress(taskId, {
  onProgress: (data) => updateUI(data),
  onComplete: (data) => finishCreation(data),
  onError: (err) => handleError(err),
});

// useEffect 中使用
useEffect(() => {
  const cleanup = startStreaming();
  return cleanup; // 组件卸载时自动清理
}, []);
```

## 测试清单

### Phase 2 测试

```bash
# 1. 编译检查
GO111MODULE=on go build ./internal/...

# 2. 单元测试
GO111MODULE=on go test ./internal/agent/... -v
GO111MODULE=on go test ./internal/scheduler/... -v

# 3. API 端点测试
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{"destination":"京都"}'

# 4. SSE 流测试
curl -N http://localhost:8080/api/v1/tasks/{task_id}/stream

# 5. 前端构建测试
cd apps/web && npm run build
```

## 已知限制

1. **Scheduler 未启动**: 当前 `router.go` 只调用 `Save()`，未启动 worker。需要在 `main.go` 中调用 `scheduler.Start()`

2. **SSE 是轮询**: 当前实现是 500ms 轮询，不是真正的推送。需要改进为任务更新时主动推送

3. **任务处理器是模拟**: `defaultCreateAgentHandler` 只是 sleep 5 秒，需要集成真正的 Subagent 执行

## 后续改进

1. **Phase 3**: 实现 LLMAgent 替换硬编码 Subagent
2. **实时推送**: 任务更新时主动推送 SSE 事件
3. **任务持久化**: 将任务状态持久化到 PostgreSQL