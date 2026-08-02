# AI Desktop Assistant

基于 Go 的桌面 AI 助手，采用 DDD 分层 + 运行时智能体（ReAct）。

## 能力

- **Agent Workflow**：统一 ReAct 循环（Thought → Action → Observation），死循环检测、步数/工具预算
- **意图识别**：规则优先 + LLM 降级，会话上下文追踪与缓存
- **上下文管理**：ContextProvider 拼装 + HybridReducer（Priority ∪ SlidingWindow）
- **动态 Prompt**：环境 / 工具 / 里程碑 / 任务分区注入
- **桌面工具**：文件（工作区 jail）、命令（deny list）、截图（Windows/macOS/Linux）
- **HTTP + SSE**：`/api/v1/chat`、`/api/v1/chat/stream`
- **本地 Docker 部署**

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

### Docker 本地部署

```bash
# Mock
docker compose up -d --build

# 真实模型
# Windows PowerShell:
$env:LLM_API_KEY="sk-xxx"; $env:LLM_USE_MOCK="false"; docker compose up -d --build
```

服务地址：`http://localhost:8080`

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

## 注意

- 容器内**无法**真正操作宿主机桌面截图/任意路径；文件操作映射到挂载的 `workspace/`
- 命令执行带危险命令拦截，但仍需注意部署环境权限
- 默认内存仓储，重启丢会话；后续可接 MySQL

## License

MIT
