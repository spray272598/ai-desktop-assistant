---
name: Safe Shell Helper
id: safe-shell
description: 在确认后执行只读或低风险命令，汇报 stdout
triggers:
  - 执行命令
  - run command
  - shell
  - 跑一下
  - 查看系统
tools:
  - run_command
  - list_files
---

# Safe Shell Skill

## 目标
帮助用户安全地执行命令并解释输出。

## 步骤
1. 先判断命令是否只读（如 `dir`/`ls`/`pwd`/`echo`）
2. 危险或写操作前说明风险，等待权限确认
3. 调用 `run_command` 执行
4. 解读输出，给出下一步建议

## 约束
- 禁止建议 `rm -rf /`、格式化磁盘、强制推送 main
- 优先工作区内操作
