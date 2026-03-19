# MVP 快速开始

## 启动开发环境

```bash
# 进入 docker 目录
cd infra/docker

# 启动服务 (仅 Qdrant + Destination Agent)
docker-compose -f docker-compose.dev.yml up -d

# 查看日志
docker-compose -f docker-compose.dev.yml logs -f destination-agent

# 停止服务
docker-compose -f docker-compose.dev.yml down
```

## 本地开发 (不用 Docker)

```bash
# 1. 启动 Qdrant
docker run -d -p 6333:6333 -p 6334:6334 qdrant/qdrant

# 2. 进入服务目录
cd services/destination-agent

# 3. 创建虚拟环境并安装依赖
uv venv
source .venv/bin/activate
uv pip install -e ".[dev]"

# 4. 设置环境变量
export QDRANT_HOST=localhost
export QDRANT_PORT=6333
export ANTHROPIC_API_KEY=your_key_here  # 可选

# 5. 启动服务
uvicorn src.main:app --reload --port 8001
```

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /health | 健康检查 |
| POST | /agents | 创建 Agent |
| GET | /agents?user_id=xxx | 列出 Agent |
| GET | /agents/{id} | 获取 Agent 详情 |
| DELETE | /agents/{id} | 删除 Agent |
| POST | /agents/{id}/query | 查询 Agent |

## 测试创建 Agent

```bash
# 创建京都导游 Agent
curl -X POST http://localhost:8001/agents \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test-user",
    "destination": "京都",
    "theme": "cultural",
    "languages": ["zh"]
  }'

# 查询 Agent 状态 (返回的 agent_id)
curl http://localhost:8001/agents/{agent_id}

# 当 status=ready 时，可以查询
curl -X POST http://localhost:8001/agents/{agent_id}/query \
  -H "Content-Type: application/json" \
  -d '{"question": "京都有哪些著名的寺庙？"}'
```

## 前端开发

```bash
cd apps/web
npm install
npm run dev
```

访问 http://localhost:3000