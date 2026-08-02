# AI Desktop Assistant

基于 Go 的桌面 AI 助手，采用 DDD 分层 + 运行时智能体（ReAct）。

## 能力（桌面智能助手核心）

| 优先级 | 能力 | 说明 |
|--------|------|------|
| P0 | **会话/消息 MySQL 持久化** | 跨重启对话、里程碑、长期记忆 |
| P0 | **本地工具** | 工作区文件读写、安全命令执行 |
| P0 | **真实 MCP** | stdio / SSE(HTTP) 客户端，工具自动注册进 Agent |
| P0 | **运行时 Agent** | ReAct 循环、意图识别、混合窗口裁剪、动态 Prompt |
| P1 | 截图 | 可选，默认关闭 |

- **Agent Workflow**：Thought → Action → Observation，死循环检测、步数/工具预算
- **意图识别**：规则优先 + LLM 降级，会话上下文追踪与缓存
- **上下文管理**：ContextProvider + HybridReducer + CoreMemory
- **HTTP + SSE**：`/api/v1/chat`、`/api/v1/chat/stream`
- **Docker Compose**：assistant + MySQL

## 快速开始

### 本机运行

```bash
# 依赖
go mod tidy

# Mock 模式（无需 API Key）
go run ./cmd/server -config configs/config.yaml -mode http

# 或 REPL
go run ./cmd/server -mode repl

# 真实 LLM（SiliconFlow / OpenAI 兼容）
set LLM_API_KEY=sk-xxx
set LLM_USE_MOCK=false
go run ./cmd/server
```

### Docker 本地部署（含 MySQL）

```bash
# 构建并启动 assistant + mysql
docker compose up -d --build

# 真实模型
# Windows PowerShell:
$env:LLM_API_KEY="sk-xxx"; $env:LLM_USE_MOCK="false"; docker compose up -d --build
```

服务地址：`http://localhost:8080`  
MySQL：`localhost:3306` / root / 123456 / `ai_desktop_assistant`

### 本机开发（MySQL + MCP demo）

```bash
# 1) 起 MySQL（可用 compose 只起库）
docker compose up -d mysql

# 2) 构建 MCP demo 与主程序
go build -o mcp-demo.exe ./cmd/mcp-demo
go build -o assistant.exe ./cmd/server

# 3) 运行（configs 默认连 127.0.0.1:3306）
./assistant.exe -config configs/config.yaml
```

无 MySQL 时自动降级 `memory` 仓储并打日志。

### API 示例

```bash
# 健康检查
curl http://localhost:8080/health

# 创建会话
curl -X POST http://localhost:8080/api/v1/session/create \
  -H "Content-Type: application/json" \
  -d "{\"userId\":\"u1\",\"title\":\"demo\"}"

# 对话
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d "{\"sessionId\":\"<id>\",\"userId\":\"u1\",\"message\":\"列出文件\"}"
```

## 目录结构

```
cmd/server              启动入口 (http | repl)
configs/                配置
internal/
  api/                  DTO / 接口
  application/          应用服务
  bootstrap/            依赖组装
  design/               通用设计模式 (chain/tree)
  domain/
    agent/              智能体域 (engine/intent/context/prompt/tools)
    desktop/            桌面能力域
    mcp/                MCP 扩展骨架
  infrastructure/       配置 / LLM / 仓储实现
  trigger/http          HTTP + SSE
  types/                枚举与异常
workspace/              文件工具沙箱目录
```

## 配置

见 `configs/config.yaml`。关键环境变量：

| 变量 | 说明 |
|------|------|
| `LLM_API_KEY` | 模型 Key；为空则 Mock |
| `LLM_API_BASE` | OpenAI 兼容地址 |
| `LLM_MODEL` | 模型名 |
| `LLM_USE_MOCK` | true/false |
| `SERVER_PORT` | 端口，默认 8080 |
| `DESKTOP_WORKSPACE` | 工作区路径 |

## MCP 配置

编辑 `configs/config.yaml`：

```yaml
mcp:
  enabled: true
  servers:
    - name: demo
      transport: stdio
      command: ""          # 空则自动找 mcp-demo
      timeout_sec: 30
    # - name: fs
    #   transport: stdio
    #   command: npx
    #   args: ["-y", "@modelcontextprotocol/server-filesystem", "./workspace"]
    # - name: remote
    #   transport: sse
    #   url: "https://host/mcp/sse"
```

## 注意

- 文件工具仅限 `workspace/` 沙箱；命令有 deny list
- 截图默认关闭；容器内无法操作宿主机桌面
- MySQL 连不上时降级内存模式（仅便于本地无库调试）

## License

MIT

