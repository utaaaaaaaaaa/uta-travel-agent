# Researcher Agent 问题清单

> 文档创建时间: 2026-03-20
> 状态: 待修复
> 优先级: P0 (阻塞核心功能)

## 概述

Researcher Agent 是 UTA Travel Agent 多 Agent 架构中的关键组件，负责为目的地 Agent 收集旅游信息。当前实现存在严重缺陷，无法收集足够的信息支撑 RAG 系统的"只使用提供的上下文回答"原则。

---

## 一、致命问题 (P0 - 阻塞)

### 1.1 搜索 API 未实现

**位置**: `internal/agent/tools.go:109-130`

**现状**:
```go
func (t *BraveSearchTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
    // TODO: Implement actual Brave Search API call
    // For now, return mock results
    return &ToolResult{
        Success: true,
        Data: map[string]any{
            "query":   query,
            "results": []any{...},  // 硬编码的 mock 数据
        },
    }, nil
}
```

**问题**:
- BraveSearchTool 只返回 mock 数据
- 没有实际调用任何搜索 API
- LLMAgent (当前使用的 Subagent) 调用此工具时只能得到假数据

**影响**:
- 用户创建 Agent 后，知识库为空或只有 mock 数据
- RAG 检索无法返回有意义的结果
- LLM 自主探索时得到错误信息

**解决方案**:
1. 集成 Brave Search API (推荐，免费额度充足)
2. 或集成 SerpAPI / Google Custom Search
3. 或使用 Tavily API (专为 AI Agent 设计)

---

### 1.2 网页读取工具未实现

**位置**: `internal/agent/tools.go:148-163`

**现状**:
```go
func (t *WebReaderTool) Execute(ctx context.Context, params map[string]any) (*ToolResult, error) {
    // TODO: Implement actual MCP call
    return &ToolResult{
        Success: true,
        Data: map[string]any{
            "url":     url,
            "content": "京都，日本的文化古都...",  // 硬编码
        },
    }, nil
}
```

**问题**:
- WebReaderTool 只返回 mock 数据
- MCP 客户端未实现
- 无法读取网页真实内容

**影响**:
- 即使搜索 API 返回 URL，也无法获取内容
- 整个信息收集流程断裂

**解决方案**:
1. 实现 HTTP 请求读取网页
2. 集成 trafilatura 或 readability 进行内容提取
3. 或通过 MCP 服务调用 (如 puppeteer-server)

---

### 1.3 Python 实现信息源单一

**位置**: `services/destination-agent/src/agent.py:319-333`

**现状**:
```python
def _get_start_urls(self, destination: str, theme: str) -> list[str]:
    encoded_dest = destination.replace(" ", "_")

    urls = [
        f"https://zh.wikipedia.org/wiki/{encoded_dest}",
        f"https://en.wikipedia.org/wiki/{encoded_dest}",
    ]

    # Add travel site URLs (example structure)
    # urls.append(f"https://www.tripadvisor.com/Search?q={destination}")

    return urls
```

**问题**:
- 只从 Wikipedia 爬取
- 旅游网站 URL 被注释掉
- 没有真正的搜索 API 集成

**影响**:
- 信息覆盖严重不足
- 只能回答基本的历史/地理问题
- 无法回答实用性问题 (门票、交通、美食等)

**解决方案**:
1. 集成搜索 API
2. 添加 TripAdvisor、Lonely Planet 等旅游网站
3. 添加官方旅游网站

---

## 二、中等问题 (P1 - 影响体验)

### 2.1 主题关键词构建但未使用

**位置**: `services/destination-agent/src/agent.py:303-317`

**现状**:
```python
def _build_search_queries(self, destination: str, theme: str) -> list[str]:
    theme_keywords = {
        "cultural": ["历史", "文化", "寺庙", "博物馆", "古迹"],
        "food": ["美食", "餐厅", "小吃", "料理"],
        "adventure": ["户外", "徒步", "探险", "自然"],
        "art": ["美术馆", "艺术", "画廊", "设计"],
    }

    keywords = theme_keywords.get(theme, [])
    queries = [f"{destination} {kw}" for kw in keywords]
    # ...
    return queries  # 构建了但从未被使用！
```

**问题**:
- `_build_search_queries` 方法构建了主题相关的搜索关键词
- 但 `_research_destination` 从未调用此方法
- 用户选择的 theme 参数被忽略

**影响**:
- 所有 Agent 都收集相同类型的信息
- 无法实现主题差异化 (文化/美食/探险/艺术)

**解决方案**:
```python
async def _research_destination(self, destination: str, theme: str):
    # 使用主题关键词进行多轮搜索
    queries = self._build_search_queries(destination, theme)
    for query in queries:
        results = await self.search_api.search(query)
        # ...
```

---

### 2.2 搜索深度不足

**现状**:
```python
results = await self._crawler.crawl(
    start_urls=start_urls,
    max_pages=20,          # 最多 20 页
    allowed_domains=None,  # 允许所有域名
)
```

**问题**:
- 只爬取起始 URL 的链接页面
- 没有递归深度搜索
- max_pages=20 对于一个目的地来说太少

**影响**:
- 信息量不足以覆盖所有方面
- 错过很多有价值的信息源

**解决方案**:
1. 增加 max_pages (如 50-100)
2. 实现递归搜索 (跟随链接深度)
3. 智能选择要跟随的链接

---

### 2.3 内容提取质量差

**位置**: `services/destination-agent/src/crawler/__init__.py:117-128`

**现状**:
```python
def _extract_content(self, soup: BeautifulSoup) -> str:
    # Remove scripts, styles, navigation
    for element in soup.find_all(["script", "style", "nav", "header", "footer"]):
        element.decompose()

    # Get text content
    text = soup.get_text(separator="\n", strip=True)
    # ...
```

**问题**:
- 使用简单的 BeautifulSoup 提取
- 没有提取主要内容区域
- 包含大量无关文本 (导航、广告、侧边栏)
- 没有结构化信息提取

**影响**:
- 文档内容杂乱，影响 RAG 检索质量
- 重要信息被稀释
- 用户可能得到无关内容

**解决方案**:
1. 使用 trafilatura 进行内容提取
2. 或使用 readability-lxml
3. 添加结构化提取 (景点名称、地址、价格等)

---

### 2.4 extract_travel_info 工具未注册

**位置**: `internal/agent/subagent.go:74-83`

**现状**:
```go
// Step 3: Extract travel information using skill
extractResult, err := a.ExecuteTool(ctx, "extract_travel_info", map[string]any{
    "documents": documents,
    "goal":      goal,
})
```

**问题**:
- `extract_travel_info` 工具从未注册
- 调用会返回 "tool not found" 错误
- 代码分支 `if err != nil` 会使用原始文档，但没有提示用户

**影响**:
- 信息提取流程不完整
- 用户不知道发生了降级

**解决方案**:
1. 实现 LLMSummarizeTool 或 ExtractTravelInfoTool
2. 注册到 ToolRegistry
3. 或移除此步骤，直接传递原始文档

---

## 三、架构问题 (P2 - 需要清理)

### 3.1 ✅ 已解决: Subagent 现在使用 LLMAgent

**位置**: `internal/agent/factory.go:58-60`

**现状** (已正确实现):
```go
case AgentTypeResearcher, AgentTypeCurator, AgentTypeIndexer, AgentTypeGuide, AgentTypePlanner:
    // Create LLM-powered subagent
    agent = f.createLLMSubagent(id, agentType, template)
```

**说明**:
- ✅ 所有 Subagent 现在通过 `createLLMSubagent()` 创建为 `LLMAgent`
- ✅ 具备完整的 Agent 能力: Memory, Context, Prompt, ReAct Action Flow, LLM Brain
- ✅ 符合 CLAUDE.md 的 Agent 范式要求

**遗留问题**: `subagent.go` 是死代码

### 3.1.1 死代码清理: subagent.go 应该删除

**位置**: `internal/agent/subagent.go`

**问题**:
- `subagent.go` 定义了 ResearcherAgent, CuratorAgent, IndexerAgent, GuideAgent, PlannerAgent
- 但 `factory.go` **不再使用**这些类型
- 只有 `subagent_test.go` 还在测试这些旧实现

**引用分析**:
```
生产代码引用: 无
测试代码引用: subagent_test.go (约 300 行测试)
```

**解决方案**:
1. 删除 `internal/agent/subagent.go`
2. 删除 `internal/agent/subagent_test.go` (或迁移为 LLMAgent 测试)
3. 或保留 `subagent.go` 中的 `extractURLs` 辅助函数到其他位置

---

### 3.2 没有信息质量评估

**问题**:
- LLMAgent 收集的信息没有质量打分
- 没有去重机制
- 没有过时信息检测

**影响**:
- 知识库中可能有大量低质量内容
- RAG 可能返回错误或过时信息

**解决方案**:
1. 在 Curator Agent 的 System Prompt 中强调质量评估
2. 实现内容去重 (simhash / embedding 相似度)
3. 标注信息时效性

---

### 3.3 缺少进度反馈机制

**问题**:
- LLMAgent 有 `ExplorationStep` 记录探索过程
- 但没有实时推送到前端
- 搜索/爬取失败没有明确提示

**现状** (`llm_agent.go:158-167`):
```go
explorationStep := ExplorationStep{
    Timestamp:  time.Now(),
    Direction:  a.inferDirection(decision.Thought),
    Thought:    decision.Thought,
    Action:     decision.Action,
    ToolName:   decision.ToolName,
    // ...
}
// 记录到内存，但没有推送到前端
a.explorationLog = append(a.explorationLog, explorationStep)
```

**影响**:
- 用户体验差
- 无法排查问题

**解决方案**:
1. 通过 SSE 推送 ExplorationStep
2. 前端显示探索雷达图
3. 添加详细的错误日志

---

## 四、信息覆盖分析

### 4.1 当前能收集的信息

| 信息类型 | 能否收集 | 来源 |
|----------|----------|------|
| 基本历史背景 | ✅ | Wikipedia |
| 主要景点简介 | ✅ | Wikipedia |
| 地理位置气候 | ✅ | Wikipedia |

### 4.2 当前无法收集的信息

| 信息类型 | 能否收集 | 原因 |
|----------|----------|------|
| 门票价格 | ❌ | 无实时数据源 |
| 开放时间 | ❌ | 无实时数据源 |
| 交通指南 | ❌ | 无交通网站数据 |
| 美食推荐 | ❌ | 无点评网站数据 |
| 住宿信息 | ❌ | 无酒店网站数据 |
| 游客评价 | ❌ | 无评论数据 |
| 节庆活动 | ❌ | 无活动日历数据 |
| 小众景点 | ❌ | 无深度内容源 |

### 4.3 用户问题测试矩阵

| 用户问题 | 能否回答 | 说明 |
|----------|----------|------|
| "金阁寺的历史是什么？" | ✅ | Wikipedia 有详细介绍 |
| "金阁寺门票多少钱？开放时间？" | ❌ | 缺少实时信息 |
| "从关西机场到京都市区怎么走？" | ❌ | 缺少交通信息 |
| "推荐几家京都料理店" | ❌ | 缺少美食信息 |
| "京都赏樱最佳时间？" | ⚠️ | 只有大概信息 |
| "京都住宿推荐？" | ❌ | 缺少住宿信息 |
| "清水寺附近有什么？" | ⚠️ | 缺少距离/关联信息 |

---

## 五、当前 LLMAgent 实现状态

### 5.1 架构已符合 Agent 范式

**位置**: `internal/agent/llm_agent.go`

```
┌─────────────────────────────────────────────────────────────┐
│                      LLMAgent                               │
├─────────────────────────────────────────────────────────────┤
│ ✅ Memory        AgentMemory (Thoughts/Actions/Results)    │
│ ✅ Context       []llm.Message (对话历史)                   │
│ ✅ Prompt        GetSubagentPrompt(agentType)              │
│ ✅ Action Flow   ReAct 循环 (thinkAndDecide)               │
│ ✅ LLM Brain     llm.Provider                              │
│ ✅ Exploration   ExplorationStep[] (探索追踪)              │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 ReAct 循环实现

```go
// llm_agent.go:133-233
for iteration < a.maxIterations {
    // 1. LLM 思考并决定行动
    decision, tokensIn, tokensOut, err := a.thinkAndDecide(ctx)

    // 2. 记录思考
    a.memory.AddThought(decision.Thought)

    // 3. 执行工具 (如果有)
    if decision.ToolName != "" {
        result, err := a.tools.Execute(ctx, decision.ToolName, params)
        a.memory.AddObservation(resultStr, decision.ToolName)
    }

    // 4. 检查是否完成
    if decision.IsComplete {
        return result
    }
}
```

### 5.3 工具依赖问题

LLMAgent 的能力完全依赖注册的工具，但核心工具未实现：

| 工具名 | 状态 | 影响 |
|--------|------|------|
| `brave_search` | ❌ Mock | 无法搜索真实信息 |
| `web_reader` | ❌ Mock | 无法读取网页 |
| `extract_travel_info` | ❌ 未注册 | 信息提取不工作 |
| `build_knowledge_base` | ❌ Mock | 知识库构建不工作 |
| `build_knowledge_index` | ⚠️ 部分 | 需要真实 Qdrant 连接 |
| `rag_query` | ⚠️ 部分 | 需要真实 Qdrant 连接 |

---

## 六、修复计划

### Phase 1: 核心功能修复 (Week 1)

| 任务 | 优先级 | 预计工时 | 负责人 |
|------|--------|----------|--------|
| 集成 Brave Search API | P0 | 4h | - |
| 实现 WebReader 真实调用 | P0 | 4h | - |
| 添加环境变量配置 (API Keys) | P0 | 1h | - |
| 单元测试 | P0 | 2h | - |

### Phase 2: 信息源扩展 (Week 2)

| 任务 | 优先级 | 预计工时 | 负责人 |
|------|--------|----------|--------|
| 使用主题关键词进行多轮搜索 | P1 | 2h | - |
| 添加 TripAdvisor 爬虫 | P1 | 4h | - |
| 添加官方旅游网站爬虫 | P1 | 2h | - |
| 实现 trafilatura 内容提取 | P1 | 3h | - |

### Phase 3: 架构清理 (Week 3)

| 任务 | 优先级 | 预计工时 | 负责人 |
|------|--------|----------|--------|
| 删除 subagent.go 死代码 | P2 | 0.5h | - |
| 删除或迁移 subagent_test.go | P2 | 1h | - |
| 添加信息质量评估逻辑 | P2 | 3h | - |
| 实现探索进度 SSE 推送 | P2 | 2h | - |

---

## 七、技术债务记录

### 6.1 代码层面的 TODO

```
tools.go:115        // TODO: Implement actual Brave Search API call
tools.go:154        // TODO: Implement actual MCP call
crawler/__init__.py:95  # TODO: Use trafilatura for better extraction
agent.py:285        # For MVP, uses curated URLs. In production, would use search API.
agent.py:331        # Add travel site URLs (example structure)
```

### 6.2 Mock 数据位置

```
tools.go:117-129    BraveSearchTool 返回硬编码搜索结果
tools.go:155-162    WebReaderTool 返回硬编码网页内容
router.go:519-529   generateMockChatResponse 返回硬编码回答
```

### 6.3 注释掉的代码

```
agent.py:331        # urls.append(f"https://www.tripadvisor.com/Search?q={destination}")
```

---

## 七、参考资料

### 7.1 搜索 API 选项

| API | 免费额度 | 特点 |
|-----|----------|------|
| Brave Search API | 2000 次/月 | 免费，无需信用卡 |
| SerpAPI | 100 次/月 | Google 结果，结构化 |
| Tavily | 1000 次/月 | 专为 AI Agent 设计 |
| Google Custom Search | 100 次/天 | 需要 Google Cloud |

### 7.2 内容提取库

| 库 | 语言 | 特点 |
|----|------|------|
| trafilatura | Python | 高质量正文提取 |
| readability-lxml | Python | Mozilla 风格提取 |
| newspaper3k | Python | 新闻文章提取 |
| goose3 | Python | 文章提取 |

---

## 八、变更日志

| 日期 | 变更内容 | 作者 |
|------|----------|------|
| 2026-03-20 | 创建文档，记录所有问题 | Claude |