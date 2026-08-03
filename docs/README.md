# 文档索引（学习本项目 / 准备秋招）

| 文档 | 内容 |
|------|------|
| [architecture.md](./architecture.md) | 架构图、主链路、设计决策、扩展点 |
| [getting-started.md](./getting-started.md) | 本地/Docker 启动、API、排错 |
| [features.md](./features.md) | P0–P3 功能与配置对照 |
| [interview-guide.md](./interview-guide.md) | 1 分钟介绍、深挖题、简历写法 |
| [dev-ops/mysql/sql/01_schema.sql](./dev-ops/mysql/sql/01_schema.sql) | 数据库 schema |

## 建议学习顺序（3 天）

**Day 1**：跑通 Mock → 读 bootstrap + engine 主循环  
**Day 2**：意图/上下文/权限/MCP → 对照 interview 题自己讲一遍  
**Day 3**：加一个自定义工具或 MCP，提交小 PR 式改动  

## 核心代码地图

```
internal/bootstrap/app.go                 组装（MCP 恢复 + Skills）
internal/domain/agent/service/engine/     ReAct 心脏（恢复执行/计划推进/预算）
internal/domain/agent/service/intent/     意图
internal/domain/agent/service/security/   权限门 + awaiting 恢复
internal/domain/agent/service/orchestrator/ 多Agent + 模型路由
internal/domain/mcp/service/              MCP 安装/健康/热同步
internal/domain/skill/service/            SKILL.md 加载匹配
internal/infrastructure/mcp/              MCP 协议客户端
internal/domain/desktop/service/          文件/浏览器/沙箱/截图
skills/*/SKILL.md                         示例工作流
scripts/eval_agent_smoke.md               冒烟评测
web/src/App.tsx                           React 控制台
```
