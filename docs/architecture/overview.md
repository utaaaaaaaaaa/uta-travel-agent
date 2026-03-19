# System Architecture

## Overview

UTA Travel Agent 采用分层架构设计，由以下组件组成：

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Frontend       │     │  Orchestrator   │     │  Agent Services │
│   (TypeScript)   │     │     (Go)        │     │    (Python)     │
└─────────────────┘     └─────────────────┘     └─────────────────┘
         │                       │                      │
         ▼                       ▼                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Storage Layer                                │
│  PostgreSQL  │  Redis  │  Qdrant  │  MinIO                    │
└─────────────────────────────────────────────────────────────────────┘
```

## Components

### Frontend (TypeScript + Next.js)
- 用户界面
- Agent 管理界面
- 实时导游界面

### Orchestrator (Go)
- Agent 注册和生命周期管理
- 任务调度
- gRPC 网关
- HTTP API

### Agent Services (Python)
- **Destination Agent**: 目的地研究， RAG 知识库构建
- **Guide Agent**: 实时导游，视觉识别
- **Planner Agent**: 行程规划

## Data Flow

1. 用户通过前端创建目的地 Agent
2. Orchestrator 分配任务给 Destination Agent
3. Destination Agent 研究目的地信息并构建知识库
4. 知识库存储在 Qdrant 中
5. 元数据存储在 PostgreSQL 中
6. 用户实地旅游时加载 Guide Agent
7. Guide Agent 从 Qdrant 检索相关信息
8. 生成讲解内容返回给用户