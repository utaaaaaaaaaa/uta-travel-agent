# 跨语言 gRPC 通信

## 问题描述

系统使用 Go (Orchestrator) 和 Python (Agent Services) 两种语言，需要实现高效的跨语言通信。

## 解决方案对比

| 方案 | 优点 | 缺点 |
|-----|-----|-----|
| REST API | 简单，广泛支持 | 性能较低，无类型 |
| gRPC | 高性能，类型安全，流式支持 | 需要生成代码 |
| 消息队列 | 解耦，异步 | 复杂度高，延迟 |

## 最终选择

选择 **gRPC** 作为服务间通信协议。

### 理由
1. **性能**: gRPC 使用 Protocol Buffers，比 JSON 更高效
2. **类型安全**: 自动生成客户端和服务端代码
3. **流式支持**: 支持双向流，适合实时导游场景
4. **多语言支持**: Go 和 Python 都有成熟的 gRPC 库

## 实现细节

### Proto 文件定义

```protobuf
syntax = "proto3";

package agent.destination;

service DestinationAgentService {
    rpc Create(CreateAgentRequest) returns (CreateAgentResponse);
    rpc Query(QueryRequest) returns (QueryResponse);
}
```

### 代码生成

使用 `buf` 工具管理 Proto 文件:

```bash
# 安装 buf
go install github.com/bufbuild/buf/cmd/buf@latest

# 生成 Go 代码
buf generate --template buf.gen.yaml

# 生成 Python 代码
buf generate --template buf.gen.python.yaml
```

### Go 服务端

```go
import (
    "google.golang.org/grpc"
    pb "github.com/utaaa/uta-travel-agent/proto/destination"
)

type server struct {
    pb.UnimplementedDestinationAgentServiceServer
}

func (s *server) Create(ctx context.Context, req *pb.CreateAgentRequest) (*pb.CreateAgentResponse, error) {
    // 实现逻辑
}
```

### Python 客户端

```python
import grpc
from proto.destination import destination_pb2, destination_pb2_grpc

channel = grpc.insecure_channel('localhost:50051')
stub = destination_pb2_grpc.DestinationAgentServiceStub(channel)

response = stub.Create(destination_pb2.CreateAgentRequest(
    destination="京都",
    theme="cultural",
))
```

## 性能考量

### 连接池
- 使用 gRPC 连接池复用连接
- 配置最大连接数和空闲超时

### 负载均衡
- 使用 gRPC 内置负载均衡
- 支持多个 Agent Service 实例

### 超时和重试
- 配置合理的超时时间
- 实现指数退避重试

## 踩坑记录

1. **Proto 版本兼容**: 确保所有服务使用相同版本的 Proto 文件
2. **连接泄漏**: 使用连接池并正确关闭连接
3. **大消息处理**: 使用流式 RPC 处理大文件上传