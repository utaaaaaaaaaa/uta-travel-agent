# 搜索策略详解

## 脚本对比

### wikipedia_search.py - 维基百科搜索

**优点:**
- 权威、可靠
- 多语言支持 (en/zh/ja 等)
- 结构化信息
- 无需 API Key

**缺点:**
- 可能不是最新信息
- 某些主题覆盖不足

**最佳场景:**
- 历史事件
- 地理位置
- 文化知识
- 科学概念

**使用示例:**
```bash
# 中文查询
python scripts/wikipedia_search.py "清水寺" --lang zh

# 英文查询
python scripts/wikipedia_search.py "Kyoto temples" --lang en

# 日文查询
python scripts/wikipedia_search.py "京都の寺" --lang ja
```

---

### baidu_baike_search.py - 百度百科 (中文)

**优点:**
- 中文内容丰富
- 中国相关主题详细
- 更新相对及时
- 无需 API Key

**缺点:**
- 仅支持中文
- 内容可能有倾向性

**最佳场景:**
- 中国景点
- 中国历史
- 中文文化
- 国内信息

**使用示例:**
```bash
python scripts/baidu_baike_search.py "故宫博物院"
python scripts/baidu_baike_search.py "苏州园林"
python scripts/baidu_baike_search.py "西湖十景"
```

---

### tavily_search.py - 实时搜索

**优点:**
- 实时结果
- 多来源聚合
- 支持复杂查询
- 适合价格/天气

**缺点:**
- 需要 TAVILY_API_KEY
- 可能有噪音

**最佳场景:**
- 实时价格
- 天气预报
- 新闻资讯
- 活动信息

**使用示例:**
```bash
# 价格查询
python scripts/tavily_search.py "京都 门票价格 2024"

# 天气查询
python scripts/tavily_search.py "东京 天气预报 明天"

# 新闻查询
python scripts/tavily_search.py "2024 巴黎奥运会 最新消息"
```

---

### web_reader.py - 网页读取

**优点:**
- 获取完整内容
- 读取特定页面
- 支持代理
- HTML 自动转文本

**缺点:**
- 需要具体 URL
- 可能遇到访问限制
- 部分网站可能屏蔽

**最佳场景:**
- 读取官方网站
- 获取详细内容
- 验证信息来源

**使用示例:**
```bash
# 读取官方网站
python scripts/web_reader.py "https://www.kiyomizudera.or.jp/"

# 支持代理
HTTP_PROXY=http://127.0.0.1:7890 python scripts/web_reader.py "https://example.com"
```

---

## 选择指南

```
┌─────────────────────────────────────────────────────┐
│                   用户问题                          │
└───────────────────────┬─────────────────────────────┘
                        │
                        ▼
              ┌─────────────────┐
              │ 需要实时信息?   │
              │ (价格/天气/新闻) │
              └────────┬────────┘
                  是 / │ \ 否
                    /  │  \
                   ▼   │   ▼
        tavily_search  │  ┌──────────────┐
                      │  │ 中文主题?    │
                      │  │ (中国相关)   │
                      │  └──────┬───────┘
                      │     是 / \ 否
                      │      /   \
                      │     ▼     ▼
                      │  baidu_  wikipedia
                      │  baike   _search
                      │
                      ▼
              有具体 URL? ──► web_reader
```

---

## 高级技巧

### 1. 关键词优化

| 场景 | 差的关键词 | 好的关键词 |
|------|-----------|-----------|
| 价格 | "门票" | "清水寺 门票价格 2024" |
| 天气 | "天气" | "京都 天气预报 明天" |
| 美食 | "美食" | "京都 必吃料理 推荐 2024" |
| 活动 | "樱花" | "京都 樱花季 2024 最佳时间" |

### 2. 多语言策略

对于日本旅游主题：
```bash
# 中文查询
python scripts/baidu_baike_search.py "清水寺"

# 日文查询 (获取本地视角)
python scripts/wikipedia_search.py "清水寺" --lang ja

# 英文查询 (补充信息)
python scripts/wikipedia_search.py "Kiyomizu-dera" --lang en
```

### 3. 信息交叉验证

```bash
# 步骤1: 百科获取背景
python scripts/baidu_baike_search.py "故宫"

# 步骤2: 实时搜索获取最新信息
python scripts/tavily_search.py "故宫 门票预约 2024"

# 步骤3: 读取官网确认
python scripts/web_reader.py "https://www.dpm.org.cn/"
```

---

## 错误处理

### 网络错误

```bash
# 检查代理
echo $HTTP_PROXY
echo $HTTPS_PROXY

# 设置代理
export HTTP_PROXY=http://127.0.0.1:7890
export HTTPS_PROXY=http://127.0.0.1:7890
```

### 搜索无结果

1. 换关键词 (更通用)
2. 换语言 (中文→英文)
3. 换工具 (百科→实时搜索)

### API Key 问题

```bash
# 检查 TAVILY_API_KEY
echo $TAVILY_API_KEY

# 设置 API Key
export TAVILY_API_KEY="your-api-key"
```

---

## 输出解读

所有脚本返回统一格式：

```json
{
  "success": true,      // 是否成功
  "results": [          // 结果列表
    {
      "title": "...",   // 标题
      "url": "...",     // 来源 URL
      "content": "...", // 内容摘要
      "snippet": "..."  // 简短描述
    }
  ]
}
```

**错误格式:**
```json
{
  "success": false,
  "error": "错误描述"
}
```