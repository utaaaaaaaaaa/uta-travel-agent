# Claude Skills 实现指南

## 概述

Skills 是一种为 Claude 提供领域知识和操作指令的标准化格式。与工具 (Tools) 不同，Skills 是声明式的指令集，通过 **渐进式披露 (Progressive Disclosure)** 实现高效的上下文利用。

## Skills vs Tools vs MCP

| 特性 | Skills | Tools | MCP |
|------|--------|-------|-----|
| 本质 | 指令/知识 | 执行能力 | 协议标准 |
| 形式 | Markdown + YAML | 代码函数 | JSON-RPC |
| 加载方式 | 渐进式披露 | 一次性加载 | 按需调用 |
| 上下文消耗 | ~100 tokens 起 | 全量参数 schema | 动态发现 |
| 适用场景 | 领域知识、工作流 | 计算操作、外部调用 | 工具标准化 |

## 核心概念：渐进式披露 (Progressive Disclosure)

渐进式披露是 Skills 的核心设计理念，分为三个层级：

```
┌─────────────────────────────────────────────────────────────┐
│                    Tier 1: Frontmatter                      │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  name: git-commit                                    │   │
│  │  description: Create well-formatted git commits     │   │
│  │  ~50-100 tokens - ALWAYS LOADED                      │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼ 用户意图匹配
┌─────────────────────────────────────────────────────────────┐
│                    Tier 2: Partial Content                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  标题、概述、关键指令摘要                              │   │
│  │  ~500 tokens - RELEVANCE CHECK                       │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼ 确认使用
┌─────────────────────────────────────────────────────────────┐
│                    Tier 3: Full Content                     │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  完整指令、示例、边界情况、最佳实践                    │   │
│  │  ~5000+ tokens - ACTIVE INVOCATION                   │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 上下文效率对比

| 场景 | 传统方式 | 渐进式披露 | 节省率 |
|------|----------|------------|--------|
| 100 个 Skills 可用 | 100 × 5000 = 500K tokens | 100 × 100 = 10K tokens | **98%** |
| 10 个潜在相关 | 10 × 5000 = 50K tokens | 10 × 500 = 5K tokens | **90%** |
| 1 个确认使用 | 5000 tokens | 5000 tokens | 0% |

**最大节省可达 140 倍上下文！**

## SKILL.md 标准格式

```markdown
---
name: skill-name
description: |
  清晰描述此 Skill 的用途和使用时机。
  这是 Tier 1 内容，始终加载，应简洁明了。
---

# Skill Name

简短概述此 Skill 的目的和核心功能。

## 何时使用

- 场景 1：描述
- 场景 2：描述
- 场景 3：描述

## 指令

### 步骤 1
详细指令...

### 步骤 2
详细指令...

## 示例

### 示例 1：基础用法
输入：...
输出：...

### 示例 2：高级用法
输入：...
输出：...

## 边界情况

- 情况 A：如何处理
- 情况 B：如何处理

## 最佳实践

1. 实践建议 1
2. 实践建议 2
```

## 实现架构

### 目录结构

```
skills/
├── git-commit/
│   └── SKILL.md
├── code-review/
│   └── SKILL.md
├── test-runner/
│   └── SKILL.md
└── travel-planner/
    └── SKILL.md
```

### 核心组件

```
┌──────────────────────────────────────────────────────────────┐
│                     SkillRegistry                             │
│  - 加载所有 SKILL.md 的 frontmatter (Tier 1)                  │
│  - 维护 name → skill 映射                                     │
│  - 提供 ListSkills() 方法                                     │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│                    SkillMatcher                               │
│  - 分析用户意图                                               │
│  - 计算 query 与 description 的相似度                         │
│  - 返回候选 Skills 列表                                       │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│                    SkillLoader                                │
│  - Tier 2: 加载部分内容 (标题、概述、关键指令)                │
│  - Tier 3: 加载完整内容                                       │
│  - 按需读取，避免全量加载                                     │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│                    SkillExecutor                              │
│  - 将 Skill 内容注入 System Prompt                           │
│  - 执行 Skill 指令                                           │
│  - 返回执行结果                                              │
└──────────────────────────────────────────────────────────────┘
```

## Go 实现示例

### 1. 数据结构

```go
// Skill 表示一个 Skill 的元数据和内容
type Skill struct {
    // Tier 1: 始终加载
    Name        string `yaml:"name"`
    Description string `yaml:"description"`

    // Tier 2/3: 按需加载
    Content     string `yaml:"-"` // 完整 Markdown 内容
    Partial     string `yaml:"-"` // 部分内容 (标题 + 概述)
    loadedTier  int    `yaml:"-"` // 当前加载层级
}

// SkillRegistry 管理所有 Skills
type SkillRegistry struct {
    skills map[string]*Skill
    mu     sync.RWMutex
}
```

### 2. 渐进式加载实现

```go
// LoadTier1 只加载 frontmatter (始终调用)
func (r *SkillRegistry) LoadTier1(skillPath string) (*Skill, error) {
    content, err := os.ReadFile(skillPath)
    if err != nil {
        return nil, err
    }

    // 解析 YAML frontmatter
    frontmatter, body := parseFrontmatter(content)

    skill := &Skill{
        Name:        frontmatter["name"].(string),
        Description: frontmatter["description"].(string),
        Content:     body,
        loadedTier:  1,
    }

    r.skills[skill.Name] = skill
    return skill, nil
}

// LoadTier2 加载部分内容 (标题 + 前 N 行)
func (s *Skill) LoadTier2() string {
    if s.loadedTier >= 2 {
        return s.Partial
    }

    // 提取标题和前 500 字符
    lines := strings.Split(s.Content, "\n")
    var partial []string
    charCount := 0

    for _, line := range lines {
        partial = append(partial, line)
        charCount += len(line)
        if charCount > 500 {
            break
        }
    }

    s.Partial = strings.Join(partial, "\n")
    s.loadedTier = 2
    return s.Partial
}

// LoadTier3 加载完整内容
func (s *Skill) LoadTier3() string {
    s.loadedTier = 3
    return s.Content
}
```

### 3. 意图匹配

```go
// MatchSkills 根据用户查询匹配相关 Skills
func (r *SkillRegistry) MatchSkills(query string, topK int) []*Skill {
    r.mu.RLock()
    defer r.mu.RUnlock()

    type scored struct {
        skill *Skill
        score float64
    }

    var candidates []scored

    // 计算每个 Skill 的相关性分数
    for _, skill := range r.skills {
        score := calculateRelevance(query, skill.Name, skill.Description)
        if score > 0.3 { // 阈值过滤
            candidates = append(candidates, scored{skill, score})
        }
    }

    // 按分数排序，返回 topK
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].score > candidates[j].score
    })

    result := make([]*Skill, 0, topK)
    for i := 0; i < topK && i < len(candidates); i++ {
        result = append(result, candidates[i].skill)
    }

    return result
}

// calculateRelevance 计算相关性分数
func calculateRelevance(query, name, description string) float64 {
    queryLower := strings.ToLower(query)

    // 关键词匹配
    nameScore := keywordMatch(queryLower, strings.ToLower(name))
    descScore := keywordMatch(queryLower, strings.ToLower(description))

    // 名称匹配权重更高
    return nameScore*0.7 + descScore*0.3
}
```

### 4. 与 Agent 集成

```go
// Agent 集成示例
type Agent struct {
    skillRegistry *SkillRegistry
    // ...
}

func (a *Agent) ProcessQuery(ctx context.Context, query string) (string, error) {
    // Step 1: 匹配相关 Skills (Tier 1 已加载)
    candidates := a.skillRegistry.MatchSkills(query, 3)

    // Step 2: 加载 Tier 2 内容进行确认
    var relevantSkills []string
    for _, skill := range candidates {
        partial := skill.LoadTier2()
        // 可以让 LLM 判断是否真的相关
        if a.confirmRelevance(query, partial) {
            relevantSkills = append(relevantSkills, skill.Name)
        }
    }

    // Step 3: 对确认的 Skills 加载 Tier 3
    var skillInstructions []string
    for _, name := range relevantSkills {
        if skill, ok := a.skillRegistry.Get(name); ok {
            fullContent := skill.LoadTier3()
            skillInstructions = append(skillInstructions, fullContent)
        }
    }

    // Step 4: 构建包含 Skill 指令的 Prompt
    systemPrompt := a.buildPromptWithSkills(skillInstructions)

    // Step 5: 调用 LLM
    return a.llm.Call(ctx, systemPrompt, query)
}
```

## 技能检测逻辑实现

### 方法 1：关键词 + 语义相似度

```go
func (r *SkillRegistry) DetectSkillIntent(query string) []*Skill {
    // 1. 预定义关键词触发
    keywordMap := map[string][]string{
        "git-commit":     {"commit", "提交", "git commit"},
        "code-review":    {"review", "审查", "代码检查"},
        "test-runner":    {"test", "测试", "运行测试"},
        "travel-planner": {"旅游", "行程", "景点", "travel"},
    }

    queryLower := strings.ToLower(query)
    var matches []*Skill

    for skillName, keywords := range keywordMap {
        for _, kw := range keywords {
            if strings.Contains(queryLower, strings.ToLower(kw)) {
                if skill, ok := r.skills[skillName]; ok {
                    matches = append(matches, skill)
                }
                break
            }
        }
    }

    return matches
}
```

### 方法 2：Embedding 相似度 (更精确)

```go
func (r *SkillRegistry) DetectSkillByEmbedding(ctx context.Context, query string) ([]*Skill, error) {
    // 1. 获取 query 的 embedding
    queryEmbedding, err := r.embeddingService.Embed(ctx, query)
    if err != nil {
        return nil, err
    }

    // 2. 与每个 skill description 计算余弦相似度
    type scored struct {
        skill *Skill
        score float64
    }

    var candidates []scored
    for _, skill := range r.skills {
        // description embedding 可预计算缓存
        descEmbedding := r.getCachedEmbedding(skill.Name)
        score := cosineSimilarity(queryEmbedding, descEmbedding)

        if score > 0.7 { // 高阈值
            candidates = append(candidates, scored{skill, score})
        }
    }

    // 3. 排序返回
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].score > candidates[j].score
    })

    result := make([]*Skill, 0, 3)
    for i := 0; i < 3 && i < len(candidates); i++ {
        result = append(result, candidates[i].skill)
    }

    return result, nil
}
```

## 真实 Skills vs 伪 Skills

### ❌ 伪 Skill (当前项目的错误实现)

```go
// 这不是 Skill，这是 Tool 的别名
type Skill struct {
    Name        string
    Description string
    Execute     func(ctx context.Context, params map[string]any) (*Result, error)
}
```

**问题**：
1. 没有 Markdown 格式
2. 没有渐进式披露
3. 没有上下文优化
4. 本质就是 Tool

### ✅ 真实 Skill

```go
// 真正的 Skill 是指令集
type Skill struct {
    // Tier 1 - 始终加载 (~100 tokens)
    Name        string `yaml:"name"`
    Description string `yaml:"description"`

    // Tier 2/3 - 按需加载
    Content     string // Markdown 指令内容

    // 状态
    loadedTier  int
}

// Skill 不执行代码，而是提供指令给 LLM
func (s *Skill) GetInstructions(tier int) string {
    switch tier {
    case 1:
        return fmt.Sprintf("Skill: %s\n%s", s.Name, s.Description)
    case 2:
        return s.Partial
    case 3:
        return s.Content
    }
    return ""
}
```

## 迁移策略

### Step 1: 创建 Skills 目录

```bash
mkdir -p skills/{git-commit,code-review,test-runner,travel-planner}
```

### Step 2: 为每个功能创建 SKILL.md

```markdown
<!-- skills/travel-planner/SKILL.md -->
---
name: travel-planner
description: |
  规划旅游行程，包括景点推荐、路线优化、时间安排。
  当用户提到"行程"、"旅游计划"、"怎么玩"时使用。
---

# Travel Planner

帮助用户规划完整的旅游行程。

## 何时使用

- 用户询问"XX怎么玩"
- 用户需要规划多日行程
- 用户需要景点推荐

## 指令

### 步骤 1：收集信息
询问用户的：
- 目的地
- 行程天数
- 偏好（文化/美食/自然风光）
- 预算范围

### 步骤 2：生成行程
...

## 示例

### 示例 1：3天京都行程
...
```

### Step 3: 实现 SkillRegistry

```go
registry := NewSkillRegistry()
registry.LoadSkillsFromDir("./skills")

// Tier 1 自动加载所有 frontmatter
// 每个 Skill 仅消耗 ~100 tokens
```

### Step 4: Agent 集成

```go
// 在 LLMAgent 中添加 Skill 支持
type LLMAgent struct {
    skillRegistry *SkillRegistry
    // ...
}

func (a *LLMAgent) Think(ctx context.Context, input string) (*Thought, error) {
    // 检测相关 Skills
    skills := a.skillRegistry.DetectSkillIntent(input)

    // 加载 Tier 2 确认相关性
    var activeSkills []string
    for _, skill := range skills {
        partial := skill.LoadTier2()
        // 简单确认逻辑
        activeSkills = append(activeSkills, skill.Name)
    }

    // 只对确认的 Skills 加载 Tier 3
    var instructions []string
    for _, name := range activeSkills {
        if skill := a.skillRegistry.Get(name); skill != nil {
            instructions = append(instructions, skill.LoadTier3())
        }
    }

    // 构建 Prompt
    prompt := a.buildPromptWithSkillInstructions(instructions)

    // LLM 思考
    return a.llm.Think(ctx, prompt)
}
```

## 最佳实践

### 1. Frontmatter 要简洁

```yaml
# ✅ 好的 description
description: "Create well-formatted git commits with conventional messages"

# ❌ 太长的 description
description: |
  This skill helps you create git commits. It will analyze your changes,
  generate a commit message following conventional commits specification,
  include a body if needed, and ensure proper formatting...
```

### 2. 分层内容设计

- **Tier 1**: 让 AI 知道"何时用"
- **Tier 2**: 让 AI 判断"是否真的相关"
- **Tier 3**: 完整指令，让 AI "正确执行"

### 3. 避免重复

```markdown
<!-- ❌ 不好 -->
---
description: Git commit skill
---

# Git Commit
This is a git commit skill...

<!-- ✅ 好 -->
---
description: Create well-formatted git commits following conventional commits spec
---

# Git Commit Skill
[详细指令从这里开始，不重复 description]
```

### 4. 与 Tools 配合

Skills 提供知识，Tools 提供能力：

```
用户: "帮我提交代码"

Skill (git-commit):   提供如何写 commit message 的指令
Tool (git):           执行实际的 git commit 命令
```

## 总结

| 要点 | 说明 |
|------|------|
| **本质** | Skills 是 Markdown 指令集，不是可执行代码 |
| **核心** | 渐进式披露 (3-Tier 加载) |
| **收益** | 高效上下文利用，最多节省 140x |
| **格式** | YAML frontmatter + Markdown body |
| **集成** | 通过 System Prompt 注入 LLM |

---

## 参考资料

- [Anthropic Skills Repository](https://github.com/anthropics/skills)
- [Awesome Claude Skills](https://github.com/ComposioHQ/awesome-claude-skills)
- [Claude Code Documentation - Skills](https://docs.anthropic.com/en/docs/claude-code/skills)
