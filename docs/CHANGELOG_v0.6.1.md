# v0.6.1-alpha 更新日志

**发布日期**: 2026-03-23

## 概述

本次更新实现了 GuideAgent Session 持久化与前端多会话分组显示功能，解决了导游 Agent 对话记忆丢失的问题，并优化了聊天界面的会话管理体验。

---

## 新增功能

### 1. GuideAgent Session 持久化

**问题**: 之前 Guide 页面使用 `/api/v1/agents/{id}/chat/stream` 直接对话，该 API 不保留对话历史，导致导游 Agent 没有记忆。

**解决方案**:
- 新增 `POST /api/v1/agents/{id}/sessions` API，为指定导游 Agent 创建会话
- 新增 `agent_id` 查询参数过滤 sessions
- Guide 页面现在使用 `/api/v1/sessions/{id}/chat/stream` 进行对话，保留完整对话历史

**数据流**:
```
用户进入导游页面
    → 检查是否有该 Agent 的 sessions
    → 有则加载最近一个 session
    → 无则创建新 session
    → 对话历史持久化到 PostgreSQL
```

### 2. /chat 页面 Session 分组显示

**功能**: 将 sessions 按 Agent 类型分组显示，用户一目了然。

**UI设计**:
```
┌─────────────────────────┐
│ 🤖 旅行助手        (3) ▼ │
│   ├─ 规划行程问题        │
│   ├─ 推荐目的地          │
│   └─ 一般问答            │
├─────────────────────────┤
│ 📍 厦门            (2) ▼ │
│   ├─ 鼓浪屿美食          │
│   └─ 住宿推荐            │
├─────────────────────────┤
│ 📍 杭州            (1) ▼ │
│   └─ 西湖游览            │
└─────────────────────────┘
```

**特点**:
- 按 Agent 类型分组（旅行助手 / 各地导游）
- 显示每个分组的会话数量
- 可折叠/展开每个分组
- 点击会话继续对应导游对话

### 3. 动态 Agent 名称显示

**功能**: 聊天界面根据 session 类型显示正确的 Agent 名称。

- Header 显示正确的导游名字（如"杭州导游助手"）
- 消息列表显示对应的 Agent 名称
- 图标根据类型变化：导游显示 📍，普通助手显示 🤖

---

## API 变更

### 新增 API

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/agents/{id}/sessions` | POST | 为指定 Agent 创建会话 |

### 修改 API

| 端点 | 变更 |
|------|------|
| `GET /api/v1/sessions` | 新增 `agent_id` 查询参数过滤 |
| `GET /api/v1/sessions/{id}` | 返回值新增 `agent_id` 字段 |

---

## 文件变更

### 后端

| 文件 | 变更 |
|------|------|
| `cmd/api-gateway/main.go` | 新增 `createSessionForAgent`，修改 `listSessions` 支持 agent_id 过滤 |
| `internal/session/session.go` | `Snapshot` 结构体新增 `AgentID` 字段 |
| `internal/session/storage.go` | `ListOptions` 新增 `AgentID` 字段，`List()` 方法支持按 agent_id 过滤 |

### 前端

| 文件 | 变更 |
|------|------|
| `apps/web/src/app/guide/[id]/page.tsx` | 新增 Tab 切换、Session 管理、自动加载/创建 session |
| `apps/web/src/app/chat/page.tsx` | 新增 Agent 分组显示、动态 Agent 名称、修复 markdown 数字列表 |
| `apps/web/src/types/session.ts` | `Session` 接口新增 `agent_id` 字段 |

---

## Bug 修复

1. **Markdown 数字列表显示问题**: 修复 `/chat` 页面数字列表丢失原始编号的问题
2. **Session 删除后无法对话**: 删除所有 session 后发送消息自动创建新 session
3. **首次进入页面消息不显示**: 优化 session 初始化逻辑，确保欢迎消息正确显示

---

## 测试验证

```bash
# 创建 agent session
curl -X POST http://localhost:8080/api/v1/agents/{agent_id}/sessions

# 列出 agent 的 sessions
curl http://localhost:8080/api/v1/sessions?agent_id={agent_id}

# session chat
curl -X POST http://localhost:8080/api/v1/sessions/{session_id}/chat/stream
```

---

## 下一步计划

- [ ] Session 标题自动生成（基于对话内容）
- [ ] Session 搜索功能
- [ ] Session 归档/恢复