# Multi-Agent Design

## 概述

UTA Travel Agent 采用 Multi-Agent 架构，不同类型的 Agent 协作完成复杂任务。

## Agent 类型

### Destination Agent
负责创建目的地知识库:
- **Researcher Sub-Agent**: 搜索网络信息
- **Curator Sub-Agent**: 整理和结构化信息
- **Indexer Sub-Agent**: 创建向量索引

### Guide Agent
负责实时导游:
- **Vision Sub-Agent**: 图像识别
- **Location Sub-Agent**: 位置服务
- **Narrator Sub-Agent**: 讲解生成

### Planner Agent
负责行程规划:
- **Route Optimizer**: 路线优化
- **Scheduler**: 时间安排
- **Recommender**: 活动推荐

## Agent 通信

Agent 之间通过 gRPC 进行通信:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Orchestrator │────▶│  Agent A     │────▶│  Agent B     │
└─────────────┘     └─────────────┘     └─────────────┘
```

## Agent 生命周期

1. **创建**: Agent 被创建并初始化
2. **运行**: Agent 执行任务
3. **持久化**: Agent 状态被保存
4. **调用**: Agent 被加载和重用
5. **归档**: Agent 不再使用但保留数据