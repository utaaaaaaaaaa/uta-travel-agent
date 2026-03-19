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

## Multi-Agent Workflow

### 目的地Agent创建流程

```
用户请求 → Orchestrator (解析/分配/监控/持久化)
         → Destination Agent
            → Researcher (搜索/提取)
            → Curator (整理/构建知识图谱)
            → Indexer (向量化/RAG索引)
         → 保存到 PostgreSQL + Qdrant
```

### 实时导游流程

```
加载Agent (PostgreSQL元数据 + Qdrant索引)
→ WebSocket 实时交互
   - 位置更新 → 查询景点 → RAG → 推送讲解
   - 拍照识别 → 图像分析 → RAG → 文化讲解
   - 提问 → RAG → LLM → 回复
→ 会话保存 (Redis/PostgreSQL)
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