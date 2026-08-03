---
name: MCP First
id: mcp-first
description: 优先使用已安装的 MCP 工具完成搜索/抓取/扩展能力
triggers:
  - mcp
  - 搜索网页
  - fetch
  - 用插件
  - 安装的工具
tools: []
---

# MCP First Skill

## 目标
演示「能力外置」：业务能力来自已安装 MCP，而不是内置硬编码。

## 步骤
1. 查看当前可用工具列表，识别 MCP 工具（名称可能带 `server__tool` 前缀）
2. 若任务需要搜索/HTTP，优先调用 fetch / search 类 MCP 工具
3. 若工具不存在，明确告诉用户：请先 `POST /api/v1/mcp/market/install` 或自定义安装
4. 汇总工具返回结果

## 约束
- 不要假装调用了不存在的工具
- 安装与热加载由系统完成；你只需在工具列表中选择
