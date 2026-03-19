# Build Knowledge Index

将结构化知识向量化并构建 RAG 检索索引。

## Description

此技能将 Curator 整理的结构化知识进行分块、向量化，并存储到向量数据库。
是 RAG 系统的关键组件，决定了检索质量。

## Parameters

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `knowledge_base` | object | ✓ | Curator 输出的结构化知识 |
| `collection_name` | string | ✓ | 向量集合名称 |
| `options` | object | | 索引选项 |

### options 子参数
| 参数 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `chunk_size` | integer | 384 | 分块大小（tokens） |
| `chunk_overlap` | integer | 50 | 分块重叠（tokens） |
| `embedding_model` | string | multilingual | Embedding 模型 |
| `batch_size` | integer | 100 | 批处理大小 |

## Usage

在 Curator Agent 完成知识整理后调用此技能。

### 分块策略

| 内容类型 | 策略 | 说明 |
|----------|------|------|
| 景点描述 | 保持完整 | 一个景点一个 chunk，避免信息割裂 |
| 交通指南 | 按路线分块 | 每条路线/方式一个 chunk |
| 美食推荐 | 按类型/区域 | 聚合相似餐厅 |
| 注意事项 | 按主题分块 | 相关 tips 聚合 |

## Execution Flow

```
1. 接收知识库和配置
   ↓
2. 检查/创建向量集合
   ↓
3. 遍历知识库内容
   ├── 按类型选择分块策略
   ├── 生成文本块
   ├── 保留元数据
   └── 添加上下文信息
   ↓
4. 批量向量化
   ├── 调用 Embedding 服务
   ├── 处理错误和重试
   └── 进度跟踪
   ↓
5. 存储到向量数据库
   ├── 上传向量和元数据
   ├── 建立索引
   └── 验证存储
   ↓
6. 验证索引质量
   ├── 测试查询
   ├── 检查召回率
   └── 生成报告
   ↓
7. 返回索引信息
```

## Dependencies

### MCP Tools
- `qdrant`: 向量数据库操作

### Services
- `embedding`: 文本向量化

## Output Schema

```json
{
  "collection_name": "string",
  "status": "created | updated | failed",
  "statistics": {
    "total_items": "integer",
    "total_chunks": "integer",
    "total_vectors": "integer",
    "by_category": {
      "attractions": "integer",
      "food": "integer",
      "transport": "integer"
    }
  },
  "config": {
    "vector_size": "integer",
    "distance_metric": "string",
    "embedding_model": "string"
  },
  "quality_report": {
    "test_queries_passed": "integer",
    "test_queries_total": "integer",
    "avg_relevance_score": "number"
  }
}
```

## Examples

### Input
```json
{
  "knowledge_base": {
    "destination": "Kyoto",
    "categories": {...}
  },
  "collection_name": "kyoto-agent-001",
  "options": {
    "chunk_size": 384,
    "embedding_model": "multilingual"
  }
}
```

### Output
```json
{
  "collection_name": "kyoto-agent-001",
  "status": "created",
  "statistics": {
    "total_items": 45,
    "total_chunks": 78,
    "total_vectors": 78,
    "by_category": {
      "attractions": 35,
      "food": 20,
      "transport": 12,
      "tips": 11
    }
  },
  "config": {
    "vector_size": 768,
    "distance_metric": "cosine",
    "embedding_model": "sentence-transformers/paraphrase-multilingual-mpnet-base-v2"
  },
  "quality_report": {
    "test_queries_passed": 9,
    "test_queries_total": 10,
    "avg_relevance_score": 0.87
  }
}
```

## Error Handling

| 错误 | 处理方式 |
|------|----------|
| 集合已存在 | 覆盖更新或报错（根据配置） |
| Embedding 失败 | 重试 3 次，记录失败项 |
| 存储失败 | 回滚部分数据，报错 |

## Notes

- 大型知识库使用批处理避免超时
- 支持增量更新，避免全量重建
- 保留原始文档 ID 以便追溯