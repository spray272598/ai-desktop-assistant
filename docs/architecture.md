# 架构设计（Architecture）

> 面向秋招：能讲清分层、主链路、扩展点与取舍。

## 1. 系统定位

**AI Desktop Assistant** 是本地优先的桌面智能助手后端 + Web 控制台：

- 理解用户意图（规则 + LLM）
- 多步任务规划与 ReAct 执行
- 安全调用本地工具 / MCP / 浏览器 / 代码沙箱
- MySQL 持久化会话与记忆
- Redis 可选缓存与限流

## 2. 总体架构

```
┌──────────────────────────────────────────────────────────┐
│  Web Console (React/Vite)  |  Wails/Browser Shell       │
│  Mobile Device (WebSocket Agent)                         │
└───────────────┬──────────────────────────┬───────────────┘
                │ HTTP/SSE/WS              │
┌───────────────▼──────────────────────────▼───────────────┐
│  trigger/http + trigger/ws                               │
│  限流 · CORS · 静态资源 · 设备中枢                        │
└───────────────┬──────────────────────────────────────────┘
                │
┌───────────────▼──────────────────────────────────────────┐
│  application（用例）                                      │
│  Chat / Session / Export / RateLimit / MCP 编排入口       │
└───────────────┬──────────────────────────────────────────┘
                │
┌───────────────▼──────────────────────────────────────────┐
│  domain/agent                                             │
│  Engine(ReAct) · Intent · Context · Prompt · Security    │
│  TaskBreakdown · Orchestrator(Router/Planner/Executor)   │
│  Tools Registry · Multi-Agent hint · Model A/B Router    │
├──────────────────────────────────────────────────────────┤
│  domain/desktop  文件/命令/截图/浏览器/沙箱               │
│  domain/mcp      MCP 客户端/热加载/插件市场               │
└───────────────┬──────────────────────────────────────────┘
                │ ports / repositories
┌───────────────▼──────────────────────────────────────────┐
│  infrastructure                                           │
│  LLM Gateway · MySQL · Redis · MCP stdio/SSE · Config    │
└──────────────────────────────────────────────────────────┘
```

## 3. DDD 分层对应（Go 目录）

| 层 | 路径 | 职责 |
|----|------|------|
| 用户接口 | `internal/trigger` | HTTP/SSE/WS、静态 Web |
| 应用层 | `internal/application` | 用例编排，不写业务细则 |
| 领域层 | `internal/domain/*` | 核心业务：引擎、意图、权限、工具 |
| 基础设施 | `internal/infrastructure` | 技术实现：DB/Redis/LLM/MCP |
| 启动组装 | `internal/bootstrap` | 依赖注入（手写 Composition Root） |

**面试话术**：领域不依赖框架细节；仓储接口在 domain/adapter，实现在 infrastructure。

## 4. 一次对话的主链路

1. **限流** Redis / 内存滑动窗口  
2. **会话** 加载/创建 Session（MySQL/Memory）  
3. **意图识别** Rule →（低置信）LLM，结果缓存；「继续」走轻量意图  
4. **Skill 匹配** 触发词 / 显式 `/skill-id` → 注入执行指南（可选工具子集）  
5. **多 Agent 编排** Router → Planner 拆任务 → Executor 提示  
6. **动态 Prompt** 环境 + **当前 ToolRegistry（含热装 MCP）** + 里程碑 + 计划 + Skill  
7. **权限恢复** 若已批准 awaiting：先执行该工具 → `plan_update` → 再进循环  
8. **ReAct 循环** Thought → ToolCall → Permission → Observation（结果截断）  
9. **计划推进** 按 expectedTools / 顺序 MarkDone，SSE `plan_update`  
10. **持久化** 消息、里程碑、token 估算  
11. **事件** SSE：intent / skill / route / plan / plan_update / resume / tool / permission / error(class) / answer  

### 4.1 扩展生态（与业务工具解耦）

```
社区 MCP（搜索/FS/…） ──install──► MCP Manager ──sync──► ToolRegistry
本地 SKILL.md        ──install──► SkillService ──match──► System Prompt
```

**面试要点**：能力实现外置；运行时负责生命周期、权限、规划与可观测。

## 5. 关键设计决策

### 5.1 为何 ReAct 而不是单次 Function Call？

桌面任务往往多步、依赖上一步观察结果（列目录 → 读文件 → 改配置）。ReAct 循环 + 死循环检测更贴合。

### 5.2 工具结果为何必须是 `role=tool`？

ChatML 语义：tool 是环境反馈，不是用户发言。用 `user` 会诱导模型重复调用或误判指令来源。

### 5.3 权限门放在引擎内而非工具内？

横切关注点：统一裁决 DENY/CONFIRM/ALLOW，便于审计与会话级批准；工具只负责执行。

### 5.4 MCP 为何热加载？

桌面助手扩展性在「工具生态」。热加载避免重启，配合 DB 持久化实现「装上就用」。

### 5.5 沙箱为何 Docker 优先、本地降级？

隔离性 vs 可用性：开发机未必有 Docker；生产建议强制 Docker + `--network none`。

## 6. 数据模型（核心表）

- `chat_session` / `chat_message`：对话
- `chat_milestone`：关键事件
- `core_memory`：长期记忆
- `mcp_server_config`：MCP 服务配置

## 7. 扩展点清单（面试加分）

| 扩展点 | 怎么加 |
|--------|--------|
| 新本地工具 | 实现 `ITool`，在 bootstrap 注册 |
| 新 MCP | 市场安装或 POST `/api/v1/mcp/servers` |
| 新意图 | `rule_classifier` 规则或 LLM 分类标签 |
| 新模型档 | `config.yaml` → `llm.models` + ModelRouter |
| 新触发方式 | trigger 下加 gRPC/Wails binding |

## 8. 非目标（当前刻意不做或弱化）

- 完整云端多租户计费
- 强保证的分布式 Agent 集群
- 完美 GUI 自动化（桌面点击坐标级）—— 浏览器与文件/命令是主路径

---

下一篇：`docs/getting-started.md` 本地跑通；`docs/interview-guide.md` 题库。
