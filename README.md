# UTA Travel Agent

**Multi-Agent 智能旅游助手系统 | Intelligent Tourism Assistant System**

[English](#english) | [中文](#中文)

---

## 中文

### 项目简介

UTA (Universal Travel Agent) 是一个基于 Multi-Agent 架构的智能旅游助手系统。核心理念是"Vibecoding"——让技术为体验服务，通过 AI Agent 为用户提供沉浸式的旅游文化体验。

**核心功能**：
- 🗺️ **目的地研究 Agent**: 自动搜索、整理旅游目的地信息，构建 RAG 知识库
- 🎯 **智能导游 Agent**: 实地旅游时，基于位置/图片识别景点，提供文化背景讲解
- 📅 **行程规划 Agent**: 根据用户偏好生成个性化旅游行程
- 💾 **Agent 持久化**: 创建的目的地 Agent 永久保存，用户可随时调用
- 🌍 **多语言支持**: 支持多语言交流和翻译

### 技术栈

| 层级 | 技术 | 用途 |
|------|------|------|
| 前端 | Next.js 15 + TypeScript | 用户界面、Agent 管理 |
| 编排层 | Go + Gin | Agent 调度、任务路由、gRPC 网关 |
| Agent 服务 | Python + gRPC | LLM 调用、RAG、向量检索 |
| 向量数据库 | Qdrant | RAG 知识存储、向量检索 |
| 关系数据库 | PostgreSQL | Agent 元数据持久化 |
| LLM | DeepSeek / Claude API | 大语言模型能力 |

### 快速开始

#### 一键部署 (推荐)

```bash
# 1. 克隆项目
git clone https://github.com/utaaaaaaaaaa/uta-travel-agent.git
cd uta-travel-agent

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env，添加你的 LLM API Key

# 3. 一键启动所有服务
docker-compose up -d --build
```

#### 访问地址

| 服务 | 地址 | 说明 |
|------|------|------|
| 🌐 前端 | http://localhost:3000 | Next.js 应用 |
| 🔌 API | http://localhost:8080 | Go API 服务 |
| 📊 Qdrant | http://localhost:6333/dashboard | 向量数据库面板 |

### 开发模式

如果需要开发调试，可以只启动基础设施：

```bash
# 启动基础设施 (PostgreSQL + Qdrant + Redis)
docker-compose -f docker-compose.dev.yml up -d

# 启动 Go Orchestrator
cd cmd/orchestrator && go run .

# 启动前端
cd apps/web && npm run dev
```

### 项目结构

```
uta-travel-agent/
├── apps/web/                    # 前端 (Next.js)
├── cmd/orchestrator/            # Go 入口
├── internal/                    # Go 包
│   ├── agent/                   # Agent 核心实现
│   ├── router/                  # HTTP 路由
│   ├── grpc/                    # gRPC 客户端
│   └── storage/                 # 存储层
├── services/                    # Python 服务
│   └── embedding/               # Embedding 服务
├── proto/agent/                 # Protocol Buffers
└── docker-compose.yml           # Docker 编排
```

---

## English

### Overview

UTA (Universal Travel Agent) is an intelligent tourism assistant system built on a Multi-Agent architecture. The core philosophy is "Vibecoding" — technology serves experience, providing users with immersive travel and cultural experiences through AI Agents.

**Key Features**:
- 🗺️ **Destination Research Agent**: Automatically searches and organizes travel destination information, builds RAG knowledge bases
- 🎯 **Intelligent Guide Agent**: Provides cultural background explanations based on location/photo recognition during trips
- 📅 **Itinerary Planning Agent**: Generates personalized travel itineraries based on user preferences
- 💾 **Agent Persistence**: Created destination agents are permanently saved and can be invoked anytime
- 🌍 **Multi-language Support**: Supports multi-language communication and translation

### Tech Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| Frontend | Next.js 15 + TypeScript | User interface, Agent management |
| Orchestration | Go + Gin | Agent scheduling, task routing, gRPC gateway |
| Agent Services | Python + gRPC | LLM calls, RAG, vector retrieval |
| Vector DB | Qdrant | RAG knowledge storage, vector retrieval |
| Relational DB | PostgreSQL | Agent metadata persistence |
| LLM | DeepSeek / Claude API | Large language model capabilities |

### Quick Start

#### One-Command Deployment (Recommended)

```bash
# 1. Clone the repository
git clone https://github.com/utaaaaaaaaaa/uta-travel-agent.git
cd uta-travel-agent

# 2. Configure environment variables
cp .env.example .env
# Edit .env and add your LLM API Key

# 3. Start all services with one command
docker-compose up -d --build
```

#### Access URLs

| Service | URL | Description |
|---------|-----|-------------|
| 🌐 Frontend | http://localhost:3000 | Next.js application |
| 🔌 API | http://localhost:8080 | Go API service |
| 📊 Qdrant | http://localhost:6333/dashboard | Vector database dashboard |

### Development Mode

For development and debugging, you can start only the infrastructure:

```bash
# Start infrastructure (PostgreSQL + Qdrant + Redis)
docker-compose -f docker-compose.dev.yml up -d

# Start Go Orchestrator
cd cmd/orchestrator && go run .

# Start Frontend
cd apps/web && npm run dev
```

### Project Structure

```
uta-travel-agent/
├── apps/web/                    # Frontend (Next.js)
├── cmd/orchestrator/            # Go entrypoint
├── internal/                    # Go packages
│   ├── agent/                   # Agent core implementation
│   ├── router/                  # HTTP routing
│   ├── grpc/                    # gRPC clients
│   └── storage/                 # Storage layer
├── services/                    # Python services
│   └── embedding/               # Embedding service
├── proto/agent/                 # Protocol Buffers
└── docker-compose.yml           # Docker compose
```

---

## License

MIT License

Copyright (c) 2024 UTA Travel Agent Contributors