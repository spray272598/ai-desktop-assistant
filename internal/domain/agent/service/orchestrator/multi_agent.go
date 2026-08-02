package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/task"
)

// Role Agent 角色
type Role string

const (
	RoleRouter   Role = "router"
	RolePlanner  Role = "planner"
	RoleExecutor Role = "executor"
	RoleReviewer Role = "reviewer"
)

// AgentStep 编排步骤记录
type AgentStep struct {
	Role    Role   `json:"role"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
}

// OrchestrationResult 多 Agent 协作结果
type OrchestrationResult struct {
	Route      string            `json:"route"`
	Plan       *valobj.TaskPlan  `json:"plan,omitempty"`
	Steps      []AgentStep       `json:"steps"`
	FinalHint  string            `json:"finalHint"`
	ModelUsed  string            `json:"modelUsed,omitempty"`
}

// MultiAgentOrchestrator Router → Planner → Executor 提示链
// 真正执行仍由主 ReAct Engine 完成；本组件产出路线与计划。
type MultiAgentOrchestrator struct {
	llm       port.ILLMPort
	breakdown *task.BreakdownService
	models    *ModelRouter
}

func NewMultiAgentOrchestrator(llm port.ILLMPort, breakdown *task.BreakdownService, models *ModelRouter) *MultiAgentOrchestrator {
	return &MultiAgentOrchestrator{llm: llm, breakdown: breakdown, models: models}
}

// Orchestrate 对用户输入做路由与规划
func (o *MultiAgentOrchestrator) Orchestrate(ctx context.Context, sessionID, userInput string) *OrchestrationResult {
	res := &OrchestrationResult{Steps: make([]AgentStep, 0, 4)}

	// Router
	route := o.route(userInput)
	res.Route = route
	res.Steps = append(res.Steps, AgentStep{Role: RoleRouter, Input: userInput, Output: route, Success: true})
	if o.models != nil {
		res.ModelUsed = o.models.Select(route)
	}

	// Planner（复杂任务）
	if route == "complex" || route == "plan" {
		if o.breakdown != nil {
			plan := o.breakdown.Breakdown(ctx, sessionID, userInput)
			res.Plan = plan
			out := "no plan"
			if plan != nil {
				out = plan.Summary + fmt.Sprintf(" (%d subtasks)", len(plan.SubTasks))
			}
			res.Steps = append(res.Steps, AgentStep{Role: RolePlanner, Input: userInput, Output: out, Success: plan != nil})
		}
	}

	// Executor hint
	hint := buildExecutorHint(route, res.Plan)
	res.FinalHint = hint
	res.Steps = append(res.Steps, AgentStep{Role: RoleExecutor, Input: route, Output: hint, Success: true})
	return res
}

func (o *MultiAgentOrchestrator) route(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case containsAny(lower, "部署", "迁移", "重构", "搭建", "全流程", "然后", "接着", "step by step"):
		return "complex"
	case containsAny(lower, "打开", "网页", "http", "浏览器", "url"):
		return "browser"
	case containsAny(lower, "代码", "python", "javascript", "运行脚本", "run_code"):
		return "code"
	case containsAny(lower, "文件", "目录", "读取", "写入", "list"):
		return "file"
	case containsAny(lower, "命令", "shell", "执行"):
		return "command"
	default:
		// 可选 LLM 路由
		if o.llm != nil && len(msg) > 40 {
			if r := o.llmRoute(context.Background(), msg); r != "" {
				return r
			}
		}
		return "chat"
	}
}

func (o *MultiAgentOrchestrator) llmRoute(ctx context.Context, msg string) string {
	resp, err := o.llm.Generate(ctx, &port.ChatRequest{
		SystemPrompt: "你是路由器。只输出一个词: chat|file|command|browser|code|complex",
		Messages:     []port.ChatMessage{{Role: "user", Content: msg}},
		Temperature:  0,
		MaxTokens:    16,
	})
	if err != nil || resp == nil {
		return ""
	}
	r := strings.ToLower(strings.TrimSpace(resp.Content))
	for _, k := range []string{"chat", "file", "command", "browser", "code", "complex"} {
		if strings.Contains(r, k) {
			return k
		}
	}
	return ""
}

func buildExecutorHint(route string, plan *valobj.TaskPlan) string {
	var b strings.Builder
	b.WriteString("【Router】路线=" + route + "\n")
	switch route {
	case "browser":
		b.WriteString("优先工具: open_url, browser_html, browser_screenshot\n")
	case "code":
		b.WriteString("优先工具: run_code (language=python|javascript)\n")
	case "file":
		b.WriteString("优先工具: list_files, read_file, write_file\n")
	case "command":
		b.WriteString("优先工具: run_command（注意权限确认）\n")
	case "complex":
		b.WriteString("按计划逐步执行，每步验证结果\n")
	}
	if plan != nil {
		b.WriteString(plan.StringForPrompt())
	}
	return b.String()
}

func containsAny(s string, kws ...string) bool {
	for _, k := range kws {
		if strings.Contains(s, strings.ToLower(k)) || strings.Contains(s, k) {
			return true
		}
	}
	return false
}
