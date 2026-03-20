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

## 当前实现状态 (v0.4.0-alpha)

### 已完成功能 ✅

| 模块 | 功能 | 文件位置 |
|------|------|----------|
| **LLMAgent** | ReAct 循环 (Think→Act→Observe) | internal/agent/llm_agent.go |
| **LLMAgent** | Memory 系统 | internal/agent/memory.go |
| **LLMAgent** | Tool 执行框架 | internal/agent/tools.go |
| **LLMAgent** | System Prompt 模板 | internal/agent/prompts.go |
| **MainAgent** | 任务编排 | internal/agent/main_agent.go |
| **Registry** | Agent 注册 + PostgreSQL 持久化 | internal/agent/registry.go |
| **Router** | REST API + SSE 流式 | internal/router/router.go |
| **RAG Service** | 向量检索 + LLM 生成 | internal/rag/service.go |
| **Qdrant** | 向量数据库客户端 | internal/storage/qdrant/ |
| **Embedding** | Python gRPC 服务 | services/embedding/ |
| **Frontend** | 创建页 + 导游页 + 任务页 | apps/web/src/app/ |
| **Docker** | 全栈部署配置 | docker-compose.yml |

### 待解决问题 🚧

详细问题见: [docs/technical-challenges/researcher-agent-issues.md](docs/technical-challenges/researcher-agent-issues.md)

#### P0 - 阻塞核心功能

| 问题 | 现状 | 影响 |
|------|------|------|
| **搜索 API 未实现** | BraveSearchTool 返回 Mock 数据 | 无法搜索真实信息 |
| **网页读取未实现** | WebReaderTool 返回 Mock 数据 | 无法获取网页内容 |
| **信息源单一** | 只从 Wikipedia 爬取 | 信息覆盖严重不足 |

#### P1 - 影响体验

| 问题 | 现状 | 影响 |
|------|------|------|
| 主题关键词未使用 | theme 参数被忽略 | 所有 Agent 收集相同信息 |
| 搜索深度不足 | max_pages=20 | 信息量不够 |
| 内容提取质量差 | 简单 BeautifulSoup | 包含大量无关文本 |
| extract_travel_info 未注册 | 工具调用失败 | 信息提取不工作 |

#### P2 - 需要清理

| 问题 | 现状 | 影响 |
|------|------|------|
| subagent.go 死代码 | 不再被使用 | 代码混乱 |
| 无信息质量评估 | 收集内容无筛选 | 知识库质量差 |
| 缺少进度反馈 | ExplorationStep 未推送 | 用户体验差 |

### Mock 数据位置 (需要替换)

```
tools.go:117-129    BraveSearchTool 返回硬编码搜索结果
tools.go:155-162    WebReaderTool 返回硬编码网页内容
router.go:519-529   generateMockChatResponse 返回硬编码回答
```

---

## v0.5.0-alpha 开发计划

**目标**: 实现真正可用的导游 Agent

详细计划: [docs/roadmap/v0.5.0-alpha.md](docs/roadmap/v0.5.0-alpha.md)

### Phase 1: 核心工具实现 (Week 1)

**目标**: 让 Agent 能搜索和读取真实信息

| 任务 | 优先级 | 状态 |
|------|--------|------|
| 集成 Brave Search API | P0 | 待开始 |
| 实现 WebReader 真实调用 (HTTP + 内容提取) | P0 | 待开始 |
| 添加环境变量配置 (BRAVE_API_KEY) | P0 | 待开始 |
| 单元测试 | P0 | 待开始 |

**验收标准**:
- [ ] 搜索工具返回真实搜索结果
- [ ] 网页读取工具返回真实网页内容
- [ ] Agent 创建时有真实的搜索日志

### Phase 2: Agent 创建流程改造 (Week 2)

**目标**: 使用真实工具创建 Agent

| 任务 | 优先级 | 状态 |
|------|--------|------|
| 移除 executeAgentCreation 中的 simulateProgress | P0 | 待开始 |
| 接入真实 LLMAgent 执行研究任务 | P0 | 待开始 |
| 实现向量索引创建 (写入 Qdrant) | P0 | 待开始 |
| 使用主题关键词进行多轮搜索 | P1 | 待开始 |
| 添加探索进度 SSE 推送 | P1 | 待开始 |

**验收标准**:
- [ ] 创建 Agent 后 Qdrant 有真实向量数据
- [ ] 创建过程显示真实搜索进度
- [ ] 不同主题的 Agent 收集不同信息

### Phase 3: RAG 导游功能 (Week 3)

**目标**: 导游基于知识库回答问题

| 任务 | 优先级 | 状态 |
|------|--------|------|
| handleAgentChat 集成 RAG Service | P0 | 待开始 |
| 实现导游角色 Prompt 模板 | P0 | 待开始 |
| 景点列表从知识库加载 | P1 | 待开始 |
| 添加来源引用显示 | P1 | 待开始 |

**验收标准**:
- [ ] 导游回答基于 RAG 检索的知识
- [ ] 回答中显示信息来源
- [ ] 景点列表从 Qdrant 加载

### Phase 4: 清理与优化 (Week 4)

**目标**: 代码清理和体验优化

| 任务 | 优先级 | 状态 |
|------|--------|------|
| 删除 subagent.go 死代码 | P2 | 待开始 |
| 删除或迁移 subagent_test.go | P2 | 待开始 |
| 实现信息质量评估 | P2 | 待开始 |
| LLM 流式输出优化 | P1 | 待开始 |
| 错误处理完善 | P1 | 待开始 |

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
├── tools.go          # 工具接口 (含 Mock)
├── prompts.go        # System Prompt 模板
├── factory.go        # Agent 工厂
├── registry.go       # Agent 注册
└── subagent.go       # ⚠️ 死代码，待删除

internal/router/
└── router.go         # REST API (含 executeAgentCreation)

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
| v0.5.0-alpha | 真实工具 + RAG 导游 (计划中) | TBD |
