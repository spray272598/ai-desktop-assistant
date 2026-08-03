# 功能全景与实现说明

## P0

### 浏览器自动化
- 配置：`tools.browser.enabled`
- 实现：`BrowserService`（chromedp）+ 工具  
  - `open_url` / `browser_html` / `browser_screenshot` / `browser_eval`
- 降级：无 Chrome 时 HTTP GET 抓取正文

### React 控制台
- 源码：`web/`（Vite + React + TS）
- 构建：`cd web && npm i && npm run build` → `web/dist`
- 能力：会话、聊天、工具、MCP、插件市场、权限、导出、模型路由展示

## P1

### Redis
- 配置：`redis.enabled`
- 用途：会话最近消息缓存 key=`sess:last:{id}`；聊天限流 `rl:chat:{userId}`
- 不可用自动降级内存限流

### 代码沙箱
- 工具：`run_code`（language=python|javascript）
- Docker：`--network none --memory 128m`  
- 降级：本地临时目录 + python/node

### 截图
- 主路径：`github.com/kbinani/screenshot`（纯 Go）
- 降级：Windows PowerShell / macOS screencapture / Linux scrot 等

## P2

### Wails 桌面壳
- `cmd/desktop`：当前打开系统浏览器指向控制台  
- 正式方案：`wails init` 后 frontend 指向 `web/`，后端仍用本仓库 HTTP API

### WebSocket 设备控制
- `GET/WS /ws?role=device&deviceId=...`  
- 控制台 `role=console` 下发 `command`  
- `GET /api/v1/devices` 查看在线设备

### 对话导出/导入
- `GET /api/v1/session/export?sessionId=&format=json|md`  
- `POST /api/v1/session/import` `{"path":"./exports/xxx.json"}`  
- 落盘目录：`./exports`

## P3（秋招核心：扩展生态 + Agent 主流程）

### MCP 安装生命周期
- 自定义安装：`POST /api/v1/mcp/install` 或 `POST /api/v1/mcp/servers`  
  body: `{name, transport:stdio|sse, command, args, env, url}`  
- 市场安装：`POST /api/v1/mcp/market/install` `{"id":"fetch"}`  
- 健康：`GET /api/v1/mcp/health`（online / toolCount / lastError）  
- 工具：`GET /api/v1/mcp/tools?server=fs`  
- 热同步：安装后 `OnToolsChanged` → ToolRegistry；下一轮对话可用  
- 重启恢复：DB `mcp_server_config` + bootstrap 合并 yaml  

### Skills（SKILL.md 工作流）
- 目录：`skills/<id>/SKILL.md`（frontmatter + 执行指南）  
- `GET /api/v1/skills` · `POST /api/v1/skills/install` `{"path":"..."}`  
- `POST /api/v1/skills/uninstall` · `POST /api/v1/skills/reload`  
- 运行时：匹配触发词 → SSE `skill` → 注入 system prompt → 可选工具子集  

### Agent 主流程增强
- 权限恢复：`CreatePending` → `Approve` 标记 Ready →「继续」或 `approve?continue=true` 先执行工具再 ReAct  
- 计划推进：`TaskPlan.AdvanceWithTool` → SSE `plan_update`  
- 工具结果预算：截断 4000 字符防 context 爆炸  
- 失败分类：error 事件 `subType` = `llm|tool|mcp|permission|loop|cancel`  

### 插件市场
- 预置：demo / fs / fetch / memory / brave-search  

### 多 Agent 编排
- Router → Planner → Executor hint；SSE `route` / `plan`  

### A/B 模型切换
- `GET /api/v1/models`  

## API 一览（节选）

| Method | Path | 说明 |
|--------|------|------|
| GET | /health | 健康检查 |
| POST | /api/v1/chat | 对话 |
| POST | /api/v1/chat/stream | SSE |
| GET | /api/v1/tools | 工具列表（含 MCP 热装） |
| GET/POST | /api/v1/mcp/servers | 列表 / 安装 |
| POST | /api/v1/mcp/install | 自定义安装 |
| GET | /api/v1/mcp/health | MCP 健康 |
| GET | /api/v1/mcp/tools | MCP 工具（可按 server 过滤） |
| GET | /api/v1/mcp/market | 插件市场 |
| POST | /api/v1/permission/approve | 批准；`continue:true` 自动恢复 |
| GET | /api/v1/skills | Skill 列表 |
| POST | /api/v1/skills/install | 从路径安装 Skill |
| GET | /api/v1/session/export | 导出 |
| GET | /api/v1/models | 模型路由 |
| WS | /ws | 设备/控制台 |

## 配置开关速查

| 配置 | 含义 |
|------|------|
| `tools.browser.enabled` | 浏览器工具 |
| `tools.screenshot.enabled` | 截图工具 |
| `sandbox.use_docker` | 沙箱走 Docker |
| `redis.enabled` | Redis |
| `rate_limit.per_minute` | 每用户每分钟请求数 |
| `mcp.enabled` | MCP 总开关 |
