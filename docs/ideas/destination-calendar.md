# 待办想法：目的地节日日历功能

## 概述

为目的地 Agent 添加日历功能，展示当地未来1个月或几天的重大节日。

## 功能描述

### 核心功能
- 通过实时搜索获取当地重大节日信息
- 日历视图展示节日日期
- 点击节日展示文化背景、风俗习惯

### 用户场景
- 用户查看目的地 Agent 时，可以看到"近期节日"卡片
- 点击某节日，查看详细介绍、文化背景、风俗习惯
- 帮助用户规划行程，避免或参与当地节日

### 技术实现

```
Destination Agent Page
├── 景点卡片
├── 美食卡片
├── 节日日历卡片(新增)
│   ├── 日历视图
│   ├── 节日标记
│   └── 点击展开详情
└── 实用信息
```

### API 设计

```
GET /api/v1/agents/{id}/events?months=1

Response:
{
  "events": [
    {
      "date": "2026-04-04",
      "name": "清明节",
      "type": "traditional",
      "description": "祭祖扫墓的传统节日",
      "cultural_background": "...",
      "customs": ["扫墓", "踏青", "放风筝"],
      "impact_on_travel": "部分景点可能人流较多"
    }
  ]
}
```

## 优先级

中等 - 可在v0.6.0 或 v0.7.0 实现

## 相关文件

- `internal/router/router.go` - 添加events API
- `apps/web/src/app/guide/[id]/` - 前端日历组件
- `internal/agent/prompts.go` - Guide Agent 添加节日查询提示