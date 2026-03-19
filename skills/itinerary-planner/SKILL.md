# Itinerary Planner

生成个性化的旅游行程。

## Description

此技能根据用户的时间、偏好、预算等条件，结合知识库信息，生成详细的旅游行程规划。
支持多日行程、路线优化、灵活调整。

## Parameters

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `collection` | string | ✓ | 知识库集合名称 |
| `days` | integer | ✓ | 行程天数 |
| `preferences` | object | | 用户偏好 |
| `constraints` | object | | 约束条件 |

### preferences 子参数
| 参数 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `interests` | array | [] | 兴趣类型：culture, nature, food, shopping |
| `pace` | string | moderate | 节奏：relaxed, moderate, intensive |
| `start_time` | string | 09:00 | 每天开始时间 |
| `end_time` | string | 21:00 | 每天结束时间 |

### constraints 子参数
| 参数 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `budget` | number | null | 总预算（当地货币） |
| `must_visit` | array | [] | 必去景点 |
| `avoid` | array | [] | 避免的景点/区域 |
| `accessibility` | boolean | false | 需要无障碍设施 |

## Usage

用户请求行程规划时调用，需要先有对应目的地的知识库。

### 适用场景
- "帮我规划京都三天行程"
- "我想在东京玩5天，喜欢文化景点"
- "大阪一日游，重点是美食"

## Execution Flow

```
1. 解析用户需求和约束
   ↓
2. 从知识库检索相关景点
   ├── 调用 RAG Query 获取景点信息
   ├── 筛选符合偏好的景点
   └── 评分和排序
   ↓
3. 路线规划
   ├── 按地理位置聚类
   ├── 计算景点间距离/时间
   └── 优化每日路线
   ↓
4. 时间分配
   ├── 估算每个景点停留时间
   ├── 安排用餐时间
   ├── 预留交通时间
   └── 平衡行程密度
   ↓
5. 生成详细行程
   ├── 每日行程安排
   ├── 景点详细信息
   ├── 交通指南
   └── 实用建议
   ↓
6. 预算估算（可选）
   ├── 门票费用
   ├── 餐饮预估
   └── 交通费用
   ↓
7. 返回完整行程
```

## Dependencies

### Skills
- `rag-query`: 检索景点信息

### MCP Tools
- `maps`: 路线规划和距离计算（可选）

### Services
- `llm_gateway`: 生成行程文本

## Output Schema

```json
{
  "itinerary": {
    "destination": "string",
    "total_days": "integer",
    "days": [
      {
        "day": "integer",
        "theme": "string",
        "summary": "string",
        "activities": [
          {
            "time": "string",
            "type": "attraction | meal | transport | rest",
            "name": "string",
            "description": "string",
            "location": {
              "lat": "number",
              "lng": "number",
              "address": "string"
            },
            "duration": "string",
            "estimated_cost": "number",
            "tips": ["string"],
            "booking_required": "boolean"
          }
        ],
        "daily_budget": "number",
        "transport": {
          "method": "string",
          "details": "string"
        }
      }
    ]
  },
  "summary": {
    "total_spots": "integer",
    "estimated_budget": {
      "min": "number",
      "max": "number",
      "currency": "string"
    },
    "highlights": ["string"],
    "tips": ["string"]
  },
  "alternatives": [
    {
      "description": "string",
      "changes": "string"
    }
  ]
}
```

## Examples

### Input
```json
{
  "collection": "kyoto-agent-001",
  "days": 3,
  "preferences": {
    "interests": ["culture", "nature"],
    "pace": "moderate"
  },
  "constraints": {
    "must_visit": ["金阁寺", "清水寺"],
    "budget": 50000
  }
}
```

### Output (部分)
```json
{
  "itinerary": {
    "destination": "京都",
    "total_days": 3,
    "days": [
      {
        "day": 1,
        "theme": "东山文化之旅",
        "summary": "探索京都最具代表性的东山区域",
        "activities": [
          {
            "time": "09:00",
            "type": "attraction",
            "name": "清水寺",
            "description": "京都最古老的寺院，以其悬空的清水舞台闻名",
            "duration": "2小时",
            "estimated_cost": 400,
            "tips": ["建议早到避开人群", "穿舒适的鞋子走坡道"]
          },
          {
            "time": "12:00",
            "type": "meal",
            "name": "二年坂午餐",
            "description": "在传统街道品尝京都料理",
            "duration": "1小时",
            "estimated_cost": 2000
          }
        ]
      }
    ]
  }
}
```

## Notes

- 行程可根据天气、节假日自动调整
- 支持导出为日历格式
- 可保存行程供后续查看和分享