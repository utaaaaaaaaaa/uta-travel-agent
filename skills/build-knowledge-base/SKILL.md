# Build Knowledge Base

将收集的原始信息整理为结构化的知识库。

## Description

此技能用于将 Researcher Agent 收集的原始旅游信息整理为结构化的知识库。
包括分类、打标签、去重、建立关联等操作。

## Parameters

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `raw_data` | object | ✓ | Researcher 收集的原始数据 |
| `destination` | string | ✓ | 目的地名称 |
| `options` | object | | 整理选项 |

### options 子参数
| 参数 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `categories` | array | 默认分类 | 自定义分类 |
| `deduplicate` | boolean | true | 是否去重 |
| `min_quality_score` | float | 0.5 | 最低质量分 |

## Usage

在 Researcher Agent 完成信息收集后调用此技能。

### 适用场景
- Researcher 完成信息收集，需要整理
- 知识库需要更新或扩充
- 知识库结构需要调整

## Execution Flow

```
1. 接收原始数据和配置
   ↓
2. 验证数据格式
   ↓
3. 内容分类
   ├── 识别内容类型
   ├── 分配到预设分类
   └── 创建新分类（如需要）
   ↓
4. 标签提取
   ├── 提取关键词
   ├── 识别主题
   └── 添加属性标签
   ↓
5. 去重处理
   ├── 内容相似度检测
   ├── 合并重复项
   └── 保留最完整版本
   ↓
6. 建立关联
   ├── 地理位置关联
   ├── 主题关联
   └── 时间关联
   ↓
7. 质量检查
   ├── 完整性检查
   ├── 一致性检查
   └── 生成质量报告
   ↓
8. 输出结构化知识库
```

## Dependencies

### Services
- `llm_gateway`: 用于分类、标签提取、质量评估

## Output Schema

```json
{
  "knowledge_base": {
    "destination": "string",
    "version": "string",
    "categories": {
      "attractions": ["KnowledgeItem"],
      "food": ["KnowledgeItem"],
      "transport": ["KnowledgeItem"],
      "accommodation": ["KnowledgeItem"],
      "culture": ["KnowledgeItem"],
      "tips": ["KnowledgeItem"]
    },
    "tags": ["string"],
    "relations": [
      {
        "from": "string",
        "to": "string",
        "type": "string",
        "weight": "number"
      }
    ]
  },
  "statistics": {
    "total_items": "integer",
    "categories_count": "integer",
    "tags_count": "integer",
    "relations_count": "integer",
    "quality_score": "number"
  },
  "quality_report": {
    "completeness": "number",
    "consistency": "number",
    "issues": ["string"]
  }
}
```

## Examples

### Input
```json
{
  "destination": "Kyoto",
  "raw_data": {
    "attractions": [
      {
        "name": "金阁寺",
        "description": "正式名称为鹿苑寺...",
        "source": "https://..."
      },
      {
        "name": "Kinkaku-ji",
        "description": "The Golden Pavilion...",
        "source": "https://..."
      }
    ],
    "food": [...]
  }
}
```

### Output
```json
{
  "knowledge_base": {
    "destination": "Kyoto",
    "version": "1.0.0",
    "categories": {
      "attractions": [
        {
          "id": "attr-001",
          "name": "金阁寺 (Kinkaku-ji)",
          "aliases": ["鹿苑寺", "Golden Pavilion"],
          "description": "...",
          "tags": ["temple", "world-heritage", "must-visit"],
          "content": "...",
          "metadata": {
            "opening_hours": "9:00-17:00",
            "admission": "400 JPY",
            "location": {...}
          }
        }
      ]
    }
  },
  "statistics": {
    "total_items": 45,
    "quality_score": 0.92
  }
}
```

## Notes

- 自动合并不同语言的重复信息
- 保留原始来源以便追溯
- 支持增量更新已有知识库