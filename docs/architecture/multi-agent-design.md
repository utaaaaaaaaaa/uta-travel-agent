# Multi-Agent Architecture Design

## 概述

UTA Travel Agent 的 Multi-Agent 架构，采用共享状态 + 自主决策模式。

---

## 核心架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         MainAgent                                │
│  - 协调所有 Subagent                                             │
│  - 维护 SharedKnowledgeState                                     │
│  - 决定何时进入下一阶段                                           │
└───────────────────────────┬─────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ Researcher-1  │   │ Researcher-2  │   │ Researcher-N  │
│               │   │               │   │               │
│  Read State   │   │  Read State   │   │  Read State   │
│  Think        │   │  Think        │   │  Think        │
│  Act          │   │  Act          │   │  Act          │
│  Update State │   │  Update State │   │  Update State │
└───────┬───────┘   └───────┬───────┘   └───────┬───────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            │
                    ┌───────▼───────┐
                    │ Shared State  │
                    │ (知识覆盖图)   │
                    └───────────────┘
```

---

## 共享知识状态 (SharedKnowledgeState)

### 数据结构

```go
type SharedKnowledgeState struct {
    Destination    string
    mu            sync.RWMutex

    // 已覆盖的主题及文档数
    CoveredTopics  map[string]*TopicCoverage

    // 已收集的所有文档
    Documents      []Document

    // 建议搜索的方向（由各 Agent 提出）
    SuggestedTopics []string

    // 当前活跃的 Researcher 数量
    ActiveResearchers int

    // 完成信号
    AllComplete    bool
}

type TopicCoverage struct {
    Name        string
    DocumentIDs []string
    Quality     float64 // 0-1
    LastUpdated time.Time
}

type Document struct {
    ID          string
    Title       string
    Content     string
    URL         string
    Source      string
    Topics      []string  // 该文档覆盖的主题
    Quality     float64
    CollectedBy string    // 哪个 Researcher 收集的
    CollectedAt time.Time
}
```

### 预定义主题类别

```go
var KnowledgeTopics = []string{
    // 核心主题
    "景点",        // 名胜古迹、自然景观
    "美食",        // 特色料理、餐厅推荐
    "历史文化",    // 历史背景、文化习俗
    "交通",        // 机场、铁路、市内交通

    // 扩展主题
    "住宿",        // 酒店、民宿推荐
    "娱乐",        // 夜生活、演出、活动
    "购物",        // 商场、特产、市场
    "实用信息",    // 签证、货币、通讯、安全

    // 主题特色（可选）
    "艺术",        // 博物馆、美术馆
    "自然",        // 公园、徒步路线
    "节庆",        // 传统节日、活动
}
```

---

## Research Agent 工作流程

### 初始化

```
每个 Researcher 启动时，MainAgent 随机分配一个初始搜索方向：
- Researcher-1: "景点"
- Researcher-2: "历史文化"
- Researcher-3: "美食"
- Researcher-4: "交通"
```

### 每轮迭代

```
┌─────────────────────────────────────────────────────────────┐
│ Round N                                                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. READ: 读取 SharedKnowledgeState                          │
│     - 已覆盖哪些主题？                                        │
│     - 各主题质量如何？                                        │
│     - 其他 Agent 在搜什么？                                   │
│                                                              │
│  2. THINK: LLM 决策                                          │
│     Prompt 包含:                                             │
│     - 当前共享状态摘要                                        │
│     - 本轮之前的搜索历史（精炼版）                             │
│     - 可用工具                                                │
│                                                              │
│     LLM 输出:                                                 │
│     {                                                        │
│       "analysis": "景点和美食已覆盖，缺少交通和住宿信息",      │
│       "decision": "搜索交通信息",                             │
│       "query": "XX 交通 机场 地铁",                           │
│       "confidence": 0.85                                     │
│     }                                                        │
│                                                              │
│  3. ACT: 执行搜索                                             │
│     - 调用 wikipedia_search / baidu_baike_search             │
│     - 可选调用 web_reader 深入阅读                            │
│                                                              │
│  4. OBSERVE: 分析结果                                         │
│     - 提取新文档                                              │
│     - 判断文档覆盖的主题                                       │
│     - 评估文档质量                                            │
│                                                              │
│  5. UPDATE: 更新共享状态                                      │
│     - 添加新文档                                              │
│     - 更新主题覆盖                                            │
│     - 记录本轮总结（精炼后）                                   │
│                                                              │
│  6. CHECK: 检查是否完成                                       │
│     - 连续 2 轮无新信息？                                     │
│     - 达到轮次上限 (5轮)？                                    │
│     - 核心主题全部覆盖且质量达标？                             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 轮间总结精炼

每轮结束后，生成精炼总结传递给下一轮：

```
{
  "round": 3,
  "searched_query": "京都交通 地铁 巴士",
  "found_documents": 3,
  "covered_topics": ["交通"],
  "quality_summary": "找到了地铁线路图和巴士信息，缺少机场交通详情",
  "suggested_next": "机场交通",
  "tokens_used": 1500
}
```

---

## System Prompt 设计

### Researcher Agent

```markdown
# 旅游信息研究专家

## 当前任务
目的地: {{destination}}
初始方向: {{initial_topic}}

## 共享知识状态
已覆盖主题:
{{#each covered_topics}}
- {{name}}: {{document_count}} 篇文档，质量 {{quality}}/10
{{/each}}

缺失主题: {{missing_topics}}

## 本轮之前的工作总结
{{previous_rounds_summary}}

## 你的任务
1. 分析当前知识缺口
2. 决定本轮搜索方向
3. 执行搜索并评估结果

## 决策规则
- 优先搜索完全缺失的主题
- 如果某主题质量低于 6/10，考虑补充
- 避免重复搜索已有足够信息的主题
- 如果其他 Researcher 正在搜索某主题，选择其他方向

## 工具
- wikipedia_search: {"query": "...", "limit": 5, "fetch_content": true}
- baidu_baike_search: {"query": "...", "limit": 5, "fetch_content": true}

## 输出格式
思考后输出 JSON:
{
  "analysis": "分析当前状态",
  "decision": "决定搜索什么",
  "query": "具体搜索词",
  "tool": "wikipedia_search 或 baidu_baike_search"
}
```

---

## 通信机制

### 方案 A: 共享内存（推荐）

```go
// Researcher 直接读写共享状态
func (r *ResearcherAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
    for round := 1; round <= r.maxRounds; round++ {
        // 1. 读取共享状态
        state := r.sharedState.Read()

        // 2. LLM 思考
        decision := r.think(ctx, state, r.roundSummaries)

        // 3. 执行搜索
        result := r.act(ctx, decision)

        // 4. 更新共享状态
        r.sharedState.Update(r.id, result)

        // 5. 检查完成
        if r.shouldComplete(state, result) {
            break
        }

        // 6. 生成轮间总结
        r.roundSummaries = append(r.roundSummaries, r.summarizeRound(result))
    }
}
```

### 方案 B: A2A 消息传递（备选）

```go
type AgentMessage struct {
    From        string
    To          string// 广播用 "*"
    Type        string  // "request", "inform", "complete"
    Topic       string
    Content     any
    Timestamp   time.Time
}

// Researcher 广播自己的搜索计划
r.broadcast(AgentMessage{
    Type: "inform",
    Content: "我正在搜索交通信息",
})

// 收到其他 Agent 的消息
for msg := range r.messageChan {
    // 更新本地知识图谱
    r.updateLocalKnowledge(msg)
}
```

**推荐使用方案 A（共享内存）**：
- 实现简单
- 状态一致性好
- 性能高

---

## 完成条件

### Researcher 完成

满足任一条件：
1. 连续 2 轮未收集到新文档
2. 达到轮次上限 (5轮)
3. 所有核心主题覆盖且质量 ≥ 7/10

### 进入 Curator 阶段

当所有 Researcher 完成后：
1. 汇总所有文档
2. Curator 评估整体质量
3. 如质量不足，MainAgent 可调度补充搜索

---

## 前端进度展示

### SSE 事件格式

```json
{
  "event": "research_progress",
  "data": {
    "researchers": [
      {
        "id": "researcher-1",
        "round": 3,
        "max_rounds": 5,
        "current_topic": "交通",
        "documents_found": 4,
        "status": "searching"
      },
      {
        "id": "researcher-2",
        "round": 4,
        "max_rounds": 5,
        "current_topic": "美食",
        "documents_found": 6,
        "status": "searching"
      }
    ],
    "covered_topics": ["景点", "历史文化", "美食"],
    "missing_topics": ["交通", "住宿"],
    "total_documents": 15
  }
}
```

### 前端雷达图

```
┌─────────────────────────────────────────┐
│苏州 Agent 创建进度│
├─────────────────────────────────────────┤
│                                         │
│     景点 ●●●●● 已完成 (5篇)              │
│     美食 ●●●●○搜索中 (4篇)              │
│     历史 ●●●○○ 搜索中 (3篇)              │
│     交通 ○○○○○ 待搜索                    │
│     住宿 ○○○○○ 待搜索                    │
│                                         │
│  [Researcher-1] 正在搜索: 交通信息...   │
│  [Researcher-2] 正在搜索: 更多美食...   │
│                                         │
│  总进度: ████████░░ 65%                 │
│  已收集: 12篇文档                        │
└─────────────────────────────────────────┘
```

---

## Curator Agent 改造

### 核心职责

Curator 负责：
1. 评估所有文档的整体质量
2. 识别重复内容
3. 判断是否需要补充搜索
4. 标注高质量文档

### 工作流程

```
Curator Agent
├── 读取共享状态中的所有文档
├── 评估质量
│   ├── 每篇文档打分 (0-1)
│   ├── 识别重复内容
│   └── 判断主题覆盖完整性
├── 决策
│   ├── 质量达标 → 标记完成
│   └── 质量不足 → 请求补充搜索
└── 更新共享状态
```

### System Prompt

```markdown
# 旅游信息整理专家

## 当前任务
目的地: {{destination}}
已收集文档: {{total_documents}}篇

## 主题覆盖情况
{{#each covered_topics}}
- {{name}}: {{document_count}}篇，质量{{quality}}/10
{{/each}}

缺失主题: {{missing_topics}}

## 你的任务
1. 评估每篇文档的质量 (0-1分)
2. 识别重复或低质量内容
3. 判断知识库是否完整

## 质量评估标准
- 准确性: 信息是否具体、可验证
- 完整性: 是否覆盖主题的关键信息
- 时效性: 信息是否仍然有效
- 来源可靠性: 来源是否权威

## 输出格式
{
  "quality_score": 0.75,
  "documents_evaluated": 12,
  "high_quality_count": 8,
  "duplicates_found": 2,
  "needs_more_search": true,
  "missing_aspects": ["交通详情", "住宿推荐"],
  "is_complete": false
}
```

### 迭代机制

| 轮次 | 条件 |
|------|------|
| 最大轮次 | 3轮 |
| 终止条件 | 整体质量≥ 0.7 或 连续2轮无改进 |

---

## Indexer Agent 改造

### 核心职责

Indexer 负责：
1. 文档分块
2. 向量索引构建
3. 索引质量验证

### 工作流程

```
Indexer Agent
├── 读取 Curator 标记的高质量文档
├── 文档分块
│   ├── 按语义边界切分
│   └── 保留元数据
├── 构建向量索引
│   ├── 调用 Embedding 服务
│   └── 存入 Qdrant
└── 验证索引效果
```

### System Prompt

```markdown
# 向量索引专家

## 当前任务
目的地: {{destination}}
待索引文档: {{document_count}}篇

## 你的任务
1. 将文档按语义分块
2. 构建向量索引
3. 验证检索效果

## 分块策略
- chunk_size: 500-1000字符
- overlap: 50-100字符
- 每块包含完整语义单元

## 输出格式
{
  "collection_id": "dest_xxx",
  "total_chunks": 45,
  "embedding_dimension": 768,
  "indexing_time_ms": 1234,
  "is_complete": true
}
```

### 迭代机制

| 轮次 | 条件 |
|------|------|
| 最大轮次 | 2轮 |
| 终止条件 | 索引成功构建 |

---

## 完整流程图

```
┌─────────────────────────────────────────────────────────────────┐
│                      Agent 创建流程                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. MainAgent 初始化共享状态                                     │
│     ↓                                                            │
│  2. 启动 4 个 Researcher (并行)                                  │
│     ├── Researcher-1: 初始搜索"景点"                             │
│     ├── Researcher-2: 初始搜索"历史文化"                         │
│     ├── Researcher-3: 初始搜索"美食"                             │
│     └── Researcher-4: 初始搜索"交通"                             │
│     ↓                                                            │
│  3. 每个 Researcher 迭代 (最多5轮)                               │
│     ├── Read: 读取共享状态                                       │
│     ├── Think: LLM 决定搜索什么                                  │
│     ├── Act: 执行搜索                                            │
│     ├── Update: 更新共享状态                                     │
│     └── Check: 检查是否完成                                      │
│     ↓                                                            │
│  4. 所有 Researcher 完成后，启动 Curator                         │
│     ├── 评估文档质量                                             │
│     ├── 判断是否需要补充搜索                                      │
│     └── 如需要 → 返回步骤 2                                      │
│     ↓                                                            │
│  5. Curator 确认质量达标后，启动 Indexer                         │
│     ├── 文档分块                                                 │
│     ├── 构建向量索引                                             │
│     └── 验证索引效果                                             │
│     ↓                                                            │
│  6. 完成，Agent 状态更新为 Ready                                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 实现计划

### Phase 1: 共享状态实现 ✅
- [x] 定义 SharedKnowledgeState 结构
- [x] 实现线程安全的读写方法
- [x] 添加主题覆盖度计算

### Phase 2: Researcher 改造 ✅
- [x] 重写 Researcher Agent 使用共享状态
- [x] 实现轮间总结精炼
- [x] 添加完成条件判断
- [x] `internal/agent/researcher_agent.go` 已实现
- [x] `internal/agent/shared_state.go` 已实现
- [x] `internal/router/router.go` 已集成 Multi-Agent 架构

### Phase 3: Curator 改造 ✅
- [x] 更新 Curator System Prompt
- [x] 实现质量评估逻辑
- [x] `internal/agent/curator_agent.go` 已实现
- [x] 集成到 executeAgentCreation 流程

### Phase 4: Indexer 改造 ✅
- [x] 更新 Indexer System Prompt
- [x] 实现文档分块逻辑
- [x] `internal/agent/indexer_agent.go` 已实现
- [x] 集成到 executeAgentCreation 流程

### Phase 5: MainAgent 协调 ✅
- [x] 实现并行启动多个 Researcher
- [x] 监控所有 Researcher 状态
- [x] 触发 Curator/Indexer 阶段
- [x] SSE 实时推送进度 (步骤、进度、完成事件)

### Phase 6: 前端集成 ✅
- [x] SSE 实时接收进度
- [x] 雷达图可视化 (主题覆盖度)
- [x] Researcher 详情展示 (进度卡片)

## 待实现功能

### Curator 改造
- [ ] 更新 Curator System Prompt
- [ ] 实现质量评估逻辑
- [ ] 添加补充搜索请求机制

### Indexer 改造
- [ ] 更新 Indexer System Prompt
- [ ] 实现文档分块逻辑
- [ ] 添加索引验证

### MainAgent 协调优化
- [ ] 实现补充搜索调度
- [ ] 监控所有 Researcher 状态优化