# MCP & Skills Integration Guide

本文档记录 MCP (Model Context Protocol) 和 Skills 的使用方式，供项目添加新工具时参考。

---

## MCP (Model Context Protocol)

### 概述

MCP 是一个标准化的协议，用于 AI Agent 与外部工具/服务之间的通信。它基于 JSON-RPC 2.0 规范。

### 协议规范

| 属性 | 值 |
|------|-----|
| 协议版本 | `2024-11-05` |
| 传输协议 | JSON-RPC 2.0 |
| 传输方式 | stdio, SSE (Server-Sent Events), HTTP |

### 核心方法

| 方法 | 说明 | 参数 |
|------|------|------|
| `initialize` | 初始化连接 | `protocolVersion`, `capabilities`, `clientInfo` |
| `tools/list` | 获取可用工具列表 | 无 |
| `tools/call` | 执行工具 | `name`, `arguments` |

### 请求格式

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {}
    },
    "clientInfo": {
      "name": "uta-travel-agent",
      "version": "0.5.0"
    }
  }
}
```

### 响应格式

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "serverInfo": {
      "name": "tavily-mcp",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": {}
    }
  }
}
```

### 工具定义格式

```json
{
  "name": "tavily-search",
  "description": "Search the web using Tavily API",
  "inputSchema": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "Search query"
      }
    },
    "required": ["query"]
  }
}
```

### 工具执行响应

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"query\": \"...\", \"results\": [...]}"
      }
    ],
    "isError": false
  }
}
```

### SSE 响应格式

当 Content-Type 为 `text/event-stream` 时：

```
data: {"jsonrpc": "2.0", "id": 1, "result": {...}}

```

### 项目实现

我们的 MCP 客户端实现在：

- `internal/mcp/protocol_client.go` - MCP 协议客户端
- `internal/mcp/tavily_tool.go` - Tavily MCP 工具封装

使用示例：

```go
// 创建 MCP 客户端
client := mcp.NewProtocolClient(mcp.ProtocolConfig{
    BaseURL: "https://mcp.tavily.com/mcp/?tavilyApiKey=xxx",
})

// 初始化连接
if err := client.Initialize(ctx); err != nil {
    return err
}

// 列出可用工具
tools := client.ListTools()

// 执行工具
result, err := client.ExecuteTool(ctx, "tavily-search", map[string]interface{}{
    "query": "西湖景点",
})
```

---

## Skills (Agent Skills)

### 概述

Skills 是一种基于目录的能力扩展机制，采用"渐进式披露"(Progressive Disclosure) 原则。Claude 只在需要时才读取 Skill 的详细内容。

### 目录结构

```
skills/
└── skill-name/
    └── SKILL.md
```

### SKILL.md 格式

**重要**: Skills 使用 YAML Front Matter（不是 XML），元数据位于 `---` 分隔符之间：

```markdown
---
name: skill-name
description: A brief description of what this skill does
triggers:
  - keyword1
  - keyword2
---

# Skill Title

Detailed instructions for the skill...

## Usage

具体的使用说明...

## Examples

示例代码或用法...
```

### 元数据字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | Skill 标识符 |
| `description` | string | 简短描述 |
| `triggers` | []string | 触发关键词 |
| `version` | string | 版本号（可选） |
| `author` | string | 作者（可选） |

### 渐进式披露

1. Claude 首先看到 Skill 的元数据（name, description, triggers）
2. 只有当对话内容匹配触发条件时，Claude 才会读取完整内容
3. 这种设计减少了上下文占用，提高了响应效率

### 与 MCP 的区别

| 特性 | MCP | Skills |
|------|-----|--------|
| 类型 | 协议 | 文档 |
| 执行方式 | 远程调用 | 本地指令 |
| 适用场景 | 外部服务集成 | Agent 能力扩展 |
| 复杂度 | 需要客户端实现 | 仅需 Markdown 文件 |
| 实时性 | 实时调用 | 静态指令 |

### 使用场景

**适合 MCP**:
- 需要调用外部 API（如搜索、数据库）
- 需要实时数据
- 需要复杂计算

**适合 Skills**:
- 静态知识注入
- 行为规范指导
- 领域专家知识

---

## Tavily 集成示例

### 三种接入方式对比

| 方式 | 特点 | 适用场景 |
|------|------|----------|
| **API** | 直接调用，最快 | 生产环境，稳定性优先 |
| **MCP** | 标准协议，可发现工具 | 多工具集成，协议标准化 |
| **Skills** | 本地指令，无需网络 | 静态知识，行为指导 |

### API 方式 (当前默认)

```go
tool := tools.NewTavilySearchToolWithProxy(apiKey, proxyURL)
result, err := tool.Execute(ctx, map[string]any{
    "query": "西湖景点",
    "max_results": 5,
})
```

### MCP 方式

```go
tool := mcp.NewTavilyMCPTool(mcp.TavilyMCPConfig{
    APIKey: apiKey,
})
if err := tool.Initialize(ctx); err != nil {
    return err
}
result, err := tool.Execute(ctx, map[string]any{
    "query": "西湖景点",
})
```

### Skills 方式 (规划中)

创建 `skills/tavily-search/SKILL.md`:

```markdown
---
name: tavily-search
description: Search the web for real-time information
triggers:
  - search
  - find
  - look up
  - 搜索
  - 查找
---

# Tavily Search Skill

## Purpose
Search the web for real-time, accurate information.

## Instructions
1. Use Tavily API endpoint: https://api.tavily.com/search
2. Include API key in headers
3. Parse JSON response for results

## Parameters
- query: Search query string
- max_results: Number of results (default: 5)
- search_depth: "basic" or "advanced"
```

---

## 项目配置

### 目录结构

```
internal/mcp/
├── protocol/           # MCP 协议核心
│   └── client.go       # JSON-RPC 2.0 客户端
├── registry.go         # MCP 工具注册逻辑
└── tools/              # MCP 工具实现
    └── tavily.go       # Tavily 搜索工具

cmd/orchestrator/
├── main.go             # 服务入口
└── config.go           # 配置 (含 TAVILY_MODE)
```

### 添加新的 MCP 工具

1. 在 `internal/mcp/tools/` 创建新文件，如 `brave.go`:

```go
package tools

import (
    "context"
    "github.com/utaaa/uta-travel-agent/internal/mcp/protocol"
)

type BraveTool struct {
    client *protocol.ProtocolClient
    apiKey string
}

func NewBrave(cfg BraveConfig) *BraveTool {
    return &BraveTool{...}
}

func (t *BraveTool) Name() string { return "brave_search_mcp" }
func (t *BraveTool) Description() string { return "..." }
func (t *BraveTool) Parameters() map[string]any { return {...} }
func (t *BraveTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
    // 实现逻辑
}
```

2. 在 `internal/mcp/registry.go` 添加注册函数:

```go
func RegisterBrave(registry ToolRegistry, cfg BraveConfig) bool {
    // 类似 RegisterTavily 的实现
}
```

3. 在 `cmd/orchestrator/main.go` 调用注册函数

### 配置选择 Tavily 接入方式

```go
type Config struct {
    // ...
    TavilyAPIKey  string
    TavilyMode    string // "api", "mcp", "skills"
}
```

环境变量配置：

```bash
# .env
TAVILY_API_KEY=tvly-xxx
TAVILY_MODE=api  # or "mcp" or "skills"
```

---

## 参考链接

- [MCP Specification](https://modelcontextprotocol.io/)
- [MCP TypeScript SDK](https://github.com/modelcontextprotocol/typescript-sdk)
- [Tavily MCP Server](https://mcp.tavily.com/)
- [Tavily Skills Documentation](https://docs.tavily.com/documentation/agent-skills)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)