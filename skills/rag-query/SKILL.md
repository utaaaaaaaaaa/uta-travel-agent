# RAG Query

从知识库中检索相关信息并生成回答。

## Description

此技能用于从已构建的目的地知识库中检索相关信息，并利用 LLM 生成自然语言回答。
支持语义搜索和多轮对话上下文。

## Parameters

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `query` | string | ✓ | 用户的问题或查询 |
| `collection` | string | ✓ | 知识库名称（通常是目的地 ID） |
| `top_k` | integer | | 返回结果数量，默认 5 |
| `score_threshold` | float | | 相关性阈值，默认 0.5 |
| `context` | array | | 对话上下文，用于多轮对话 |
| `language` | string | | 回答语言，默认与查询语言一致 |

## Usage

当用户询问关于已创建的目的地相关信息时使用此技能。

### 适用场景
- "京都有哪些著名寺庙？"
- "金阁寺的开放时间是什么？"
- "京都有什么美食推荐？"
- "从京都站怎么去清水寺？"

### 不适用场景
- 创建新的目的地 Agent（需要 build-knowledge-base 技能）
- 实时导航（需要地图服务）
- 预订酒店/门票（需要第三方服务）

## Execution Flow

```
1. 接收查询参数
   ↓
2. 调用 Embedding 服务向量化查询
   ↓
3. 调用 Qdrant MCP 在知识库中搜索
   ↓
4. 获取 Top-K 相关文档块
   ↓
5. 构建上下文：原始文档 + 相关文档
   ↓
6. 调用 LLM Gateway 生成回答
   ↓
7. 返回回答和来源
```

## Dependencies

### MCP Tools
- `qdrant`: 向量数据库搜索

### Services
- `embedding`: 查询向量化
- `llm_gateway`: 生成回答

## Examples

### Input
```json
{
  "query": "京都有哪些必去的寺庙？",
  "collection": "kyoto-agent-001",
  "top_k": 5,
  "score_threshold": 0.6
}
```

### Output
```json
{
  "answer": "京都著名的寺庙包括：\n\n1. **金阁寺（鹿苑寺）**：世界文化遗产，金光闪闪的三层楼阁...\n2. **清水寺**：京都最古老的寺院，以其悬空的清水舞台闻名...\n3. **银阁寺（慈照寺）**：与金阁寺对应，展现侘寂美学...\n4. **天龙寺**：岚山地区著名的禅寺...\n5. **南禅寺**：京都五大禅寺之首...",
  "sources": [
    {
      "content": "金阁寺，正式名称为鹿苑寺...",
      "score": 0.89,
      "metadata": {
        "document_title": "京都寺庙指南",
        "category": "attractions"
      }
    }
  ],
  "confidence": 0.85,
  "suggested_followups": [
    "金阁寺的开放时间和门票？",
    "如何安排一天游览这些寺庙？"
  ]
}
```

## Error Handling

| 错误 | 处理方式 |
|------|----------|
| 集合不存在 | 返回错误，提示需要先创建知识库 |
| 无相关结果 | 返回友好提示，建议换一种问法 |
| Embedding 服务不可用 | 降级为关键词搜索 |

## Notes

- 支持 Streaming 输出，适合长回答
- 自动检测查询语言，用相同语言回答
- 缓存热门查询以提高响应速度