---
name: Workspace Overview
id: workspace-overview
description: 快速盘点工作区文件结构并给出摘要
triggers:
  - 工作区
  - 列出文件
  - 看看目录
  - workspace
  - list files
  - 概览
tools:
  - list_files
  - read_file
---

# 工作区概览 Skill

## 目标
用最少步骤让用户了解 `workspace/` 里有什么。

## 步骤
1. 调用 `list_files`（path 用 `.` 或空）列出顶层
2. 若有 README/说明类文件，用 `read_file` 读前几份关键文件
3. 用中文汇总：目录结构、关键文件、建议下一步

## 约束
- 不要删除或覆盖文件
- 不要执行 shell，除非用户明确要求
