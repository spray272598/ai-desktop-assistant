# 快速开始

## 环境要求

- Go 1.22+
- （可选）MySQL 8、Redis 7、Docker Desktop、Node 18+、Chrome（浏览器工具）

## 1. 最简 Mock 模式（无外部依赖）

```bash
cd D:\project_go\ai-desktop-assistant

# MCP demo + 主服务
go build -o mcp-demo.exe ./cmd/mcp-demo
go build -o assistant.exe ./cmd/server

# Windows PowerShell
$env:LLM_USE_MOCK="true"
$env:DB_TYPE="memory"
$env:REDIS_ENABLED="false"
.\assistant.exe -config configs/config.yaml
```

打开：http://localhost:8080/

> 若尚未 `npm run build`，会回退到 `web/index.html`（Vite 入口）或旧静态页。建议构建 React：

```bash
cd web
npm install
npm run build
cd ..
# 重启 assistant，将优先托管 web/dist
```

## 2. Docker 全家桶（MySQL + Redis + App）

```bash
# 启动 Docker Desktop 后
docker compose up -d --build
```

| 服务 | 地址 |
|------|------|
| API/UI | http://localhost:8080 |
| MySQL | localhost:3306 / root / 123456 |
| Redis | localhost:6379 |

真实 LLM：

```powershell
$env:LLM_API_KEY="sk-xxx"
$env:LLM_USE_MOCK="false"
docker compose up -d --build
```

## 3. 常用 API

```bash
# 健康检查
curl http://localhost:8080/health

# 创建会话
curl -X POST http://localhost:8080/api/v1/session/create \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"u1\",\"title\":\"demo\"}"

# 对话（开发可 autoApprove）
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d "{\"sessionId\":\"...\",\"userId\":\"u1\",\"message\":\"list files\",\"autoApprove\":true}"

# 导出
curl "http://localhost:8080/api/v1/session/export?sessionId=...&format=md"

# 插件市场
curl http://localhost:8080/api/v1/mcp/market
```

## 4. WebSocket 设备（手机遥控骨架）

设备端连接：

```
ws://localhost:8080/ws?role=device&deviceId=phone-1
```

控制台连接：

```
ws://localhost:8080/ws?role=console
```

发送 JSON：

```json
{"type":"command","deviceId":"phone-1","action":"ping","content":"hello"}
```

## 5. 桌面壳

```bash
go run ./cmd/desktop
# 打开系统浏览器指向控制台；正式方案见 docs/features.md 中 Wails 章节
```

## 6. 目录导读

```
cmd/server     HTTP 入口
cmd/mcp-demo   示例 MCP stdio 服务
cmd/desktop    桌面壳占位
web/           React 控制台源码
internal/domain/agent  智能体核心
docs/          学习与面试文档
```

## 7. 常见问题

| 现象 | 处理 |
|------|------|
| MySQL 连不上 | `DB_TYPE=memory` 或启动 docker mysql |
| MCP 无工具 | 先 `go build -o mcp-demo.exe ./cmd/mcp-demo` |
| 浏览器工具失败 | 安装 Chrome；或依赖 HTTP 降级 |
| 沙箱失败 | 安装 Docker 或本地 python/node |
| 截图失败 | 桌面环境限制；容器内无法截宿主机 |
