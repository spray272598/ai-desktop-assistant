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

## P3

### 插件市场
- `GET /api/v1/mcp/market`  
- `POST /api/v1/mcp/market/install|uninstall` `{"id":"fs"}`  
- 预置：demo / filesystem / fetch / memory（后三者需 npx）

### 多 Agent 编排
- `MultiAgentOrchestrator`：Router → Planner → Executor hint  
- 注入 System Prompt；SSE 事件 `route` / `plan`

### A/B 模型切换
- `config.yaml` → `llm.models`  
- `ModelRouter.Select(scenario)` 加权轮询  
- `GET /api/v1/models` 查看路由表

## API 一览（节选）

| Method | Path | 说明 |
|--------|------|------|
| GET | /health | 健康检查 |
| POST | /api/v1/chat | 对话 |
| POST | /api/v1/chat/stream | SSE |
| GET | /api/v1/tools | 工具列表 |
| GET/POST | /api/v1/mcp/servers | MCP 热加载 |
| GET | /api/v1/mcp/market | 插件市场 |
| GET | /api/v1/session/export | 导出 |
| POST | /api/v1/session/import | 导入 |
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
