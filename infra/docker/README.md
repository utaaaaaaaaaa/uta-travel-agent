# UTA Travel Agent - Docker 部署

## 快速开始 (开发环境)

### 方式 1: 使用 Mock LLM (无需 API Key)

```bash
cd infra/docker
docker-compose -f docker-compose.dev.yml up -d
```

### 方式 2: 使用真实 LLM

```bash
# 创建 .env 文件
cp ../../.env.example .env

# 编辑 .env，设置你的 API Key 和选择 Provider
# LLM_PROVIDER=glm  # 可选: mock, anthropic, openai, glm, deepseek
# GLM_API_KEY=your-key

# 启动
docker-compose -f docker-compose.dev.yml up -d
```

服务地址：
- **前端**: http://localhost:3000
- **API Gateway**: http://localhost:8080
- **LLM Gateway**: localhost:50051 (gRPC)
- **Qdrant**: http://localhost:6333

## 支持的 LLM Provider

| Provider | LLM_PROVIDER 值 | 环境变量 | 模型 |
|----------|-----------------|----------|------|
| Mock (测试) | `mock` | 无需 | - |
| 智谱 GLM | `glm` | `GLM_API_KEY` | glm-4-flash |
| OpenAI | `openai` | `OPENAI_API_KEY` | gpt-4o-mini |
| DeepSeek | `deepseek` | `DEEPSEEK_API_KEY` | deepseek-chat |
| Anthropic | `anthropic` | `ANTHROPIC_API_KEY` | claude-sonnet-4 |

## 环境变量

```bash
# LLM Provider (必选)
LLM_PROVIDER=glm  # mock, anthropic, openai, glm, deepseek

# API Keys (根据 provider 选择)
GLM_API_KEY=your-glm-key
OPENAI_API_KEY=your-openai-key
DEEPSEEK_API_KEY=your-deepseek-key
ANTHROPIC_API_KEY=your-anthropic-key
```

## 服务架构

```
┌─────────────────────────────────────────────────────────────┐
│  Frontend (Next.js:3000)                                    │
│    ↓ HTTP                                                   │
│  API Gateway (Go:8080)                                      │
│    ↓ gRPC                                                   │
│  LLM Gateway (Python:50051)                                 │
│    ↓ HTTP API                                               │
│  ┌────────────┬────────────┬────────────┬────────────┐     │
│  │   GLM      │  OpenAI    │ DeepSeek   │ Anthropic  │     │
│  │  智谱AI    │            │            │   Claude   │     │
│  └────────────┴────────────┴────────────┴────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

## 单独构建镜像

```bash
# API Gateway (Go)
docker build -f api-gateway.Dockerfile -t uta-api-gateway ../..

# LLM Gateway (Python)
docker build -f llm-gateway.Dockerfile -t uta-llm-gateway ../..

# Frontend (Next.js)
docker build -f web.Dockerfile -t uta-web ../..
```

## 本地开发 (不用 Docker)

```bash
# 1. 启动存储服务
docker run -d -p 6379:6379 redis:7-alpine
docker run -d -p 6333:6333 -p 6334:6334 qdrant/qdrant

# 2. 设置环境变量
export LLM_PROVIDER=glm
export GLM_API_KEY=your-key

# 3. 启动 Python LLM Gateway
cd services/llm-gateway
uv venv && source .venv/bin/activate
uv pip install -e .
python -m src.grpc_service

# 4. 启动 Go API Gateway
go run cmd/api-gateway/main.go

# 5. 启动前端
cd apps/web && pnpm dev
```

## 测试 API

```bash
# 健康检查
curl http://localhost:8080/health

# 聊天
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "你好"}'

# 创建目的地 Agent
curl -X POST http://localhost:8080/api/v1/agent/create \
  -H "Content-Type: application/json" \
  -d '{"destination": "京都", "theme": "cultural", "languages": ["zh"]}'
```

## 故障排查

```bash
# 查看容器状态
docker-compose -f docker-compose.dev.yml ps

# 查看日志
docker-compose -f docker-compose.dev.yml logs -f llm-gateway
docker-compose -f docker-compose.dev.yml logs -f api-gateway

# 重新构建
docker-compose -f docker-compose.dev.yml build --no-cache llm-gateway

# 进入容器
docker exec -it uta-llm-gateway bash
```