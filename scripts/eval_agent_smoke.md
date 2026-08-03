# Agent 冒烟评测集（秋招演示 / 回归）

用 Mock 或真模型跑通。关注：**扩展安装 → 动态工具 → 权限恢复 → Skill 注入 → 计划推进**。

## 前置

```bash
go build -o mcp-demo.exe ./cmd/mcp-demo
go build -o assistant.exe ./cmd/server
# PowerShell
$env:LLM_USE_MOCK="true"; $env:DB_TYPE="memory"
./assistant.exe -config configs/config.yaml
```

## 用例

| ID | 输入 | 期望 |
|----|------|------|
| E1 | 列出工作区文件 | 调用 list_files，有结果；可匹配 workspace-overview Skill |
| E2 | 先安装 fetch MCP 再抓网页（分两步 API） | install 后 `/api/v1/tools` 出现新工具 |
| E3 | write 一个文件到 workspace | permission confirm → approve continue=true → 文件落盘 |
| E4 | 复杂：先列目录然后读 test.txt | 出现 plan 事件，plan_update 推进 |
| E5 | GET /api/v1/mcp/health | 返回 online/toolCount |
| E6 | GET /api/v1/skills | 至少 3 个内置 skill |
| E7 | 发送「继续」且无 pending | 不崩溃，正常回复 |

## API 手测脚本（curl / Invoke-RestMethod）

```bash
# 健康
curl http://localhost:8080/health

# Skills
curl http://localhost:8080/api/v1/skills

# MCP 市场安装 demo
curl -X POST http://localhost:8080/api/v1/mcp/market/install -H "Content-Type: application/json" -d "{\"id\":\"demo\"}"

# 自定义 MCP（示例 filesystem，需 npx）
# curl -X POST http://localhost:8080/api/v1/mcp/install -H "Content-Type: application/json" -d "{\"name\":\"fs\",\"transport\":\"stdio\",\"command\":\"npx\",\"args\":[\"-y\",\"@modelcontextprotocol/server-filesystem\",\"./workspace\"]}"

# 工具列表（应含 MCP 工具）
curl http://localhost:8080/api/v1/tools

# 会话 + 聊天
# curl -X POST http://localhost:8080/api/v1/session/create -d "{\"userId\":\"u1\",\"title\":\"eval\"}"
# curl -X POST http://localhost:8080/api/v1/chat -d "{\"sessionId\":\"...\",\"userId\":\"u1\",\"message\":\"列出文件\"}"
```

## 通过标准（学习项目）

- 安装 MCP 后无需重启即可在下一轮对话使用
- 权限批准 + continue 能执行原工具
- SSE/响应中可见 intent / route / plan / skill / plan_update / permission
- 失败时 error 事件带 class：`llm|tool|mcp|permission|loop`
