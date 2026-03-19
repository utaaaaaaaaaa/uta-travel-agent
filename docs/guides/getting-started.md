# 快速开始指南

## 禂述

本指南帮助你快速开始 UTA Travel Agent 开发。

## 如念要求

- Go 1.22+
- Python 3.11+
- Node.js 18+
- Docker & Docker Compose

## 开发环境设置

### 1. 克隆项目

```bash
git clone https://github.com/utaaa/uta-travel-agent.git
cd uta-travel-agent
```

### 2. 启动基础设施

```bash
cd infra/docker
docker-compose up -d
```

这将启动:
- PostgreSQL (端口 5432)
- Redis (端口 6379)
- Qdrant (端口 6333)
- MinIO (端口 9000,9001)

### 3. 启动后端服务

```bash
# Go Orchestrator
cd cmd/orchestrator
go run main.go

# Python Agent Services
cd services/destination-agent
uv run main:app --reload

cd ../guide-agent
uv run main:app --reload

cd ../planner-agent
uv run main:app --reload
```

### 4. 启动前端

```bash
cd apps/web
npm install
npm run dev
```

## 访问服务

- 前端: http://localhost:3000
- Orchestrator: http://localhost:8080
- Destination Agent: http://localhost:8001
- Guide Agent: http://localhost:8002
- Planner Agent: http://localhost:8003

## 环境变量

创建 `.env` 文件:

```bash
# Python services
ANTHROPIC_API_KEY=your_api_key
QDRANT_HOST=localhost
QDRANT_PORT=6333
```

## 下一步

- 查看 [API 文档](../api/rest.md) 了解 API 接口
- 查看 [核心流程](../core-flows/agent-creation.md) 了解 Agent 创建流程