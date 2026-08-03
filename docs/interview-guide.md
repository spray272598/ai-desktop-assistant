# 秋招面试指南（基于本项目）

把项目当成「可深挖的业务系统」，而不是 CRUD。建议准备 **1 分钟介绍 + 3 个深挖故事**。

## 1. 一分钟项目介绍（背熟）

> 我做了一个本地 **Agent 运行时**（不是堆业务功能的 App）。DDD 分层的 Go 服务：  
> 意图 → Skill 匹配 → 多 Agent 路由规划 → ReAct → 权限门 → 动态工具。  
> **能力外置**：搜索/文件等用社区 MCP 安装热加载；流程规范用 SKILL.md 注入。  
> 重点打磨安装生命周期、批准后恢复执行、计划步进、工具结果预算与错误分类。  
> 工程上有 MySQL 持久化、SSE 可观测、React 控制台。

## 2. 高频深挖题

### Q1：为什么用 DDD？有什么收益？

**答**：桌面助手业务规则多（权限、意图、计划、工具协议），和框架生命周期耦合会变乱。  
Domain 放引擎/权限/意图；Infrastructure 换 MySQL/Redis/LLM 实现不改业务。  
**收益**：可测试、边界清晰、换存储/模型成本低。

### Q2：ReAct 循环如何防止死循环？

**答**：

1. `maxRounds` / 全局工具调用上限  
2. 工具签名哈希：连续相同调用达阈值终止（diminishing returns）  
3. 超时 context  
4. 权限 CONFIRM 中断等待用户  

### Q3：工具结果为什么用 tool 角色？

**答**：ChatML 区分 user / assistant / tool。tool 是环境观测。  
若用 user，模型会当作新指令，导致重复调用或错误推理。我们还补了 `tool_call_id` 配对。

### Q4：权限系统设计？

**答**：PermissionGuard 三态：ALLOW / DENY / CONFIRM。  
写文件、shell、敏感命令走 CONFIRM；`rm -rf /` 等 DENY。  
支持 once / session 批准；连续拒绝触发断路器。  
**为什么引擎内拦截**：横切、可审计、工具无感。

### Q5：上下文如何控制 token？

**答**：HybridReducer = 优先级保留 ∪ 滑动窗口；保证最近消息；超预算再收紧。  
动态 Prompt 分区：实时环境 vs 历史上下文，并提示「以当前意图为主」。

### Q6：MCP 是什么？你们怎么接？

**答**：Model Context Protocol，标准化工具发现与调用（JSON-RPC）。  
实现 stdio + SSE 客户端；Manager 热增删；工具同步进 ToolRegistry。  
插件市场预置 filesystem/fetch 等，一键安装写入 DB 并热加载。

### Q7：MySQL 挂了怎么办？

**答**：bootstrap 连接失败降级 memory 仓储，保证本地可演示；生产应强依赖健康检查。

### Q8：如果让你做多租户/云端版本？

**答**：

- 会话表加 tenant_id + RLS  
- 工具执行改为 Host Agent（用户机器侧车）  
- 网关鉴权 JWT  
- 对象存储存截图/导出  
- 可观测：trace_id 贯穿 ReAct 每步  

### Q9：最大技术难点？

**推荐故事**：  
「工具协议与安全」——早期 tool 结果误用 user 角色导致乱调工具；权限门与确认流打断 ReAct 后的恢复（批准后「继续」）。

### Q10：性能优化点？

- 意图缓存（session+hash）  
- Redis 会话最近消息缓存  
- 限流防刷  
- 截图/HTML 截断防上下文爆炸  
- Mock/真实 LLM 可切换便于压测编排层  

## 3. 建议手绘白板图

面试时画：

```
User → API → Intent → Router/Planner → ReAct Engine
                              ↓
                     Permission → Tools/MCP
                              ↓
                          MySQL/Redis
```

## 4. 代码阅读路径（30 分钟）

1. `internal/bootstrap/app.go` — 依赖如何组装  
2. `internal/domain/agent/service/engine/agent_engine.go` — 主循环  
3. `internal/domain/agent/service/security/permission_guard.go` — 安全  
4. `internal/infrastructure/mcp/*` — 协议客户端  
5. `internal/trigger/http/server.go` — API 面  

## 5. 简历写法示例

- 独立设计并实现基于 ReAct 的桌面 Agent 运行时，支持意图识别、任务拆解与多工具编排  
- 实现 MCP 热加载与插件市场，扩展工具无需重启  
- 设计权限确认门与危险命令正则拦截，降低本地命令执行风险  
- 完成 MySQL 持久化、Redis 限流、SSE 流式事件与 React 控制台  

## 6. 诚实边界（加分）

面试官问「生产级吗」：  
> 核心链路可用；Wails 为可演进骨架；浏览器依赖本机 Chrome；沙箱在无 Docker 时降级本地。  
> 我清楚差距：鉴权、多租户、可观测性、Host Agent 分离。
