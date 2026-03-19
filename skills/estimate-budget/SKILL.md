# Estimate Budget

估算旅游预算。

## Description

此技能根据行程安排和用户偏好，估算旅游费用。
包括交通、住宿、餐饮、门票、购物等各项费用。

## Parameters

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `itinerary` | object | ✓* | 行程安排（与 destination 二选一） |
| `destination` | string | ✓* | 目的地名称 |
| `travelers` | integer | | 旅行人数，默认 1 |
| `days` | integer | | 天数（无 itinerary 时需要） |
| `style` | string | | 旅游风格：budget, moderate, luxury |
| `includes` | array | | 包含项目 |
| `excludes` | array | | 排除项目 |

### includes 可选值
- `flights` - 往返机票
- `accommodation` - 住宿
- `meals` - 餐饮
- `transport` - 当地交通
- `tickets` - 景点门票
- `shopping` - 购物预算
- `insurance` - 旅游保险

## Usage

在行程规划后调用，或单独用于预算估算。

### 适用场景
- "京都三天大概要花多少钱？"
- "帮我估算这趟行程的预算"
- "两人去东京5天，中等消费要多少？"

## Output Schema

```json
{
  "currency": "string",
  "total": {
    "min": "number",
    "max": "number",
    "average": "number"
  },
  "breakdown": {
    "flights": {
      "min": "number",
      "max": "number",
      "note": "string"
    },
    "accommodation": {
      "min": "number",
      "max": "number",
      "note": "string",
      "per_night": "number"
    },
    "meals": {
      "min": "number",
      "max": "number",
      "note": "string",
      "per_day": "number"
    },
    "transport": {
      "min": "number",
      "max": "number",
      "note": "string"
    },
    "tickets": {
      "min": "number",
      "max": "number",
      "details": ["string"]
    },
    "misc": {
      "min": "number",
      "max": "number",
      "note": "string"
    }
  },
  "tips": ["string"],
  "money_saving_tips": ["string"]
}
```

## Examples

### Input
```json
{
  "destination": "京都",
  "days": 3,
  "travelers": 2,
  "style": "moderate",
  "includes": ["accommodation", "meals", "transport", "tickets"]
}
```

### Output
```json
{
  "currency": "JPY",
  "total": {
    "min": 45000,
    "max": 75000,
    "average": 60000
  },
  "breakdown": {
    "accommodation": {
      "min": 15000,
      "max": 30000,
      "note": "2晚，经济型到中档酒店",
      "per_night": 10000
    },
    "meals": {
      "min": 12000,
      "max": 20000,
      "note": "3天，含早餐、午餐、晚餐",
      "per_day": 5000
    },
    "transport": {
      "min": 3000,
      "max": 8000,
      "note": "巴士一日券、出租车"
    },
    "tickets": {
      "min": 5000,
      "max": 8000,
      "details": [
        "金阁寺: ¥400 x 2",
        "清水寺: ¥400 x 2",
        "伏见稻荷: 免费",
        "岚山小火车: ¥600 x 2"
      ]
    },
    "misc": {
      "min": 10000,
      "max": 15000,
      "note": "纪念品、零食、应急金"
    }
  },
  "tips": [
    "购买京都巴士一日券可节省交通费",
    "午餐选择定食套餐性价比更高",
    "部分寺庙傍晚门票打折"
  ],
  "money_saving_tips": [
    "住青年旅舍可节省 50% 住宿费",
    "便利店早餐约 ¥300-500",
    "使用 Kansai Thru Pass 覆盖更多交通"
  ]
}
```

## Notes

- 价格基于历史数据和当地物价估算
- 季节性因素会影响价格（樱花季、红叶季较贵）
- 建议预留 10-20% 应急预算