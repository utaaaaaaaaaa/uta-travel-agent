# Skills 系统指南

## 概述

Skills 是指导 LLM 完成特定任务的 Markdown 指令文件。遵循 [Agent Skills 标准](https://agentskills.io/specification)。

### Skills vs Tools

| Aspect | Skills | Tools |
|--------|--------|-------|
| 本质 | Markdown 指令 | 可执行代码 |
| 目的 | 教 LLM **如何做** | **实际执行** |
| 分发 | 复制文件夹 | 需要编译 |
| 性能 | N/A (指令) | 快 (Go) |
| 灵活性 | 易修改 | 需重新编译 |

**关键区别**: Skills 教授知识，Tools 执行操作。两者互补，不是替代关系。

## 目录结构

```
skill-name/
├── SKILL.md          # 必需: YAML frontmatter + Markdown 指令
├── scripts/          # 可选: Python/Bash 脚本
├── references/       # 可选: 额外文档
└── assets/           # 可选: 模板、资源
```

### SKILL.md 格式

```yaml
---
name: skill-name           # 必需: 1-64字符, 小写, 连字符
description: When to use   # 必需: 1-1024字符
compatibility: Python 3.9+ # 可选: 环境要求
allowed-tools: bash read   # 可选: 预批准工具
---

# 指令内容

这里写具体的操作指南...
```

## Progressive Disclosure (渐进式加载)

### 三层加载机制

```
┌─────────────────────────────────────────────────────┐
│ Tier 1: Metadata (~100 tokens)                      │
│ - 启动时加载                                         │
│ - 只有 name + description                           │
│ - 用于快速匹配                                       │
├─────────────────────────────────────────────────────┤
│ Tier 2: Instructions (< 5000 tokens)                │
│ - Skill 被激活时加载                                 │
│ - SKILL.md 的完整内容                               │
│ - 提供详细指令                                       │
├─────────────────────────────────────────────────────┤
│ Tier 3: Resources (scripts, references)             │
│ - 按需加载                                          │
│ - Python 脚本、参考文档                              │
│ - 完整功能                                          │
└─────────────────────────────────────────────────────┘
```

### 上下文效率

| 场景 | 无 Progressive | 有 Progressive | 节省 |
|------|---------------|----------------|------|
| 启动时 | 5000+ tokens | ~100 tokens | 98% |
| 激活时 | 5000+ tokens | ~500 tokens | 90% |
| 完整使用 | 5000+ tokens | 5000+ tokens | 0% |

## 当前 Skills 列表

| Skill | 描述 | 文件 |
|-------|------|------|
| web-research | 网络搜索指南 | `skills/web-research/` |
| travel-planner | 旅游行程规划 | `skills/travel-planner/` |
| destination-research | 目的地研究 | `skills/destination-research/` |

### web-research

```
skills/web-research/
├── SKILL.md                    # 搜索工具使用指南
├── scripts/                    # Python 搜索脚本
│   ├── wikipedia_search.py     # 维基百科搜索
│   └── tavily_search.py        # Tavily 实时搜索
└── references/
    └── search-strategies.md    # 搜索策略详解
```

**何时使用**:
- 需要实时信息 (价格、天气、新闻)
- 需要权威知识 (维基百科)
- 需要读取网页内容

**使用方法**:
```bash
# 维基百科搜索
python skills/web-research/scripts/wikipedia_search.py "清水寺" --lang zh

# 实时搜索
python skills/web-research/scripts/tavily_search.py "京都 门票价格 2024"
```

## 内置工具 vs Skill 脚本

### 架构

```
┌─────────────────────────────────────────────────────────┐
│                    UTA Agent System                      │
├─────────────────────────────────────────────────────────┤
│                                                          │
│   ┌───────────────────┐    ┌─────────────────────────┐  │
│   │  Built-in Tools   │    │  Skill Scripts          │  │
│   │  (Go - 快速)      │    │  (Python - 灵活)        │  │
│   ├───────────────────┤    ├─────────────────────────┤  │
│   │ • WikipediaSearch │    │ • 可选功能              │  │
│   │ • TavilySearch    │    │ • 社区贡献              │  │
│   │ • WebReader       │    │ • 快速原型              │  │
│   │ • BaiduBaike      │    │ • 易于分发              │  │
│   └───────────────────┘    └─────────────────────────┘  │
│            │                         │                   │
│            └─────────┬───────────────┘                   │
│                      │                                   │
│               ┌──────▼──────┐                            │
│               │ Skills Layer│                            │
│               │ (指令)      │                            │
│               └─────────────┘                            │
│                      │                                   │
│               ┌──────▼──────┐                            │
│               │     LLM     │                            │
│               └─────────────┘                            │
└─────────────────────────────────────────────────────────┘
```

### 选择指南

| 使用 Go Tools | 使用 Skill Scripts |
|--------------|-------------------|
| 核心功能 | 可选功能 |
| 高性能要求 | 社区分享 |
| 类型安全 | 快速迭代 |
| 深度集成 | 原型验证 |

## 创建新 Skill

### 步骤 1: 创建目录

```bash
mkdir -p skills/my-skill/scripts
mkdir -p skills/my-skill/references
```

### 步骤 2: 编写 SKILL.md

```yaml
---
name: my-skill
description: |
  简短描述何时使用此 skill。
  关键词：关键词1、关键词2、关键词3。
compatibility: Python 3.9+
allowed-tools: bash read python
---

# My Skill

## 何时使用

- 场景1
- 场景2

## 使用方法

```bash
python scripts/my_script.py "参数"
```

## 示例

...
```

### 步骤 3: 添加脚本 (可选)

```python
#!/usr/bin/env python3
"""
My Script

Usage:
    python my_script.py "query"
"""

import json
import sys

def main():
    query = sys.argv[1]
    # 处理逻辑
    result = {"success": True, "data": "..."}
    print(json.dumps(result, ensure_ascii=False))

if __name__ == "__main__":
    main()
```

### 步骤 4: 测试

```bash
# 运行 skills 测试
go test ./internal/skills/... -v

# 测试脚本
python skills/my-skill/scripts/my_script.py "test"
```

## Registry API

### Go 代码中使用

```go
import "github.com/utaaa/uta-travel-agent/internal/skills"

// 创建 registry
registry := skills.NewRegistry()
registry.AddDir("./skills")

// 加载 skills (Tier 1)
if err := registry.LoadSkills(); err != nil {
    log.Fatal(err)
}

// 获取 skill
skill := registry.Get("web-research")

// 加载完整内容 (Tier 2/3)
content := skill.LoadTier(skills.Tier2)

// 匹配相关 skills
matches := registry.MatchSkills("帮我搜索京都天气", 3)
```

### 中文匹配

Registry 使用 rune-based 匹配，支持中文:

```go
// 这些查询都能正确匹配
registry.MatchSkills("帮我规划行程", 3)      // → travel-planner
registry.MatchSkills("今天天气怎么样", 3)    // → web-research
registry.MatchSkills("研究一下景点", 3)      // → destination-research
```

## 安装社区 Skill

```bash
# 方式1: Git clone
git clone https://github.com/community/awesome-skill skills/awesome-skill

# 方式2: 下载压缩包
curl -L https://example.com/skill.tar.gz | tar -xz -C skills/

# 重启服务后自动加载
```

## 最佳实践

1. **描述要清晰**: `description` 是 LLM 判断是否使用的关键
2. **关键词很重要**: 在 description 中包含触发关键词
3. **脚本要独立**: Python 脚本应尽量减少依赖
4. **输出 JSON**: 脚本输出 JSON 格式，便于 LLM 解析
5. **错误处理**: 脚本要有完善的错误处理和提示

## 参考资源

- [Agent Skills Specification](https://agentskills.io/specification)
- [Anthropic Skills Repository](https://github.com/anthropics/skills)
- [Claude Skills Guide](https://support.claude.com/en/articles/12512198-creating-custom-skills)