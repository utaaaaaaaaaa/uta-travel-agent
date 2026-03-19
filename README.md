# UTA Travel Agent

Multi-Agent 智能旅游助手系统

## 项目状态

| 服务 | 端口 | 状态 |
|-------|------|--------|
| Orchestrator (Go) | 8080 | 开发中 |
| Destination Agent | 8001 | 开发中 |
| Guide Agent | 8002 | 开发中 |
| Planner Agent | 8003 | 开发中 |
| PostgreSQL | 5432 | 需配置 |
| Redis | 6379 | 运行中 |
| Qdrant | 6333 | 运行中 |
| MinIO | 9000 | 运行中 |

## 快速开始

```bash
# 启动基础设施
docker-compose up -d

# 启动 Go 服务
cd cmd/orchestrator
go run .

# 启动 Python 服务
cd services/destination-agent
uv run main:app --reload

cd services/guide-agent
uv run main:app --reload

cd services/planner-agent
uv run main:app --reload

# 启动前端
cd apps/web
npm run dev
```

并创建其他必要文件。然后完成基础设施配置任务，创建项目文档。最后更新任务列表。现在创建 CI 配置和 README 等文档。让我完成并展示。然后准备推送到 GitHub。现在创建文档和配置。让我先创建 README.md 和更新项目目录结构。继续创建文档和核心流程文档、技术难点文档、然后创建 CI 配置。最终完成推送到 GitHub。让我创建项目文档和配置文件。我需要检查目录结构，然后创建文档和配置文件。先创建 README.md，然后创建文档目录结构说明。让我先查看当前目录结构，然后创建文档。README.md 文档，基础，创建核心流程文档、然后创建技术难点文档。最后创建 CI 配置。然后完成所有文档创建。最后推送到 GitHub。我先检查项目状态。然后检查是否有必要的目录。现在开始创建文档和配置文件。首先创建 README.md。好的，让我查看当前目录结构，然后创建 README.md 文件。README.md 风格需要简洁明了，介绍项目愿景、快速开始指南。安装说明等。README.md 文档应该包含项目介绍、快速开始指南、安装说明等，但 技术栈部分需要列出主要技术栈
    - 技术栈: Go, Python, TypeScript 等
    - 先决条件: Go 1.22+, Python 3.11+, Node.js 18+
    - 开发环境: Docker, Make开发环境一致
- 项目结构需要清晰展示项目目录结构
    - 寽令需要列出目录及其说明
- 安装依赖: 陑要详细说明安装步骤
    - 对于 Go 项目，说明如何初始化 Go 模块和安装依赖
    - 对于 Python 项目，说明如何使用 uv 创建虚拟环境
    - 对于前端项目，说明如何安装依赖和启动开发服务器
- 快速验证: 提供基本功能验证步骤
- 故障排除: 常见问题及解决方案
- 更多信息: 可选，提供链接到更多资源
- 贡献指南: 说明如何为项目做贡献
- 许可证: MIT License
- 联系方式: GitHub Issues

## 许可证

本项目采用 MIT 许可证。

Copyright (c) 2024 UTA Travel Agent Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS provided "as is", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT not limited to the warranties of MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN no event shall the authors or copyright holders be liable for any claim, damages or or other liability, whether in an action of contract, tort or otherwise, arising from, out of or in connection with the Software or the use or other dealings in the Software.
```