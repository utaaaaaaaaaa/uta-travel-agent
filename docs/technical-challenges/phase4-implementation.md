# Phase 4 技术文档

## 概述

Phase 4 实现了实时导游基础功能：
- 导游页面 UI（景点列表 + 流式聊天）
- 流式讲解 API（SSE）
- RAG 查询服务集成
- 前后端实时通信

## 导游页面架构

### 页面布局

```
┌──────────────────────────────────────────────────────────────────────┐
│                           Header                                       │
│  [←返回] 目的地导游助手                              UTA Travel         │
├─────────────────────┬────────────────────────────────────────────────┤
│                     │                                                 │
│    景点推荐列表      │              AI 讲解区域                        │
│    (左側边栏)        │                                                 │
│                     │    ┌─────────────────────────────────────────┐  │
│  ┌───────────────┐  │    │ Bot: 你好！我是京都导游助手...           │  │
│  │ 📍 金阁寺      │  │    └─────────────────────────────────────────┘  │
│  │ 世界文化遗产   │  │                                                 │
│  └───────────────┘  │    ┌─────────────────────────────────────────┐  │
│                     │    │ User: 介绍一下清水寺                     │  │
│  ┌───────────────┐  │    └─────────────────────────────────────────┘  │
│  │ 🍱 抹茶甜点    │  │                                                 │
│  │ 宇治抹茶冰淇淋  │  │    ┌─────────────────────────────────────────┐  │
│  └───────────────┘  │    │ Bot: 清水寺是京都最著名的...            │  │
│                     │    │     (流式输出)                           │  │
│  ┌───────────────┐  │    └─────────────────────────────────────────┘  │
│  │ 🛍️ 锦市场      │  │                                                 │
│  │ 400年老街      │  │                                                 │
│  └───────────────┘  │                                                 │
│                     ├────────────────────────────────────────────────┤
│                     │  [景点] [美食] [交通] [购物]  快捷按钮          │
│                     ├────────────────────────────────────────────────┤
│                     │  [📷] [🎤] [________________] [发送]            │
└─────────────────────┴────────────────────────────────────────────────┘
```

### 组件结构

```typescript
// apps/web/src/app/guide/[id]/page.tsx

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  isStreaming?: boolean;
}

interface Attraction {
  id: string;
  name: string;
  category: string;
  description: string;
  icon: React.ReactNode;
}

export default function GuidePage() {
  // State
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [agentInfo, setAgentInfo] = useState<Agent | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);

  // Features
  // - 景点列表（分类筛选）
  // - 流式聊天
  // - 快捷问题按钮
  // - 响应式布局
}
```

## 流式讲解 API

### API 端点

#### POST /api/v1/agents/{id}/chat

同步聊天接口。

**请求**:
```json
{
  "message": "介绍一下金阁寺",
  "session_id": "session-123"
}
```

**响应**:
```json
{
  "response": "金阁寺，正式名称为鹿苑寺...",
  "session_id": "session-123"
}
```

#### GET /api/v1/agents/{id}/chat/stream

流式聊天接口（SSE）。

**请求参数**:
- `message` (query): 聊天消息
- `session_id` (query, optional): 会话 ID

**响应事件**:

```
event: chunk
data: {"content":"金阁","done":false}

event: chunk
data: {"content":"寺是","done":false}

event: chunk
data: {"content":"世界文化遗产...","done":true}

event: complete
data: {"session_id":"session-123","agent_id":"agent-456"}
```

### 前端 SSE 连接

```typescript
const streamChat = async (message: string) => {
  const eventSource = new EventSource(
    `${API_URL}/api/v1/agents/${agentId}/chat/stream?message=${encodeURIComponent(message)}`
  );

  eventSource.addEventListener("chunk", (event) => {
    const data = JSON.parse(event.data);
    // 更新消息内容
    setMessages(prev => prev.map(m =>
      m.id === streamingId
        ? { ...m, content: m.content + data.content }
        : m
    ));
  });

  eventSource.addEventListener("complete", () => {
    eventSource.close();
  });

  eventSource.onerror = (error) => {
    eventSource.close();
    // 处理错误
  };
};
```

## RAG 查询服务

### 服务架构

```go
// internal/rag/service.go

type Service struct {
    qdrantClient *qdrant.Client
    llmProvider  llm.Provider
    embeddingSvc EmbeddingService
}

type QueryResult struct {
    Answer     string
    Sources    []Source
    TokensUsed int
}

func (s *Service) Query(ctx context.Context, collectionID, query string, topK int) (*QueryResult, error) {
    // 1. 生成查询向量
    queryVector, err := s.embeddingSvc.Embed(ctx, query)

    // 2. 向量检索
    searchResults, err := s.qdrantClient.Search(ctx, collectionID, queryVector, topK)

    // 3. 构建上下文
    context := buildContext(searchResults)

    // 4. LLM 生成答案
    answer, err := s.generateAnswer(ctx, query, context)

    return &QueryResult{Answer: answer, Sources: sources}, nil
}
```

### 查询流程

```
用户问题: "介绍一下清水寺"
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Embedding Service                                         │
│    将问题转换为向量: [0.1, 0.3, -0.2, ..., 0.5]              │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Qdrant Vector Search                                      │
│    搜索集合: kyoto-1234567890                                │
│    Top 5 相似文档                                            │
│    ├─ 清水寺介绍 (score: 0.92)                              │
│    ├─ 清水寺历史 (score: 0.87)                              │
│    ├─ 清水寺门票 (score: 0.82)                              │
│    ├─ 音羽瀑布 (score: 0.78)                                │
│    └─ 清水舞台 (score: 0.75)                                │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Context Building                                          │
│    合并相关文档为上下文:                                      │
│    "[文档 1] 清水寺是京都最著名的寺院..."                     │
│    "[文档 2] 清水寺建于778年..."                             │
│    ...                                                       │
└─────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. LLM Generation                                            │
│    System: 你是一位专业的旅行导游助手...                      │
│    User: 用户问题: 介绍一下清水寺                            │
│          参考信息: [上下文内容]                              │
│                                                              │
│    Output: 清水寺是京都最著名的寺院之一，建于778年...         │
└─────────────────────────────────────────────────────────────┘
```

### System Prompt

```
你是一位专业的旅行导游助手。你的任务是根据提供的目的地知识库信息，为用户提供准确、有用的旅行建议。

规则:
1. 只使用提供的上下文信息回答问题
2. 如果信息不足，坦诚告知用户
3. 保持回答简洁但有价值
4. 适当添加文化背景和有趣的事实
5. 使用友好的语气，像一位本地导游一样交流
6. 用中文回答
```

## Router 更新

### RouterConfig

```go
type RouterConfig struct {
    Registry  *agent.Registry
    Scheduler *scheduler.Scheduler
    LLMClient llm.Provider
    RAGSvc    *rag.Service
}

func NewRouter(cfg RouterConfig) *Router {
    r := &Router{
        registry:  cfg.Registry,
        scheduler: cfg.Scheduler,
        llmClient: cfg.LLMClient,
        ragSvc:    cfg.RAGSvc,
        mux:       http.NewServeMux(),
    }
    r.setupRoutes()
    return r
}
```

### Chat Handler

```go
func (r *Router) handleAgentChat(w http.ResponseWriter, req *http.Request) {
    agentID := req.PathValue("id")

    // 验证 Agent 存在
    ag, exists := r.registry.Get(agentID)
    if !exists {
        writeError(w, http.StatusNotFound, "agent not found")
        return
    }

    // 解析请求
    var chatReq ChatRequest
    json.NewDecoder(req.Body).Decode(&chatReq)

    var response string

    // 尝试 RAG 查询
    if r.ragSvc != nil && ag.VectorCollectionID != "" {
        result, err := r.ragSvc.Query(req.Context(), ag.VectorCollectionID, chatReq.Message, 5)
        if err != nil {
            // 回退到 mock 响应
            response = generateMockChatResponse(ag.Destination, chatReq.Message)
        } else {
            response = result.Answer
        }
    } else {
        response = generateMockChatResponse(ag.Destination, chatReq.Message)
    }

    writeJSON(w, http.StatusOK, ChatResponse{
        Response:  response,
        SessionID: chatReq.SessionID,
    })
}
```

## 前端特性

### 景点列表

- 分类筛选（景点、美食、购物、交通）
- 点击景点自动填充问题
- 响应式布局（移动端可隐藏）

### 流式聊天

- SSE 连接实时更新
- 打字机效果
- 加载状态显示
- 错误处理和重试

### 快捷按钮

- 必去景点
- 美食推荐
- 交通指南
- 购物攻略

## 测试清单

### 后端测试

```bash
# 1. 编译检查
GO111MODULE=on go build ./...

# 2. 单元测试
GO111MODULE=on go test ./internal/rag/... -v
GO111MODULE=on go test ./internal/router/... -v

# 3. API 端点测试
curl http://localhost:8080/api/v1/agents/{agent_id}/chat \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"message":"介绍一下金阁寺"}'

# 4. SSE 流测试
curl -N "http://localhost:8080/api/v1/agents/{agent_id}/chat/stream?message=介绍一下金阁寺"
```

### 前端测试

```bash
# 1. 编译检查
cd apps/web && npm run build

# 2. 类型检查
npm run lint

# 3. 手动测试
# - 打开导游页面
# - 测试景点点击
# - 测试聊天功能
# - 测试流式输出
# - 测试分类筛选
```

## 已知限制

1. **RAG 需要 Embedding 服务**: 当前使用 Mock Embedding，需要集成真实服务
2. **景点列表是静态数据**: 需要从知识库动态加载
3. **没有会话持久化**: 刷新页面后历史消息丢失
4. **没有语音/图片功能**: 按钮已预留但功能未实现

## 后续改进

1. **Phase 5**: 实现语音识别和图片识别
2. **知识库动态加载**: 从 Qdrant 加载景点列表
3. **会话持久化**: Redis 存储会话历史
4. **多语言支持**: 根据用户语言切换响应语言
5. **位置服务**: 基于用户位置推荐附近景点