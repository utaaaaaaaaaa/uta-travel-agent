# MCP (Model Context Protocol) 系统设计

## 概述

MCP (Model Context Protocol) 是一种标准化的 Agent-工具通信协议。本文档描述 UTA Travel Agent 中简化版 MCP 客户端的实现。

## 设计理念

参考 [Datawhale hello-agents](https://datawhalechina.github.io/hello-agents/#/en/chapter10/Chapter10-Agent-Communication-Protocols) 的设计理念：

> **注册新 MCP 工具只需一行代码**

```go
// Stdio 模式 - 本地 MCP 服务器
github_tool := mcpclient.NewMCPTool(mcpclient.StdioConfig{
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-github"},
})

// HTTP 模式 - 远程 MCP 服务器
tavily_tool := mcpclient.NewMCPTool(mcpclient.HTTPConfig{
    URL: "https://mcp.tavily.com/mcp/?tavilyApiKey=xxx",
})
```

## 核心接口

### 单一 Run 方法

所有 MCP 操作通过统一的 `Run` 方法完成：

```go
// 列出所有工具
result, err := tool.Run(ctx, mcpclient.ActionListTools)

// 调用特定工具
result, err := tool.Run(ctx, mcpclient.ActionCall("search_repos", map[string]any{
    "query": "mcp",
}))
```

### Action 类型

```go
// 预定义 Action
ActionListTools    = Action{Type: "list_tools"}
ActionInitialized  = Action{Type: "initialized"}

// 创建调用 Action
ActionCall(toolName string, arguments map[string]any) Action
```

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                     MCPTool                              │
│  - 统一接口 (Run 方法)                                   │
│  - 延迟初始化 (sync.Once)                                │
│  - 自动协议协商                                          │
└───────────────────────┬─────────────────────────────────┘
                        │
          ┌─────────────┴─────────────┐
          │                           │
          ▼                           ▼
┌─────────────────┐         ┌─────────────────┐
│  StdioClient    │         │   HTTPClient    │
│  - 本地进程      │         │   - 远程服务    │
│  - JSON-RPC     │         │   - JSON-RPC    │
│  - stdin/stdout │         │   - SSE 支持    │
└─────────────────┘         └─────────────────┘
```

## 传输模式

### Stdio 模式 (本地 MCP 服务器)

适用于通过命令行启动的 MCP 服务器：

```go
tool := mcpclient.NewMCPTool(mcpclient.StdioConfig{
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-github"},
    Env:     []string{"GITHUB_TOKEN=xxx"}, // 可选环境变量
})
```

**工作流程**:
1. 启动子进程 (如 `npx -y @modelcontextprotocol/server-github`)
2. 通过 stdin 发送 JSON-RPC 请求
3. 通过 stdout 接收 JSON-RPC 响应
4. 自动发送 initialize 和 initialized 消息

### HTTP 模式 (远程 MCP 服务器)

适用于 HTTP/HTTPS 暴露的 MCP 服务器：

```go
tool := mcpclient.NewMCPTool(mcpclient.HTTPConfig{
    URL:      "https://mcp.tavily.com/mcp/?tavilyApiKey=xxx",
    Proxy:    os.Getenv("HTTPS_PROXY"), // 可选代理
    Timeout:  60 * time.Second,          // 可选超时
    Insecure: false,                      // 可选跳过 TLS 验证
})
```

**工作流程**:
1. 发送 POST 请求到 MCP URL
2. 支持 JSON 和 SSE (Server-Sent Events) 响应
3. 自动解析响应格式

## Agent 集成

### MCPToolAdapter

将 MCP 工具适配到 Agent 系统的 ToolExecutor 接口：

```go
// 创建适配器
adapter := mcpclient.NewMCPToolAdapter(mcpTool, "search_repos")

// 使用适配器执行
result, err := adapter.Execute(ctx, map[string]any{"query": "mcp"})
```

### 自动发现并注册

一键发现 MCP 服务器的所有工具并注册到 Agent 工具库：

```go
registry := agent.NewToolRegistry()
mcpTool := mcpclient.NewMCPTool(config)

// 自动发现并注册所有工具
err := mcpclient.DiscoverAndRegisterTools(ctx, registry, mcpTool)
```

## 数据结构

### ToolInfo

```go
type ToolInfo struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    InputSchema map[string]any `json:"inputSchema"`
}
```

### Result

```go
type Result struct {
    Success bool       `json:"success"`
    Data    any        `json:"data,omitempty"`
    Error   string     `json:"error,omitempty"`
    Tools   []ToolInfo `json:"tools,omitempty"`
    Content string     `json:"content,omitempty"`
}
```

## 错误处理

```go
result, err := tool.Run(ctx, action)
if err != nil {
    // 网络错误、进程启动失败等
    return err
}

if !result.Success {
    // MCP 协议错误
    return fmt.Errorf("MCP error: %s", result.Error)
}

// 成功
fmt.Println(result.Content)
```

## 示例：GitHub MCP

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/utaaa/uta-travel-agent/internal/mcp/mcpclient"
)

func main() {
    ctx := context.Background()

    // 1. 创建 MCP 工具 (一行注册)
    tool := mcpclient.NewMCPTool(mcpclient.StdioConfig{
        Command: "npx",
        Args:    []string{"-y", "@modelcontextprotocol/server-github"},
    })
    defer tool.Close()

    // 2. 列出可用工具
    result, err := tool.Run(ctx, mcpclient.ActionListTools)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d tools:\n", len(result.Tools))
    for _, t := range result.Tools {
        fmt.Printf("  - %s: %s\n", t.Name, t.Description)
    }

    // 3. 调用工具
    result, err = tool.Run(ctx, mcpclient.ActionCall("search_repos", map[string]any{
        "query": "mcp protocol",
    }))
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Result: %s\n", result.Content)
}
```

## 示例：Tavily MCP

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/utaaa/uta-travel-agent/internal/mcp/mcpclient"
)

func main() {
    ctx := context.Background()

    // 1. 创建 MCP 工具 (一行注册)
    tool := mcpclient.NewMCPTool(mcpclient.HTTPConfig{
        URL:    fmt.Sprintf("https://mcp.tavily.com/mcp/?tavilyApiKey=%s", os.Getenv("TAVILY_API_KEY")),
        Proxy:  os.Getenv("HTTPS_PROXY"),
    })

    // 2. 搜索
    result, err := tool.Run(ctx, mcpclient.ActionCall("tavily-search", map[string]any{
        "query": "京都旅游攻略",
    }))
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Search result: %s\n", result.Content)
}
```

## 与现有工具系统对比

| 特性 | 旧实现 | 新实现 |
|------|--------|--------|
| 注册复杂度 | 多个函数调用 | 一行代码 |
| 使用方式 | 多个方法 | 单一 Run 方法 |
| 传输支持 | 仅 HTTP | HTTP + Stdio |
| 自动发现 | 手动配置 | 自动列出工具 |
| Agent 集成 | 需要适配器 | 内置适配器 |

## 文件结构

```
internal/mcp/mcpclient/
├── client.go    # 核心实现 (MCPTool, stdioClient, httpClient)
└── adapter.go   # Agent 集成 (MCPToolAdapter, DiscoverAndRegisterTools)
```

## 配置选项

### StdioConfig

| 字段 | 类型 | 说明 |
|------|------|------|
| Command | string | 命令 (如 "npx", "python") |
| Args | []string | 命令参数 |
| Env | []string | 额外环境变量 |

### HTTPConfig

| 字段 | 类型 | 说明 |
|------|------|------|
| URL | string | MCP 服务器 URL |
| Proxy | string | HTTP 代理 URL |
| Timeout | time.Duration | 请求超时 (默认 60s) |
| Insecure | bool | 跳过 TLS 验证 |

## 协议版本

当前实现支持 MCP 协议版本 `2024-11-05`。

## 参考资料

- [MCP 官方文档](https://modelcontextprotocol.io/)
- [Datawhale hello-agents - MCP 章节](https://datawhalechina.github.io/hello-agents/#/en/chapter10/Chapter10-Agent-Communication-Protocols)
- [JSON-RPC 2.0 规范](https://www.jsonrpc.org/specification)