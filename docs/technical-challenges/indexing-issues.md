# 索引系统问题记录

## 问题概述

当前 `BuildKnowledgeIndexToolAdapter` 和 `CuratorAgent` 的实现存在严重问题，导致 RAG 检索效果不佳。

## 问题 1: 缺少文档分块 (Chunking)

### 当前实现

```go
// cmd/orchestrator/main.go:507-549
// ❌ 直接将整个文档作为一个向量
texts := make([]string, 0, len(documents))
for _, doc := range documents {
    if m, ok := doc.(map[string]any); ok {
        if content, ok := m["content"].(string); ok {
            texts = append(texts, content)  // 整篇文档，未分块！
        }
    }
}

// ❌ 每个文档 = 1 个向量
resp, err := a.embeddingClient.Embed(ctx, clients.EmbedRequest{
    Texts:    texts,  // 整篇文档作为一个向量
})
```

### 问题影响

| 问题 | 影响 |
|------|------|
| 检索粒度太粗 | 无法检索到具体段落 |
| Token 限制 | Embedding 模型通常限制 512-8192 tokens |
| 向量质量差 | 长文本的向量会"稀释"关键信息 |
| RAG 效果差 | 用户问具体问题时无法精准检索 |

### 正确实现

```
文档 (5000 字符)
    │
    ▼ 文本分块 (算法，非 LLM)
    ├─ Chunk 1 (500 字符) → Embedding → Vector 1
    ├─ Chunk 2 (500 字符) → Embedding → Vector 2
    ├─ Chunk 3 (500 字符) → Embedding → Vector 3
    └─ ... 每个文档分成 N 个 chunk
```

```go
// SemanticChunk 按语义边界分块
func SemanticChunk(text string, maxChunkSize int) []string {
    // 按段落分割
    paragraphs := strings.Split(text, "\n\n")

    var chunks []string
    var currentChunk strings.Builder

    for _, para := range paragraphs {
        if currentChunk.Len() + len(para) > maxChunkSize && currentChunk.Len() > 0 {
            chunks = append(chunks, currentChunk.String())
            currentChunk.Reset()
        }
        currentChunk.WriteString(para)
        currentChunk.WriteString("\n\n")
    }

    if currentChunk.Len() > 0 {
        chunks = append(chunks, currentChunk.String())
    }

    return chunks
}
```

## 问题 2: CuratorAgent 滥用 LLM

### 当前实现

```go
// internal/agent/curator_agent.go:202-298
// ❌ 使用 LLM 评估质量 - 昂贵且不必要
result, tokensIn, tokensOut, err := a.evaluateDocuments(ctx, documents, state)
```

### 问题影响

| 问题 | 影响 |
|------|------|
| 成本高 | 每次评估都消耗 LLM tokens |
| 速度慢 | LLM 调用有延迟 |
| 不必要 | 质量评估可以用简单规则 |

### 正确实现

质量评估应该用确定性规则：

```go
func evaluateDocumentQuality(doc Document) float64 {
    score := 1.0

    // 1. 内容长度检查
    if len(doc.Content) < 100 {
        score *= 0.3  // 内容过短
    } else if len(doc.Content) < 300 {
        score *= 0.7  // 内容较短
    }

    // 2. 来源可信度
    trustedSources := []string{"wikipedia.org", "baike.baidu.com", "gov.cn"}
    isTrusted := false
    for _, src := range trustedSources {
        if strings.Contains(doc.Source, src) {
            isTrusted = true
            break
        }
    }
    if !isTrusted {
        score *= 0.8
    }

    // 3. 去重检测 (使用 hash)
    // ...

    return score
}
```

## 修复优先级

| 优先级 | 问题 | 影响 |
|--------|------|------|
| P0 | 缺少分块 | RAG 完全不可用 |
| P1 | CuratorAgent 滥用 LLM | 成本和性能问题 |
| P2 | 分块策略优化 | 检索质量提升 |

## 相关文件

| 文件 | 需要修改 |
|------|----------|
| `cmd/orchestrator/main.go` | `BuildKnowledgeIndexToolAdapter.Execute()` |
| `internal/agent/curator_agent.go` | `evaluateDocuments()` |
| `internal/agent/indexer_agent.go` | 调用分块工具 |

## 参考资料

- [LangChain Text Splitters](https://python.langchain.com/docs/modules/data_connection/document_transformers/)
- [LlamaIndex Chunking Strategies](https://docs.llamaindex.ai/en/stable/module_guides/loading/node_parsers/modules/)
- [RAG 分块最佳实践](https://www.pinecone.io/learn/chunking-strategies/)

---

**状态**: 待修复
**创建时间**: 2026-03-21
**预计工作量**: 2-3 小时