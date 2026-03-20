# Phase 3 技术文档

## 概述

Phase 3 实现了 Subagent 到真正 Agent 的升级：
- 使用 LLMAgent 替代硬编码 Subagent
- 定义专业的 System Prompts
- 实现 ReAct 循环（Think → Act → Observe）
- 探索追踪机制（用于雷达图可视化）

## LLMAgent 架构

### 核心组件

```
┌─────────────────────────────────────────────────────────────┐
│                      LLMAgent                                │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                    LLM Brain                        │    │
│  │   • 接收观察 → 思考 → 决策下一步行动                  │    │
│  │   • 自主判断任务是否完成                             │    │
│  │   • 可以"自由探索"寻找目标                           │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                   System Prompt                      │    │
│  │   • 角色定义 (你是谁)                                │    │
│  │   • 职责边界 (你要做什么)                            │    │
│  │   • 行为规范 (你怎么做)                              │    │
│  │   • 输出格式 (你输出什么)                            │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                     Memory                           │    │
│  │   • Thoughts: 思考过程                               │    │
│  │   • Actions: 执行的动作                              │    │
│  │   • Observations: 观察到的结果                       │    │
│  │   • Results: 任务结果                                │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                  Action Flow (ReAct)                │    │
│  │                                                      │    │
│  │   ┌──────────┐    ┌──────────┐    ┌──────────┐      │    │
│  │   │  Think   │ ─► │   Act    │ ─► │ Observe  │ ─┐   │    │
│  │   │  思考    │    │   行动   │    │   观察   │   │   │    │
│  │   └──────────┘    └──────────┘    └──────────┘   │   │    │
│  │         ▲                                         │   │    │
│  │         └─────────────────────────────────────────┘   │    │
│  │                                                      │    │
│  │   循环直到: 任务完成 / 达到最大步数 / 遇到错误        │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                    Tools                             │    │
│  │   brave_search │ web_reader │ llm_summarize │ ...   │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 数据结构

```go
// LLMAgent 配置
type LLMAgentConfig struct {
    ID            string
    AgentType     AgentType
    LLMProvider   llm.Provider
    SystemPrompt  string
    Tools         ToolRegistry
    MaxIterations int
}

// 探索步骤（用于雷达图可视化）
type ExplorationStep struct {
    Timestamp  time.Time `json:"timestamp"`
    Direction  string    `json:"direction"`  // 景点/美食/文化/交通/住宿/购物
    Thought    string    `json:"thought"`    // Agent 的思考
    Action     string    `json:"action"`     // 执行的动作
    ToolName   string    `json:"tool_name"`  // 使用的工具
    ToolArgs   map[string]any `json:"tool_args,omitempty"`
    Result     string    `json:"result"`     // 结果摘要
    TokensIn   int       `json:"tokens_in"`  // 输入 tokens
    TokensOut  int       `json:"tokens_out"` // 输出 tokens
    DurationMs int64     `json:"duration_ms"`
    Success    bool      `json:"success"`
}

// Agent 决策
type AgentDecision struct {
    Thought     string         `json:"thought"`
    Action      string         `json:"action"`
    ToolName    string         `json:"tool_name,omitempty"`
    ToolParams  map[string]any `json:"tool_params,omitempty"`
    IsComplete  bool           `json:"is_complete"`
    Result      string         `json:"result,omitempty"`
}
```

## ReAct 循环实现

### Run 方法流程

```go
func (a *LLMAgent) Run(ctx context.Context, goal string) (*AgentResult, error) {
    // 1. 初始化上下文
    a.context = append(a.context, llm.Message{
        Role:    "user",
        Content: fmt.Sprintf("请完成以下任务:\n\n%s", goal),
    })

    // 2. ReAct 循环
    for iteration < a.maxIterations {
        // 2.1 LLM 思考并决策
        decision, tokensIn, tokensOut, err := a.thinkAndDecide(ctx)

        // 2.2 记录思考到 Memory
        a.memory.AddThought(decision.Thought)

        // 2.3 执行工具（如果有）
        if decision.ToolName != "" {
            result, err := a.tools.Execute(ctx, decision.ToolName, params)
            a.memory.AddObservation(resultStr, decision.ToolName)
        }

        // 2.4 记录探索步骤
        a.explorationLog = append(a.explorationLog, explorationStep)

        // 2.5 检查任务是否完成
        if decision.IsComplete {
            return &AgentResult{Success: true, Output: {...}}
        }
    }
}
```

### LLM 决策过程

```go
func (a *LLMAgent) thinkAndDecide(ctx context.Context) (*AgentDecision, int, int, error) {
    // 1. 构建系统提示词（包含可用工具列表）
    systemPrompt := a.systemPrompt + toolsDesc + `
## 响应格式
你必须以 JSON 格式响应:
{
  "thought": "你的思考过程",
  "action": "你的行动描述",
  "tool_name": "要使用的工具名(可选)",
  "tool_args": {"参数名": "参数值"},
  "is_complete": false,
  "result": "任务完成时的总结"
}`

    // 2. 调用 LLM
    response, err := a.llmProvider.CompleteWithSystem(ctx, systemPrompt, a.context)

    // 3. 解析 JSON 响应
    decision, err := a.parseDecision(response.Content)

    return decision, response.InputTokens, response.OutputTokens, nil
}
```

## System Prompts 设计

### ResearcherAgent Prompt

```
# 旅游信息研究专家

## 角色定义
你是一个专业的旅游信息研究专家。你的任务是为指定的旅游目的地收集全面、准确、有价值的信息。

## 核心职责
1. 信息搜索: 使用搜索工具查找目的地的各类旅游信息
2. 内容筛选: 评估搜索结果的相关性和质量
3. 深度探索: 访问有价值的网页，提取详细信息
4. 信息整合: 将收集的信息整理成结构化的文档

## 工作流程
1. 首先思考需要搜索哪些方向的信息（景点、美食、文化、交通、住宿等）
2. 使用 brave_search 工具搜索相关信息
3. 分析搜索结果，选择最有价值的网页
4. 使用 web_reader 工具阅读详细内容
5. 提取关键信息并整理

## 可用工具
- brave_search: 搜索网络信息
- web_reader: 读取网页内容

## 信息收集指南
必须覆盖的方向: 景点、美食、文化、交通、住宿、购物
```

### CuratorAgent Prompt

```
# 旅游信息整理专家

## 核心职责
1. 信息分类: 将文档按主题分类
2. 去重处理: 识别并合并重复的信息
3. 质量评估: 评估信息的可靠性和实用性
4. 知识构建: 构建知识图谱

## 分类标准
景点、美食、住宿、交通、文化、购物、实用信息

## 可用工具
- llm_summarize: 生成摘要
- build_knowledge_base: 构建知识库
```

### IndexerAgent Prompt

```
# 向量索引专家

## 核心职责
1. 文本分块: 将文档按语义边界切分
2. 分块优化: 确保每个分块包含完整的语义单元
3. 索引构建: 调用 Embedding 服务构建向量索引
4. 质量验证: 验证索引的检索效果

## 可用工具
- text_chunker: 文本分块
- build_knowledge_index: 构建知识索引

## 分块策略
- chunk_size: 500-1000 字符
- overlap: 50-100 字符重叠
```

### GuideAgent Prompt

```
# 智能导游专家

## 核心职责
1. 知识检索: 从向量数据库检索相关的旅游知识
2. 智能讲解: 生成生动、有趣、有深度的讲解内容
3. 实时互动: 回答游客的各种问题
4. 个性化服务: 根据游客偏好调整讲解风格

## 可用工具
- rag_query: RAG 检索工具

## 讲解风格
- 亲切友好，像当地朋友一样
- 专业但不枯燥，生动有趣
- 适当使用比喻和故事
- 融入当地文化和历史背景
```

### PlannerAgent Prompt

```
# 旅游行程规划专家

## 核心职责
1. 需求分析: 理解游客的偏好、时间和预算
2. 景点筛选: 选择最适合的景点和活动
3. 路线规划: 设计高效的游览路线
4. 时间安排: 合理分配每天的行程
5. 预算估算: 提供费用参考

## 可用工具
- rag_query: 检索景点信息
- itinerary_planner: 行程规划工具

## 规划原则
- 每天主要景点 2-3 个
- 预留用餐和休息时间
- 考虑交通时间
- 安排弹性时间
```

## 工厂模式更新

### 创建 LLMAgent Subagent

```go
func (f *AgentFactory) createLLMSubagent(id string, agentType AgentType, template *AgentTemplate) *LLMAgent {
    // 获取对应类型的 System Prompt
    systemPrompt := GetSubagentPrompt(agentType)

    // 从模板获取配置，或使用默认值
    maxIterations := 10
    if template != nil && template.Spec.Decision.MaxIterations > 0 {
        maxIterations = template.Spec.Decision.MaxIterations
    }

    config := LLMAgentConfig{
        ID:            id,
        AgentType:     agentType,
        LLMProvider:   f.llmProvider,
        SystemPrompt:  systemPrompt,
        Tools:         f.toolRegistry,
        MaxIterations: maxIterations,
    }

    return NewLLMAgent(config)
}
```

### 主入口创建

```go
// 工厂创建 Agent 时自动使用 LLMAgent
func (f *AgentFactory) CreateAgentFromTemplate(agentType AgentType, template *AgentTemplate) (Agent, error) {
    switch agentType {
    case AgentTypeMain:
        return NewMainAgent(MainAgentConfig{...}), nil
    case AgentTypeResearcher, AgentTypeCurator, AgentTypeIndexer, AgentTypeGuide, AgentTypePlanner:
        return f.createLLMSubagent(id, agentType, template), nil
    }
}
```

## 探索追踪机制

### 方向推断

```go
func (a *LLMAgent) inferDirection(thought string) string {
    directions := map[string][]string{
        "景点": {"景点", "景观", "名胜", "寺庙", "神社", "公园", "temple", "shrine"},
        "美食": {"美食", "料理", "餐厅", "小吃", "抹茶", "寿司", "food", "cuisine"},
        "文化": {"文化", "历史", "传统", "艺术", "culture", "history"},
        "交通": {"交通", "地铁", "巴士", "车站", "transport", "train"},
        "住宿": {"住宿", "酒店", "民宿", "hotel", "ryokan"},
        "购物": {"购物", "商店", "市场", "shopping", "market"},
    }

    for direction, keywords := range directions {
        for _, keyword := range keywords {
            if strings.Contains(thought, keyword) {
                return direction
            }
        }
    }

    return "综合"
}
```

### 前端展示

探索日志用于前端雷达图和进度展示：

```typescript
interface ExplorationStep {
  timestamp: string;
  direction: string;    // 雷达图维度
  thought: string;      // 进度展示
  tool_name: string;
  result: string;
  tokens_in: number;
  tokens_out: number;
}

// 前端根据 direction 统计各方向探索次数
// 生成雷达图数据
```

## 测试覆盖

### 测试用例

| 测试 | 说明 |
|------|------|
| TestLLMAgentCreation | 创建 LLMAgent |
| TestLLMAgentReActLoop | ReAct 循环测试 |
| TestLLMAgentMaxIterations | 最大迭代次数测试 |
| TestLLMAgentToolExecutionFailure | 工具执行失败处理 |
| TestLLMAgentDirectionInference | 方向推断测试 |
| TestLLMAgentMemoryTracking | Memory 追踪测试 |
| TestLLMAgentJSONParsing | JSON 解析测试 |
| TestSubagentPromptRetrieval | Prompt 获取测试 |
| TestFactoryCreatesLLMSubagent | 工厂创建测试 |
| TestFactoryCreatesAllSubagentTypes | 所有类型创建测试 |
| TestMainAgentWithLLMSubagents | MainAgent 集成测试 |
| TestExplorationLog | 探索日志测试 |

### 运行测试

```bash
GO111MODULE=on go test ./internal/agent/... -v
```

## 与 Phase 2 的对比

| 特性 | Phase 2 | Phase 3 |
|------|---------|---------|
| Subagent 实现 | 硬编码工作流 | LLMAgent + ReAct |
| 决策方式 | 固定步骤 | LLM 自主决策 |
| 探索能力 | 无（按固定路径） | 有（自主探索方向） |
| Prompt | 简单角色定义 | 专业 Prompt 模板 |
| 追踪 | 无 | 探索日志 + 雷达图 |
| 工具使用 | 硬编码调用 | LLM 选择和参数 |

## 后续改进

1. **Prompt 优化**: 根据实际使用效果优化 System Prompts
2. **Token 优化**: 减少重复的 System Prompt，使用增量上下文
3. **并行执行**: 多个独立任务可以并行执行
4. **记忆压缩**: 压缩历史 Memory 以节省 Token
5. **错误恢复**: 更智能的错误处理和恢复策略