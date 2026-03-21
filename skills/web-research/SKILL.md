---
name: web-research
description: |
  执行网络搜索和网页内容提取。当用户需要查找任何外部信息时使用此 skill。
  包括：实时信息（价格、天气、新闻、活动）、权威知识（维基百科、百度百科）、
  读取特定网页内容、翻译查询、文化背景研究。
  关键词：搜索、查询、天气、价格、新闻、网页、实时、最新、查找、查一下、搜一下。
  触发词：帮我查、搜索一下、找一下、看看、了解、查查。
  Make sure to use this skill whenever the user asks for information that might require looking up online,
  even if they don't explicitly say "search" - phrases like "查一下", "多少钱", "什么时候", "怎么样" should trigger this.
compatibility: Requires Python 3, network access, TAVILY_API_KEY for Tavily search
allowed-tools: bash read python
---

# 网络搜索指南

## 何时使用

当用户询问以下类型问题时，立即使用此 skill：

- **价格查询**: 门票、酒店、机票、物价
- **天气查询**: 今天/明天/本周天气
- **新闻资讯**: 最新消息、活动、节庆
- **知识查询**: 历史、地理、文化、科学
- **网页内容**: 读取特定 URL
- **实时信息**: 任何需要最新数据的查询

## 可用脚本

| 脚本 | 用途 | 最佳场景 |
|------|------|----------|
| `wikipedia_search.py` | 维基百科 | 历史、地理、文化、科学知识 |
| `baidu_baike_search.py` | 百度百科 | 中文内容、中国相关 |
| `tavily_search.py` | 实时搜索 | 价格、天气、新闻、活动 |
| `web_reader.py` | 网页读取 | 读取特定 URL 内容 |

## 决策流程

```
用户问题
    │
    ├── 需要实时信息? ──────────► tavily_search.py
    │   (价格/天气/新闻/活动)
    │
    ├── 中文主题? ──────────────► baidu_baike_search.py
    │   (中国相关/中文内容)
    │
    ├── 权威知识? ──────────────► wikipedia_search.py
    │   (历史/地理/文化)
    │
    ├── 有具体 URL? ────────────► web_reader.py
    │
    └── 复杂查询? ──────────────► 组合使用多个脚本
```

## 使用方法

### 1. 维基百科搜索

```bash
python scripts/wikipedia_search.py "query" --lang zh
```

| 参数 | 说明 |
|------|------|
| `--lang en` | 英文维基 (默认) |
| `--lang zh` | 中文维基 |
| `--lang ja` | 日文维基 |

**示例:**
```bash
python scripts/wikipedia_search.py "清水寺" --lang zh
python scripts/wikipedia_search.py "Kyoto temples" --lang en
```

### 2. 百度百科 (中文内容)

```bash
python scripts/baidu_baike_search.py "中文查询"
```

**示例:**
```bash
python scripts/baidu_baike_search.py "故宫博物院"
python scripts/baidu_baike_search.py "苏州园林"
```

### 3. 实时搜索 (Tavily)

```bash
python scripts/tavily_search.py "实时查询"
```

**需要**: `TAVILY_API_KEY` 环境变量

**示例:**
```bash
python scripts/tavily_search.py "京都 门票价格 2024"
python scripts/tavily_search.py "东京 天气预报 明天"
python scripts/tavily_search.py "2024 巴黎奥运会 最新消息"
```

### 4. 网页读取

```bash
python scripts/web_reader.py "https://example.com"
```

**支持**: HTTP_PROXY / HTTPS_PROXY 环境变量

**示例:**
```bash
python scripts/web_reader.py "https://www.kiyomizudera.or.jp/"
```

## 搜索策略

### 关键词优化

```
❌ 差: "京都"
✅ 好: "京都 门票价格 2024"

❌ 差: "天气"
✅ 好: "京都天气预报 3月"

❌ 差: "美食"
✅ 好: "京都 必吃料理 推荐 2024"
```

### 多工具协作

对于复杂查询，组合使用工具：

1. **先用百科** 获取背景知识
2. **再用实时搜索** 获取最新信息
3. **最后用网页读取** 深入详情

## 示例场景

### 场景1: 查询景点门票

```bash
# 用户: "清水寺门票多少钱?"

# 步骤1: 实时搜索价格
python scripts/tavily_search.py "清水寺 门票价格 2024"

# 步骤2: 如果需要背景
python scripts/wikipedia_search.py "清水寺" --lang zh
```

### 场景2: 研究中国文化

```bash
# 用户: "介绍一下故宫的历史"

# 使用百度百科 (中文内容丰富)
python scripts/baidu_baike_search.py "故宫博物院"

# 补充维基百科
python scripts/wikipedia_search.py "Forbidden City" --lang en
```

### 场景3: 查询天气

```bash
# 用户: "明天京都天气怎么样?"

python scripts/tavily_search.py "京都 天气预报 明天"
```

### 场景4: 读取官方网站

```bash
# 用户: "帮我看看清水寺官网有什么信息"

python scripts/web_reader.py "https://www.kiyomizudera.or.jp/"
```

## 输出格式

所有脚本返回统一 JSON 格式：

```json
{
  "success": true,
  "results": [
    {
      "title": "...",
      "url": "...",
      "content": "..."
    }
  ]
}
```

## 结果验证

### 来源优先级

1. 官方网站 (最可靠)
2. 权威媒体
3. 百科全书
4. 社交媒体/博客 (谨慎使用)

### 时效性检查

| 信息类型 | 注意事项 |
|----------|----------|
| 价格 | 标注日期和来源 |
| 天气 | 标注更新时间 |
| 活动 | 确认年份 |
| 开放时间 | 可能有季节变化 |

## 错误处理

| 错误 | 解决方案 |
|------|----------|
| 网络超时 | 检查代理设置，重试 |
| 无结果 | 换关键词，换工具 |
| API Key 缺失 | 设置 TAVILY_API_KEY |

## 详细参考

更多搜索技巧参见 [references/search-strategies.md](references/search-strategies.md)