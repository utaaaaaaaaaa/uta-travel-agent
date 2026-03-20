# 🚀 快速开始指南

## 一键启动

```bash
# 1. 克隆项目后，创建 .env 文件
cp .env.example .env

# 2. 编辑 .env，添加你的 LLM API Key (可选，不配置则使用 Mock)
# LLM_API_KEY=your_api_key

# 3. 一键启动所有服务
docker-compose up --build
```

## 访问地址

| 服务 | 地址 | 说明 |
|------|------|------|
| 🌐 **前端** | http://localhost:3000 | Next.js 应用 |
| 🔌 **API** | http://localhost:8080 | Go API 服务 |
| 📊 **Qdrant** | http://localhost:6333/dashboard | 向量数据库面板 |
| 🗄️ **PostgreSQL** | localhost:5432 | 数据库 |

## 开发模式

如果需要开发调试，可以只启动基础设施，然后本地运行应用服务：

```bash
# 启动基础设施 (PostgreSQL + Qdrant + Redis)
docker-compose -f docker-compose.dev.yml up -d

# 启动 Go Orchestrator (Terminal 1)
cd cmd/orchestrator && go run .

# 启动前端 (Terminal 2)
cd apps/web && npm run dev
```

## 测试 Agent 创建流程

1. 打开 http://localhost:3000/destinations/create
2. 输入目的地（如"京都"）
3. 选择主题和语言
4. 点击"开始创建"
5. 观察 Agent 创建进度（雷达图动画）
6. 创建完成后进入导游页面

## 常用命令

```bash
# 查看日志
docker-compose logs -f orchestrator
docker-compose logs -f web

# 停止所有服务
docker-compose down

# 停止并清理数据
docker-compose down -v

# 重新构建
docker-compose up --build
```

## 环境变量说明

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `LLM_API_KEY` | LLM API 密钥 | - |
| `LLM_BASE_URL` | LLM API 地址 | GLM API |
| `LLM_MODEL` | 模型名称 | glm-4-flash |
| `DATABASE_HOST` | 数据库地址 | postgres |
| `QDRANT_HOST` | Qdrant 地址 | qdrant |

## 故障排查

### 前端无法连接 API
检查 `NEXT_PUBLIC_API_URL` 是否正确设置为 `http://localhost:8080`

### Agent 创建失败
1. 检查 embedding 服务是否正常：`docker-compose logs embedding`
2. 检查 Qdrant 是否正常：访问 http://localhost:6333/

### 数据库连接失败
1. 检查 PostgreSQL 是否启动：`docker-compose ps`
2. 查看日志：`docker-compose logs postgres`
