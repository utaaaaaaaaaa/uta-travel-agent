# UTA Travel Agent - Multi-Agent Tourism Assistant

## Project Vision

UTA (Universal Travel Agent) 是一个基于 Multi-Agent 架构的智能旅游助手系统。核心理念是"Vibecoding"——让技术为体验服务，通过 AI Agent 为用户提供沉浸式的旅游文化体验。

### 核心功能
1. **目的地研究 Agent**: 自动搜索、整理旅游目的地信息，构建 RAG 知识库
2. **智能导游 Agent**: 实地旅游时，基于位置/图片识别景点，提供文化背景讲解
3. **行程规划 Agent**: 根据用户偏好生成个性化旅游行程
4. **Agent 持久化**: 创建的目的地 Agent 永久保存，用户可随时调用
5. **多语言支持**: 支持多语言交流和翻译

## Technical Architecture

**分层架构** (自上而下):
- **Frontend (TypeScript)**: Destination Explorer | Guide Mode | User Dashboard
- **Orchestration (Go)**: Agent Scheduler | Task Router | Agent Registry | gRPC Gateway
- **Agent Services (Python)**: Destination Agent (RAG) | Guide Agent (Vision) | Planner Agent
- **Storage Layer**: PostgreSQL | Redis | Qdrant | S3/MinIO
- **Sandbox Layer (Rust, 可选)**: 安全隔离执行

## Technology Stack

| Layer | Language | Technology | Purpose |
|-------|----------|------------|---------|
| Frontend | TypeScript | Next.js / React | 用户界面，Agent管理，实时导游 |
| Orchestration | Go | Gin/Fiber | Agent调度、任务路由、gRPC网关 |
| Agent Services | Python | FastAPI | LLM调用、RAG、视觉识别、推理 |
| Sandbox | Rust | Tokio | 沙箱执行、安全隔离、资源限制 |
| Communication | gRPC | buf | 跨语言服务通信 |
| Vector DB | Qdrant | - | RAG知识存储、向量检索 |
| Cache | Redis | - | 会话缓存、实时状态 |
| Database | PostgreSQL | - | Agent元数据、用户数据持久化 |
| Storage | MinIO/S3 | - | 文件、图片存储 |
| LLM | Claude API | anthropic SDK | 主要LLM能力 |

## Project Structure

```
uta-travel-agent/
├── apps/web/                    # 前端 (Next.js)
│   └── src/{app,components,hooks,lib,stores,types}/
├── cmd/orchestrator/            # Go entrypoint
├── internal/                    # Go packages
│   ├── agent/{registry,lifecycle,persistence}.go
│   ├── scheduler/, router/, grpc/
│   └── storage/{postgres,redis,qdrant}/
├── proto/agent/                 # Protocol Buffers
├── services/                    # Python Agent 服务
│   ├── destination-agent/src/{rag,crawler,agent.py}
│   ├── guide-agent/src/{vision,narrator,location,agent.py}
│   └── planner-agent/
├── sandbox/                     # Rust (可选)
├── infra/{docker,k8s,terraform}/
├── docs/
└── CLAUDE.md
```

## Agent Persistence Design

### 核心概念
- **随时调用**: 实地旅游时直接使用已创建的 Agent
- **收藏管理**: 收藏常用的目的地 Agent
- **分享**: 与其他用户分享 Agent
- **版本控制**: Agent 知识库可更新迭代

### Agent 数据模型

```go
type DestinationAgent struct {
    ID, UserID, Name, Description, Destination string
    VectorCollectionID string
    DocumentCount int
    Language string
    Tags []string
    Theme string  // cultural/adventure/food
    Status AgentStatus  // creating/ready/archived
    CreatedAt, UpdatedAt time.Time
    LastUsedAt *time.Time
    UsageCount int
    Rating float64
}
```

### 存储架构

**创建 Agent**: PostgreSQL(元数据) → Qdrant(向量索引) → S3(原始文档)

**使用 Agent**: Redis(会话缓存) → Qdrant(RAG检索)

## Frontend Design

### 技术栈

| Category | Technology |
|----------|------------|
| Framework | Next.js 15 (App Router) |
| UI | shadcn/ui + Tailwind CSS |
| State | Zustand |
| Data Fetching | TanStack Query |
| Forms | React Hook Form + Zod |
| Maps | Mapbox GL / Deck.gl |
| Animation | Framer Motion |

### 页面结构

- **首页**: Hero + Agent 卡片列表 + 创建入口
- **导游模式**: 左侧地图 | 右侧 AI Chat | 底部操作栏
- **创建流程**: Step1 目的地信息 → Step2 进度展示

### 前端目录约定

```
src/
├── app/           # Next.js App Router
├── components/    # ui/, destination/, guide/, chat/, layout/
├── lib/           # api/, utils/, constants/
├── hooks/
├── stores/
└── types/
```

## Development Guidelines

### Frontend (TypeScript/Next.js)
- 使用 App Router，优先 Server Components
- 组件使用 shadcn/ui
- 状态: Zustand + TanStack Query
- 地图: Mapbox GL JS
- 响应式设计

### Go (Orchestration)
- 标准项目布局
- 错误处理: 自定义错误类型
- 配置: Viper/环境变量
- 日志: zerolog/zap
- 并发: goroutine + channel

### Python (Agent Services)
- 包管理: `uv`
- 异步: `asyncio` + `httpx`
- LLM: `anthropic` SDK
- 向量存储: Qdrant 客户端
- 类型: `pydantic`

### Rust (Sandbox)
- 异步: `tokio`
- 错误: `thiserror`/`anyhow`

### gRPC
- 服务间通信
- 使用 `buf` 管理 Proto

## Multi-Agent Architecture

### 核心设计理念

**MainAgent 是中央编排者**，负责：
- 接收用户请求
- 分解任务
- 编排 Subagent 协作
- 汇总结果
- 持久化 Agent

**Subagent 各司其职**，每个 Agent 有独立的：
- 职责边界
- 工具集
- Prompt 模板
- 状态管理

### Agent 层级结构

```
┌─────────────────────────────────────────────────────────────┐
│                        MainAgent                             │
│  (中央编排者 - 任务分解、调度、监控、汇总)                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  Researcher  │  │   Curator    │  │   Indexer    │       │
│  │   Agent      │  │   Agent      │  │   Agent      │       │
│  │              │  │              │  │              │       │
│  │ • 网页搜索   │  │ • 信息整理   │  │ • 文本切分   │       │
│  │ • 内容抓取   │  │ • 质量筛选   │  │ • 向量化     │       │
│  │ • 数据提取   │  │ • 知识图谱   │  │ • RAG索引    │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐                         │
│  │ Guide Agent  │  │Planner Agent │                         │
│  │              │  │              │                         │
│  │ • 景点讲解   │  │ • 行程规划   │                         │
│  │ • RAG查询    │  │ • 时间优化   │                         │
│  │ • 位置服务   │  │ • 预算估算   │                         │
│  └──────────────┘  └──────────────┘                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 目的地 Agent 创建流程 (核心流程)

```
用户请求: "帮我创建一个京都的导游 Agent"
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ MainAgent 接收请求                                           │
│ • 解析目标: destination="京都", theme="cultural"             │
│ • 创建任务计划                                                │
│ • 初始化进度追踪                                              │
└─────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 1: 调用 Researcher Agent                                │
│ ─────────────────────────────                                │
│ 工具: web_search, web_crawler, content_extractor            │
│ 任务:                                                        │
│   1. 搜索京都旅游景点、历史文化、美食等信息                    │
│   2. 爬取权威旅游网站内容                                     │
│   3. 提取结构化数据 (景点名、地址、简介、开放时间等)           │
│ 输出: raw_documents[] (原始文档集合)                          │
└─────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 2: 调用 Curator Agent                                   │
│ ─────────────────────────                                    │
│ 工具: llm_summarize, knowledge_graph_builder                 │
│ 任务:                                                        │
│   1. 去重、清洗、质量筛选                                     │
│   2. 信息整合、摘要生成                                       │
│   3. 构建知识图谱 (景点关系、主题分类)                        │
│ 输出: curated_documents[] (整理后的文档)                      │
│       knowledge_graph (知识图谱)                             │
└─────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 3: 调用 Indexer Agent                                   │
│ ─────────────────────────                                    │
│ 工具: text_chunker, embedding_service, qdrant_client         │
│ 任务:                                                        │
│   1. 文本分块 (按语义边界切分)                                │
│   2. 生成向量 Embedding                                      │
│   3. 存入 Qdrant 向量数据库                                   │
│ 输出: collection_id (Qdrant 集合ID)                          │
│       chunk_count (分块数量)                                  │
└─────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ MainAgent 汇总 & 持久化                                       │
│ ─────────────────────                                        │
│ • 创建 DestinationAgent 记录 (PostgreSQL)                    │
│ • 关联 VectorCollectionID (Qdrant)                           │
│ • 存储原始文档 (S3/MinIO)                                    │
│ • 更新用户 Agent 列表                                         │
│ • 返回创建结果给用户                                          │
└─────────────────────────────────────────────────────────────┘
```

### 实时导游流程

```
用户进入导游模式 (加载已创建的 Kyoto Agent)
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ MainAgent 加载 Destination Agent                             │
│ • 从 PostgreSQL 读取元数据                                   │
│ • 获取 Qdrant Collection ID                                  │
│ • 创建 Guide Agent 实例 (绑定该 Collection)                  │
└─────────────────────────────────────────────────────────────┘
                    │
        ┌───────────┼───────────┐
        ▼           ▼           ▼
   ┌─────────┐ ┌─────────┐ ┌─────────┐
   │ 位置更新 │ │ 拍照识别 │ │ 文字提问 │
   └─────────┘ └─────────┘ └─────────┘
        │           │           │
        ▼           ▼           ▼
   ┌─────────────────────────────────────┐
   │         Guide Agent 处理             │
   │ • RAG 查询 (Qdrant Vector Search)   │
   │ • LLM 生成回复 (带上下文)            │
   │ • 流式返回讲解内容                   │
   └─────────────────────────────────────┘
```

### Subagent 职责详解

| Agent | 职责 | 工具 | 输入 | 输出 |
|-------|------|------|------|------|
| **Researcher** | 信息搜集 | web_search, crawler, extractor | 目的地名称 | raw_documents[] |
| **Curator** | 内容整理 | llm, knowledge_graph | raw_documents | curated_docs, graph |
| **Indexer** | 向量索引 | chunker, embedding, qdrant | curated_docs | collection_id |
| **Guide** | 实时讲解 | rag_query, location | question, location | 讲解内容 |
| **Planner** | 行程规划 | llm, constraint_solver | 偏好、时间、预算 | itinerary |

### Agent 通信机制

```
Go (Orchestration)          Python (Services)
     │                            │
     │  gRPC                      │
     ├──────────────────────────► │
     │  CreateDestinationRequest  │
     │                            │
     │  ◄──────────────────────────┤
     │  StreamProgress (SSE)      │
     │                            │
     │  gRPC: LLM/Embedding/Vision│
     ├──────────────────────────► │
```

### 任务编排模式

MainAgent 使用 **任务链模式** 编排 Subagent：

```go
// 伪代码
func (a *MainAgent) CreateDestinationAgent(ctx context.Context, destination string) error {
    // 1. 创建任务计划
    plan := a.createPlan("create_destination_agent")

    // 2. 顺序执行 Subagent 任务
    rawDocs := a.researcher.Search(ctx, destination)
    a.reportProgress("research_complete", len(rawDocs))

    curatedDocs := a.curator.Curate(ctx, rawDocs)
    a.reportProgress("curation_complete", len(curatedDocs))

    collectionID := a.indexer.Index(ctx, curatedDocs)
    a.reportProgress("indexing_complete", collectionID)

    // 3. 持久化
    a.persistence.SaveDestinationAgent(destination, collectionID)

    return nil
}
```

### 进度反馈机制

创建过程通过 SSE 实时反馈前端：

```
event: progress
data: {"stage": "researching", "progress": 20, "message": "正在搜索京都旅游信息..."}

event: progress
data: {"stage": "curating", "progress": 50, "message": "已找到 42 篇文档，正在整理..."}

event: progress
data: {"stage": "indexing", "progress": 80, "message": "正在构建向量索引..."}

event: complete
data: {"agent_id": "agent-123", "status": "ready"}
```

## Development Documentation

### 文档结构

```
docs/
├── architecture/        # overview.md, multi-agent.md, data-flow.md
├── guides/             # getting-started.md, frontend.md, backend-go.md
├── core-flows/         # agent-creation.md, agent-persistence.md
├── technical-challenges/
└── api/                # rest.md, grpc.md, websocket.md
```

### 文档要求

每个核心功能完成后编写:
1. 流程图 (Mermaid)
2. 关键步骤
3. 数据结构
4. 错误处理

## Testing Strategy

**分层**: E2E (Playwright) → Integration → Unit

| Layer | Framework | Coverage |
|-------|-----------|----------|
| Go | testing + testify | ≥80% |
| Python | pytest + pytest-asyncio | ≥80% |
| TypeScript | Vitest + Testing Library | ≥70% |
| E2E | Playwright | 关键流程 |

## GitHub Release Strategy

**仓库**: `github.com/utaaa/uta-travel-agent` (Public, MIT)

| Milestone | 内容 |
|-----------|------|
| v0.1.0-alpha | 项目骨架 |
| v0.2.0-alpha | 基础架构 (gRPC/API/前端框架) |
| v0.3.0-alpha | Agent创建 + RAG + 持久化 |
| v0.4.0-alpha | 导游功能 + 地图 + WebSocket |
| v0.5.0-beta | 行程规划 + 多语言 |
| v1.0.0 | 正式发布 |

### Commit 规范

```
feat/fix/docs/test/refactor/chore(scope): message
```

## Key Design Principles

1. **Agent Autonomy**: 独立职责边界和决策能力
2. **Agent Persistence**: 永久保存，随时调用
3. **Loose Coupling**: gRPC 解耦
4. **Graceful Degradation**: 部分失败不影响整体
5. **Observability**: 日志、指标、追踪
6. **Security First**: 敏感操作沙箱隔离
7. **Beautiful UX**: 精美流畅
8. **Test-Driven**: 同步测试
9. **Documentation-First**: 核心功能配套文档

## Notes

- 渐进式开发，优先核心 Agent 功能
- Agent 持久化是核心特性
- 前端体验是重点
- Rust 沙箱可选，初期可用 Python 替代
- 优先使用 Claude API
- 同步编写测试和文档

## v0.3.0-alpha 开发计划

**目标**: 实现完整的目的地 Agent 创建流程 (MainAgent 编排 Subagent)

### Phase 1: 基础设施 (Week 1)

1. **Qdrant 向量数据库集成**
   - Docker Compose 添加 Qdrant 服务
   - Go Qdrant 客户端封装
   - Collection 创建/管理 API

2. **Embedding gRPC 服务**
   - Python Embedding 服务完善
   - 支持文本 Embedding API
   - 批量 Embedding 优化

### Phase 2: Subagent 实现 (Week 2)

3. **Researcher Agent**
   - 网页搜索工具 (SerpAPI/自定义爬虫)
   - 内容提取和清洗
   - 输出结构化文档

4. **Curator Agent**
   - 文档质量评估
   - 信息去重和整合
   - 生成知识摘要

5. **Indexer Agent**
   - 文本分块策略
   - 调用 Embedding 服务
   - 写入 Qdrant 索引

### Phase 3: MainAgent 编排 (Week 3)

6. **任务编排引擎**
   - Subagent 注册和发现
   - 任务链执行
   - 错误处理和重试

7. **进度反馈系统**
   - SSE 进度推送
   - 前端进度展示
   - 创建状态持久化

### Phase 4: 前端创建流程 (Week 4)

8. **目的地创建向导**
   - Step 1: 输入目的地信息
   - Step 2: 实时进度展示
   - Step 3: 创建完成/预览

9. **Agent 管理页面**
   - Agent 列表/详情
   - 删除/更新操作
   - 快速启动导游模式

### 技术依赖

| 组件 | 用途 | 状态 |
|------|------|------|
| Qdrant | 向量存储 | 待集成 |
| Embedding Service | 文本向量化 | 待完善 |
| SerpAPI / 爬虫 | 网页搜索 | 待实现 |
| PostgreSQL | Agent 元数据 | 待集成 |
| Redis | 任务状态缓存 | 待集成 |

### 验收标准

- [ ] 用户可以输入目的地名称创建 Agent
- [ ] 创建过程有实时进度反馈
- [ ] 创建完成后可以与 Agent 对话
- [ ] Agent 回答基于 RAG 检索的知识
- [ ] Agent 数据持久化到 PostgreSQL