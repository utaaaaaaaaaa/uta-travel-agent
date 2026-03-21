# UTA Travel Agent - Multi-Agent Tourism Assistant

## Project Vision

UTA (Universal Travel Agent) 是一个基于 Multi-Agent 架构的智能旅游助手系统。核心理念是"Vibecoding"——让技术为体验服务，通过 AI Agent 为用户提供沉浸式的旅游文化体验。

### 核心功能
1. **目的地研究 Agent**: 自动搜索、整理旅游目的地信息，构建 RAG 知识库
2. **智能导游 Agent**: 实地旅游时，基于位置/图片识别景点，提供文化背景讲解
3. **行程规划 Agent**: 根据用户偏好生成个性化旅游行程
4. **Agent 持久化**: 创建的目的地 Agent 永久保存，用户可随时调用
5. **多语言支持**: 支持多语言交流和翻译

---

## Technical Architecture

**分层架构** (自上而下):
- **Frontend (TypeScript)**: Destination Explorer | Guide Mode | User Dashboard
- **Orchestration (Go)**: Agent Scheduler | Task Router | Agent Registry | gRPC Gateway
- **Agent Services (Python)**: Destination Agent (RAG) | Guide Agent (Vision) | Planner Agent
- **Storage Layer**: PostgreSQL | Redis | Qdrant | S3/MinIO

### Technology Stack

| Layer | Language | Technology | Purpose |
|-------|----------|------------|---------|
| Frontend | TypeScript | Next.js 15 | 用户界面、Agent 管理 |
| Orchestration | Go | Gin | Agent 调度、任务路由、gRPC 网关 |
| Agent Services | Python | gRPC | LLM 调用、RAG、向量检索 |
| Vector DB | Qdrant | - | RAG 知识存储 |
| Database | PostgreSQL | - | Agent 元数据持久化 |
| LLM | DeepSeek/Claude | - | 大语言模型能力 |

---

## 当前实现状态 (v0.5.0-alpha) - ✅ 已完成

### 已完成功能 ✅

| 模块 | 功能 | 文件位置 |
|------|------|----------|
| **LLMAgent** | ReAct 循环 (Think→Act→Observe) | internal/agent/llm_agent.go |
| **LLMAgent** | Memory 系统 | internal/agent/memory.go |
| **LLMAgent** | Tool 执行框架 | internal/agent/tools.go |
| **LLMAgent** | System Prompt 模板 | internal/agent/prompts.go |
| **MainAgent** | 任务编排 + 实时搜索 | internal/agent/main_agent.go |
| **Registry** | Agent 注册 + PostgreSQL 持久化 | internal/agent/registry.go |
| **Router** | REST API + SSE 流式 + 实时搜索 | internal/router/router.go |
| **RAG Service** | 向量检索 + LLM 生成 | internal/rag/service.go |
| **Qdrant** | 向量数据库客户端 | internal/storage/qdrant/ |
| **Embedding** | Python gRPC 服务 | services/embedding/ |
| **Frontend** | 创建页 + 导游页 + 任务页 | apps/web/src/app/ |
| **Docker** | 全栈部署配置 | docker-compose.yml |
| **Search Tools** | Wikipedia + Tavily + WebReader | internal/tools/search.go |
| **Real-time Search** | 关键词触发实时搜索 | internal/router/router.go |

### v0.5.0-alpha 新增功能 ✅

| 功能 | 描述 | 状态 |
|------|------|------|
| **WikipediaSearchTool** | Wikipedia API 搜索权威知识 | ✅ 已实现 |
| **TavilySearchTool** | Tavily API 实时搜索 | ✅ 已实现 |
| **WebReaderTool** | 读取任意网页内容 | ✅ 已实现 |
| **实时搜索触发** | 关键词检测自动触发实时搜索 | ✅ 已实现 |
| **信息来源显示** | 回答中显示信息来源 | ✅ 已实现 |
| **景点列表 API** | 从知识库获取景点 | ✅ 已实现 |
| **信息质量评估** | Curator Agent 质量评分 | ✅ 已实现 |
| **死代码清理** | subagent.go 已删除 | ✅ 已完成 |

### 待优化项 🚧

| 问题 | 优先级 | 状态 |
|------|--------|------|
| 网络超时重试 | P1 | 已实现 (LLM Stream) |
| 内容缓存 | P2 | 待实现 |
| 并行研究 | P2 | 待实现 |

---

## v0.5.0-alpha 开发计划 - ✅ 已完成

**目标**: 实现真正可用的导游 Agent

详细计划: [docs/roadmap/v0.5.0-alpha.md](docs/roadmap/v0.5.0-alpha.md)

### 完成状态

| Phase | 内容 | 状态 |
|-------|------|------|
| Phase 1 | 核心工具实现 (Wikipedia, Tavily, WebReader) | ✅ 已完成 |
| Phase 2 | Agent 创建流程改造 | ✅ 已完成 |
| Phase 3 | RAG 导游功能 + 实时搜索 | ✅ 已完成 |
| Phase 4 | 清理与优化 | ✅ 已完成 |

---

## Agent 范式要求

**所有 Subagent 必须是完整的 Agent**，具备：

| 组件 | 说明 | 状态 |
|------|------|------|
| Memory | 独立记忆 (Thoughts/Actions/Results) | ✅ |
| Context | 独立上下文窗口 | ✅ |
| Prompt | 独立系统提示词 | ✅ |
| Action Flow | ReAct 循环 | ✅ |
| LLM Brain | 独立 LLM 实例 | ✅ |

当前所有 Subagent 通过 `factory.go` 创建为 `LLMAgent`，符合范式要求。

---

## 关键文件索引

```
internal/agent/
├── llm_agent.go      # ReAct 循环实现
├── main_agent.go     # 主 Agent 编排
├── tools.go          # 工具接口
├── prompts.go        # System Prompt 模板
├── factory.go        # Agent 工厂
└── registry.go       # Agent 注册

internal/tools/
└── search.go         # Wikipedia + Tavily + WebReader 工具

internal/router/
└── router.go         # REST API + 实时搜索

internal/rag/
└── service.go        # RAG 检索服务

services/embedding/   # Python gRPC 服务

apps/web/src/app/
├── destinations/     # Agent 创建页
├── guide/[id]/       # 导游聊天页
└── tasks/[id]/       # 任务进度页
```

---

## 开发规范

1. **每个功能完成后必须测试验证**
2. **不添加 Mock 数据到生产代码**
3. **工具实现优先于流程优化**
4. **保持代码整洁，及时删除死代码**

---

## 版本历史

| 版本 | 内容 | 日期 |
|------|------|------|
| v0.4.0-alpha | Docker 部署 + LLMAgent + 持久化 | 2026-03-20 |
| v0.5.0-alpha | 真实工具 + RAG 导游 + 实时搜索 + Skills 系统 | 2026-03-21 |
| v0.6.0-alpha | Session-based Agents + 多会话前端 + 上下文工程 | 计划中 |

---

## v0.6.0-alpha 开发计划 - 🚧 计划中

**目标**: Session-based Agent 架构 + ChatGPT 风格多会话前端

详细计划: [docs/roadmap/v0.6.0-alpha.md](docs/roadmap/v0.6.0-alpha.md)

### 核心改进

| 功能 | 描述 |
|------|------|
| **Session-based Agents** | MainAgent 和 GuideAgent 支持持久化会话 |
| **多会话前端** | ChatGPT 风格的侧边栏 + 多会话管理 |
| **上下文工程** | 自动 Token 管理 + 上下文压缩 |
| **长期记忆** | PostgreSQL 持久化 + 向量索引 |

### 架构设计

详见: [docs/architecture/agent-refactoring-design.md](docs/architecture/agent-refactoring-design.md)
