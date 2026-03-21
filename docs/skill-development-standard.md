# Skill 开发标准与模板

本文档记录 Agent Skills 标准规范，用于指导后续 Skill 开发。

## 官方标准

参考: [agentskills.io/specification](https://agentskills.io/specification)

---

## 一、目录结构

```
skill-name/
├── SKILL.md          # 必需: 元数据 + 指令
├── scripts/          # 可选: 可执行脚本
├── references/       # 可选: 额外文档
└── assets/           # 可选: 资源文件
```

### 文件说明

| 文件/目录 | 必需性 | 用途 | 加载层级 |
|-----------|--------|------|----------|
| `SKILL.md` | **必需** | 主指令文件 | Tier 1-2 |
| `scripts/` | 可选 | Python/Bash 脚本 | Tier 3 |
| `references/` | 可选 | 详细文档、参考 | Tier 3 |
| `assets/` | 可选 | 模板、图片等 | Tier 3 |

---

## 二、SKILL.md 格式

### 2.1 YAML Frontmatter

```yaml
---
# 必需字段
name: skill-name                    # 1-64字符, 小写字母, 连字符分隔
description: |                      # 1-1024字符, 描述何时使用
  简短描述功能和使用场景。
  关键词：关键词1、关键词2、关键词3。

# 可选字段
version: 1.0.0                      # 版本号
author: Your Name                   # 作者
license: Apache-2.0                 # 许可证
compatibility: Python 3.9+          # 环境要求
allowed-tools: bash read python     # 预批准工具
metadata:                           # 自定义元数据
  category: utility
  tags: [search, web]
---
```

### 2.2 字段规则

#### name (必需)
- 长度: 1-64 字符
- 格式: 小写字母 + 连字符
- 示例: `web-research`, `travel-planner`, `pdf-merger`

#### description (必需)
- 长度: 1-1024 字符
- 内容: 描述功能 + 何时使用 + 关键词
- 目的: 让 LLM 判断是否激活此 skill

```yaml
# 好的 description
description: |
  执行网络搜索和网页内容提取。当需要查找实时信息（价格、天气、活动）、
  权威知识（维基百科）、或读取特定网页时使用。
  关键词：搜索、查询、天气、价格、新闻。

# 不好的 description
description: 搜索工具  # 太简短，缺少触发条件
```

#### compatibility (可选)
```yaml
compatibility: Python 3.9+, requests>=2.0
compatibility: Requires TAVILY_API_KEY environment variable
compatibility: Node.js 18+
```

#### allowed-tools (可选)
```yaml
allowed-tools: bash read python
allowed-tools: bash read write edit
```

### 2.3 Markdown 正文

```markdown
---
name: skill-name
description: 描述
---

# Skill 标题

## 何时使用

- 场景1
- 场景2
- 场景3

## 使用方法

### 方法1

```bash
command here
```

### 方法2

```python
code here
```

## 示例

### 示例1: 场景描述

输入: xxx
输出: xxx

### 示例2: 另一个场景

...

## 注意事项

- 注意点1
- 注意点2

## 参考资料

- [参考1](link)
- [参考2](link)
```

---

## 三、Scripts 规范

### 3.1 脚本结构

```python
#!/usr/bin/env python3
"""
脚本名称 - 一句话描述

详细描述脚本功能...

Usage:
    python script_name.py <required_arg> [options]

Examples:
    python script_name.py "query"
    python script_name.py "query" --lang zh

Environment:
    API_KEY - API密钥 (如需要)

Dependencies:
    pip install requests
"""

import json
import sys
import os

def main():
    # 1. 参数验证
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    # 2. 解析参数
    query = sys.argv[1]

    # 3. 执行逻辑
    try:
        result = do_something(query)

        # 4. 输出 JSON
        print(json.dumps({
            "success": True,
            "data": result
        }, ensure_ascii=False, indent=2))

    except Exception as e:
        print(json.dumps({
            "success": False,
            "error": str(e)
        }, ensure_ascii=False, indent=2))
        sys.exit(1)

def do_something(query):
    # 实现逻辑
    return {"query": query, "result": "..."}

if __name__ == "__main__":
    main()
```

### 3.2 输出格式

**标准输出格式 (JSON)**:

```json
{
  "success": true,
  "data": {
    "results": [...]
  }
}
```

**错误输出格式**:

```json
{
  "success": false,
  "error": "错误描述"
}
```

### 3.3 最佳实践

| 实践 | 说明 |
|------|------|
| 独立运行 | 不依赖项目特定模块 |
| 无外部状态 | 纯函数，输入→输出 |
| JSON输出 | 便于 LLM 解析 |
| 完整文档 | Usage 和 Examples |
| 错误处理 | try-except + 有意义的错误信息 |
| 环境变量 | 敏感信息用环境变量 |

### 3.4 Bash 脚本模板

```bash
#!/usr/bin/env bash
#
# 脚本名称 - 一句话描述
#
# Usage:
#   ./script.sh <arg1> [options]
#
# Examples:
#   ./script.sh "query"
#

set -e

# 参数检查
if [ $# -lt 1 ]; then
    echo "Usage: $0 <required_arg>"
    exit 1
fi

ARG="$1"

# 执行逻辑
result=$(echo "$ARG" | tr '[:lower:]' '[:upper:]')

# JSON 输出
cat <<EOF
{
  "success": true,
  "data": "$result"
}
EOF
```

---

## 四、References 规范

### 4.1 用途

- 详细的技术文档
- 扩展的示例
- API 参考
- 故障排除指南

### 4.2 命名

```
references/
├── api-reference.md      # API 详细文档
├── examples.md           # 更多示例
├── troubleshooting.md   # 故障排除
└── advanced-usage.md     # 高级用法
```

### 4.3 在 SKILL.md 中引用

```markdown
## 详细参考

更多示例和高级用法参见:
- [API 参考](references/api-reference.md)
- [故障排除](references/troubleshooting.md)
```

---

## 五、Progressive Disclosure 实现

### 5.1 三层加载

```
┌─────────────────────────────────────────────────────┐
│ Tier 1: 启动加载 (~100 tokens)                      │
│ 内容: name + description                            │
│ 目的: 快速判断是否相关                              │
├─────────────────────────────────────────────────────┤
│ Tier 2: 激活加载 (< 5000 tokens)                    │
│ 内容: SKILL.md 完整内容                             │
│ 目的: 提供详细指令                                  │
├─────────────────────────────────────────────────────┤
│ Tier 3: 按需加载 (无限制)                           │
│ 内容: scripts + references + assets                 │
│ 目的: 执行实际操作                                  │
└─────────────────────────────────────────────────────┘
```

### 5.2 上下文优化

| 加载时机 | 无 Progressive | 有 Progressive | 节省 |
|----------|----------------|----------------|------|
| 启动 | 所有内容 | 仅 metadata | ~98% |
| 激活 | 所有内容 | SKILL.md | ~50% |
| 完整 | 所有内容 | 所有内容 | 0% |

### 5.3 SKILL.md 长度建议

| 部分 | 建议长度 | 原因 |
|------|----------|------|
| 核心指令 | < 2000 tokens | Tier 2 必须加载 |
| 示例 | 3-5 个 | 足够理解用法 |
| 详细文档 | 放 references | Tier 3 按需 |

---

## 六、完整模板

### 6.1 最小模板

```
my-skill/
└── SKILL.md
```

```yaml
---
name: my-skill
description: |
  简短描述功能和何时使用。
  关键词：关键词1、关键词2。
---

# My Skill

## 何时使用

- 场景1
- 场景2

## 使用方法

具体步骤...
```

### 6.2 标准模板

```
my-skill/
├── SKILL.md
├── scripts/
│   └── main.py
└── references/
    └── details.md
```

**SKILL.md**:

```yaml
---
name: my-skill
description: |
  功能描述和使用场景。
  关键词：关键词1、关键词2、关键词3。
compatibility: Python 3.9+
allowed-tools: bash python read
---

# My Skill

## 何时使用

- 场景1: 描述
- 场景2: 描述
- 场景3: 描述

## 可用脚本

| 脚本 | 用途 |
|------|------|
| `scripts/main.py` | 主功能 |

## 使用方法

```bash
python scripts/main.py "参数"
```

## 示例

### 示例1

```bash
python scripts/main.py "test"
```

输出:
```json
{
  "success": true,
  "data": "..."
}
```

## 注意事项

- 注意点1
- 注意点2

## 详细参考

参见 [references/details.md](references/details.md)
```

**scripts/main.py**:

```python
#!/usr/bin/env python3
"""
My Skill Main Script

Usage:
    python main.py <query>

Examples:
    python main.py "test"
"""

import json
import sys

def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    query = sys.argv[1]

    # 处理逻辑
    result = process(query)

    print(json.dumps({
        "success": True,
        "query": query,
        "result": result
    }, ensure_ascii=False, indent=2))

def process(query):
    return f"Processed: {query}"

if __name__ == "__main__":
    main()
```

---

## 七、命名规范

### 7.1 Skill 命名

| 规则 | 示例 |
|------|------|
| 小写字母 | `pdf`, `web` |
| 连字符分隔 | `web-research`, `travel-planner` |
| 动词-名词 | `merge-pdf`, `extract-text` |
| 或名词短语 | `destination-research` |

### 7.2 文件命名

| 文件 | 命名 |
|------|------|
| SKILL.md | 固定名称 |
| scripts/ | 小写 + 下划线: `search_wikipedia.py` |
| references/ | 小写 + 连字符: `api-reference.md` |

---

## 八、检查清单

发布前检查:

- [ ] `name` 符合规范 (小写 + 连字符)
- [ ] `description` 包含使用场景和关键词
- [ ] SKILL.md 长度 < 5000 tokens
- [ ] 脚本有完整 docstring
- [ ] 脚本输出标准 JSON 格式
- [ ] 错误处理完善
- [ ] 测试通过

---

## 九、示例: 官方 PDF Skill

来源: [github.com/anthropics/skills](https://github.com/anthropics/skills/tree/main/skills/pdf)

```
pdf/
├── SKILL.md           # 主指令 (详细)
├── forms.md           # 表单填写指南
├── reference.md       # 高级参考
├── LICENSE.txt        # 许可证
└── scripts/
    ├── check_bounding_boxes.py
    ├── check_fillable_fields.py
    ├── convert_pdf_to_images.py
    ├── create_validation_image.py
    ├── extract_form_field_info.py
    ├── extract_form_structure.py
    ├── fill_fillable_fields.py
    └── fill_pdf_form_with_annotations.py
```

**特点**:
- SKILL.md 包含完整使用指南
- 多个独立脚本处理不同任务
- references 提供高级用法
- 脚本独立、可单独运行

---

## 十、开发流程

```
1. 确定需求
   ↓
2. 创建目录结构
   ↓
3. 编写 SKILL.md (metadata + 核心指令)
   ↓
4. 编写 scripts (如需要)
   ↓
5. 编写 references (如需要)
   ↓
6. 测试
   ↓
7. 文档审查
   ↓
8. 发布
```

---

## 参考资源

- [Agent Skills 官方规范](https://agentskills.io/specification)
- [Anthropic Skills 仓库](https://github.com/anthropics/skills)
- [Claude Skills 使用指南](https://support.claude.com/en/articles/12512180-using-skills-in-claude)
- [创建自定义 Skills](https://support.claude.com/en/articles/12512198-creating-custom-skills)