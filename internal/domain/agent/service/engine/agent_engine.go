package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/repository"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
	contextsvc "github.com/ai-desktop/assistant/internal/domain/agent/service/context"
	"github.com/ai-desktop/assistant/internal/domain/agent/service"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/intent"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/prompt"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/security"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/task"
	"github.com/ai-desktop/assistant/internal/types/enums"
)

// AgentEngine 统一 ReAct 引擎
type AgentEngine struct {
	agent          *entity.AgentEntity
	intentService  intent.IIntentService
	promptService  *prompt.PromptService
	milestones     *prompt.MilestoneTracker
	chatContext    *contextsvc.ChatContextService
	llm            port.ILLMPort
	tools          *service.ToolRegistry
	sessionRepo    repository.ISessionRepository
	messageRepo    repository.IMessageRepository
	baseLoopConfig LoopConfig
	breakdown      *task.BreakdownService
	permGuard      *security.PermissionGuard
}

func NewAgentEngine(
	agent *entity.AgentEntity,
	intentService intent.IIntentService,
	promptService *prompt.PromptService,
	milestones *prompt.MilestoneTracker,
	chatContext *contextsvc.ChatContextService,
	llm port.ILLMPort,
	tools *service.ToolRegistry,
	sessionRepo repository.ISessionRepository,
	messageRepo repository.IMessageRepository,
	loopCfg LoopConfig,
) *AgentEngine {
	if loopCfg.MaxRounds <= 0 {
		loopCfg = DefaultLoopConfig()
	}
	return &AgentEngine{
		agent:          agent,
		intentService:  intentService,
		promptService:  promptService,
		milestones:     milestones,
		chatContext:    chatContext,
		llm:            llm,
		tools:          tools,
		sessionRepo:    sessionRepo,
		messageRepo:    messageRepo,
		baseLoopConfig: loopCfg,
	}
}

func (e *AgentEngine) SetBreakdown(b *task.BreakdownService) { e.breakdown = b }
func (e *AgentEngine) SetPermissionGuard(g *security.PermissionGuard) {
	e.permGuard = g
}
func (e *AgentEngine) PermissionGuard() *security.PermissionGuard { return e.permGuard }

// RunOptions 运行选项
type RunOptions struct {
	AutoApprove bool
}

// Run 同步运行，可选 eventCh 推送事件
func (e *AgentEngine) Run(ctx context.Context, session *entity.SessionEntity, userInput string, eventCh chan<- *AgentEvent) (*AgentResult, error) {
	return e.RunWithOptions(ctx, session, userInput, eventCh, RunOptions{})
}

func (e *AgentEngine) RunWithOptions(ctx context.Context, session *entity.SessionEntity, userInput string, eventCh chan<- *AgentEvent, opts RunOptions) (*AgentResult, error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}
	userInput = strings.TrimSpace(userInput)
	if userInput == "" {
		return nil, fmt.Errorf("empty user input")
	}

	publish := func(ev *AgentEvent) {
		if eventCh == nil || ev == nil {
			return
		}
		select {
		case eventCh <- ev:
		default:
		}
	}

	// 1. 意图识别
	intentResult, _ := e.intentService.Recognize(ctx, session.ID, session.UserID, userInput)
	if intentResult == nil {
		intentResult = valobj.NewIntentResult(string(enums.IntentChat), 0.5, nil)
	}
	publish(&AgentEvent{
		Type: EventIntent, Content: intentResult.Intent,
		Data: intentResult, Timestamp: time.Now().UnixMilli(),
	})

	loopCfg := LoopConfigFromIntent(intentResult.Intent, e.baseLoopConfig)
	if e.agent != nil && e.agent.MaxSteps > 0 {
		loopCfg.MaxRounds = e.agent.MaxSteps
	}

	// 2. 任务拆解
	var plan *valobj.TaskPlan
	if e.breakdown != nil {
		// 用户说「继续」复用已有计划
		if isContinue(userInput) {
			plan = e.breakdown.GetPlan(session.ID)
		} else {
			plan = e.breakdown.Breakdown(ctx, session.ID, userInput)
		}
		if plan != nil && len(plan.SubTasks) > 0 {
			publish(&AgentEvent{
				Type: EventPlan, Content: plan.Summary, Data: plan,
				Timestamp: time.Now().UnixMilli(),
			})
			e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneTaskStart, "计划: "+plan.Summary, 0))
		}
	}

	// 3. 里程碑
	e.milestones.DetectAndRecordWithUser(session.ID, session.UserID, "user", userInput, 0)
	e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneTaskStart, userInput, 0))
	if e.chatContext != nil {
		e.chatContext.SetTask(session.ID, userInput)
	}

	// 4. 历史裁剪
	historyMaps, _ := e.messageRepo.ListAsMaps(ctx, session.ID, 100)
	if e.chatContext != nil {
		historyMaps = e.chatContext.TrimHistory(historyMaps, loopCfg.MaxTokenBudget/2)
	}

	// 5. Prompt 上下文
	workDir := session.WorkingDir
	if workDir == "" {
		workDir = "./workspace"
	}
	var pctx *valobj.PromptContextVO
	if e.chatContext != nil {
		pctx = e.chatContext.BuildPromptContext(session.ID, session.UserID, workDir, historyMaps)
	} else {
		pctx = valobj.NewPromptContext(e.agent.Name, session.UserID, workDir)
	}
	if e.agent != nil {
		pctx.AgentName = e.agent.Name
	}
	pctx.IntentResult = intentResult
	pctx.Milestones = e.milestones.GetMilestones(session.ID)
	pctx.SetTools(e.toolInfos())
	pctx.UserInput = userInput
	if plan != nil {
		pctx.TaskDescription = plan.StringForPrompt()
	}

	systemPrompt := e.promptService.BuildSystemPrompt(pctx)
	if plan != nil {
		systemPrompt += "\n## 执行计划\n请按下列子任务顺序推进，每步完成后简要标记进度：\n" + plan.StringForPrompt()
	}
	userPrompt := e.promptService.BuildUserPrompt(userInput, pctx)

	// 6. 持久化用户消息
	userMsg := entity.NewUserMessage(session.ID, userInput)
	_ = e.messageRepo.Save(ctx, userMsg)
	session.AddMessage(0)

	// 7. 组装 LLM 消息
	messages := make([]port.ChatMessage, 0, len(historyMaps)+8)
	for _, m := range historyMaps {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if role == "" || content == "" || role == "system" {
			continue
		}
		messages = append(messages, port.ChatMessage{Role: role, Content: content})
	}
	messages = append(messages, port.ChatMessage{Role: "user", Content: userPrompt})

	publish(NewEvent(EventThought, 0, "开始处理："+truncate(userInput, 80)))

	// 8. ReAct 循环
	totalTokens := 0
	totalToolCalls := 0
	var finalAnswer string
	var lastToolSig string
	sameSigCount := 0
	var pendingPerm *PendingPermissionInfo

	for step := 1; step <= loopCfg.MaxRounds; step++ {
		select {
		case <-ctx.Done():
			publish(&AgentEvent{Type: EventError, Step: step, Content: "context cancelled", Completed: true, Timestamp: time.Now().UnixMilli()})
			return &AgentResult{SessionID: session.ID, Response: "请求已取消", Intent: intentResult.Intent, Steps: step - 1, TokenUsed: totalTokens, ToolCalls: totalToolCalls}, ctx.Err()
		default:
		}

		publish(NewEvent(EventThought, step, fmt.Sprintf("思考步骤 %d...", step)))

		var resp *port.ChatResponse
		var err error
		for attempt := 0; attempt <= loopCfg.MaxAiRetries; attempt++ {
			resp, err = e.llm.Generate(ctx, &port.ChatRequest{
				SystemPrompt: systemPrompt,
				Messages:     messages,
				Temperature:  0.3,
			})
			if err == nil {
				break
			}
			if attempt < loopCfg.MaxAiRetries {
				time.Sleep(time.Duration(loopCfg.RetryDelayBaseMs*(1<<attempt)) * time.Millisecond)
			}
		}
		if err != nil {
			publish(&AgentEvent{Type: EventError, Step: step, Content: err.Error(), Completed: true, Timestamp: time.Now().UnixMilli()})
			e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneError, err.Error(), step))
			finalAnswer = "LLM 调用失败: " + err.Error()
			break
		}

		totalTokens += resp.TotalTokens
		if totalTokens == 0 {
			totalTokens += len(resp.Content) / 2
		}

		toolCalls := resp.ToolCalls
		if len(toolCalls) == 0 {
			toolCalls = parseToolCalls(resp.Content)
		}

		if len(toolCalls) == 0 {
			finalAnswer = strings.TrimSpace(resp.Content)
			if finalAnswer == "" {
				finalAnswer = "已完成处理。"
			}
			asst := entity.NewAssistantMessage(session.ID, finalAnswer, step)
			_ = e.messageRepo.Save(ctx, asst)
			session.AddMessage(resp.TotalTokens)
			e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneTaskComplete, truncate(finalAnswer, 120), step))
			publish(&AgentEvent{Type: EventAnswer, Step: step, Content: finalAnswer, Completed: true, Timestamp: time.Now().UnixMilli()})
			break
		}

		sig := toolSignature(toolCalls)
		if sig == lastToolSig {
			sameSigCount++
			if sameSigCount >= loopCfg.DiminishingReturnsThreshold {
				finalAnswer = "检测到重复工具调用，已停止循环。\n\n最后模型输出：\n" + resp.Content
				publish(&AgentEvent{Type: EventError, Step: step, Content: "diminishing returns", Completed: true, Timestamp: time.Now().UnixMilli()})
				break
			}
		} else {
			sameSigCount = 0
			lastToolSig = sig
		}

		asstContent := resp.Content
		if asstContent == "" {
			asstContent = formatToolCallsJSON(toolCalls)
		}
		messages = append(messages, port.ChatMessage{Role: "assistant", Content: asstContent})
		_ = e.messageRepo.Save(ctx, entity.NewAssistantMessage(session.ID, asstContent, step))

		if len(toolCalls) > loopCfg.MaxToolCallsPerRound {
			toolCalls = toolCalls[:loopCfg.MaxToolCallsPerRound]
		}

		needBreakForPerm := false
		for _, tc := range toolCalls {
			if totalToolCalls >= loopCfg.MaxTotalToolCalls {
				break
			}
			totalToolCalls++

			// ---- 权限门 ----
			if e.permGuard != nil && !opts.AutoApprove {
				dec := e.permGuard.Check(session.ID, tc.Name, tc.Args)
				switch dec.Action {
				case security.ActionDeny:
					resultText := fmt.Sprintf("⛔ 权限拒绝: %s (%s)", dec.Reason, dec.RuleID)
					publish(&AgentEvent{
						Type: EventPermission, SubType: "deny", Step: step,
						Content: resultText, Data: dec, Timestamp: time.Now().UnixMilli(),
					})
					messages = append(messages, port.ChatMessage{
						Role: "user", Content: fmt.Sprintf("[工具 %s 执行结果]\n%s", tc.Name, resultText),
					})
					continue
				case security.ActionConfirm:
					p := e.permGuard.CreatePending(session.ID, tc.Name, tc.Args, dec)
					pendingPerm = &PendingPermissionInfo{
						ID: p.ID, Tool: p.Tool, Args: p.Args, Reason: p.Reason, RuleID: p.RuleID,
					}
					msg := fmt.Sprintf("⚠️ 操作需要确认\n工具: %s\n原因: %s\n确认ID: %s\n\n请在界面点击「批准」或调用 POST /api/v1/permission/approve 后重试（或发送「继续」）。",
						tc.Name, dec.Reason, p.ID)
					publish(&AgentEvent{
						Type: EventPermission, SubType: "confirm", Step: step,
						Content: msg, Data: pendingPerm, Completed: true, Timestamp: time.Now().UnixMilli(),
					})
					finalAnswer = msg
					needBreakForPerm = true
					break
				}
			}

			publish(&AgentEvent{
				Type: EventToolCall, SubType: tc.Name, Step: step,
				Content: fmt.Sprintf("调用工具: %s", tc.Name),
				Data:    tc.Args, Timestamp: time.Now().UnixMilli(),
			})
			e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneToolCalled, tc.Name, step))

			resultText := e.executeTool(ctx, tc)
			if e.chatContext != nil {
				e.chatContext.PushToolResult(session.ID, tc.Name, resultText)
			}
			e.milestones.DetectAndRecord(session.ID, "tool", resultText, step)

			publish(&AgentEvent{
				Type: EventToolResult, SubType: tc.Name, Step: step,
				Content: truncate(resultText, 800), Timestamp: time.Now().UnixMilli(),
			})

			toolMsg := entity.NewToolMessage(session.ID, tc.Name, tc.ID, resultText, step)
			_ = e.messageRepo.Save(ctx, toolMsg)
			messages = append(messages, port.ChatMessage{
				Role: "user",
				Content: fmt.Sprintf("[工具 %s 执行结果]\n%s", tc.Name, resultText),
				Name: tc.Name,
			})
		}
		if needBreakForPerm {
			break
		}

		messages = append(messages, port.ChatMessage{
			Role:    "user",
			Content: e.promptService.BuildStepPrompt(step, userInput),
		})

		if step >= loopCfg.MaxRounds {
			finalAnswer = "已达到最大执行步数，请根据已有结果继续或重新提问。"
			publish(&AgentEvent{Type: EventError, Step: step, Content: finalAnswer, Completed: true, Timestamp: time.Now().UnixMilli()})
		}
	}

	if finalAnswer == "" {
		finalAnswer = "未能生成最终答案。"
	}

	session.Touch()
	_ = e.sessionRepo.Save(ctx, session)

	publish(&AgentEvent{Type: EventComplete, Content: finalAnswer, Completed: true, Timestamp: time.Now().UnixMilli()})

	result := &AgentResult{
		SessionID: session.ID,
		Response:  finalAnswer,
		Intent:    intentResult.Intent,
		Steps:     totalToolCalls + 1,
		TokenUsed: totalTokens,
		ToolCalls: totalToolCalls,
		TaskPlan:  plan,
	}
	if pendingPerm != nil {
		result.PendingPermission = pendingPerm
		result.NeedPermission = true
	}
	return result, nil
}

func (e *AgentEngine) executeTool(ctx context.Context, tc port.ToolCall) string {
	tool := e.tools.GetTool(tc.Name)
	if tool == nil {
		return fmt.Sprintf("工具 %s 不存在", tc.Name)
	}
	result, err := tool.Execute(ctx, tc.Args)
	if err != nil {
		return fmt.Sprintf("工具 %s 执行失败: %v", tc.Name, err)
	}
	return result
}

func (e *AgentEngine) toolInfos() []*valobj.ToolInfo {
	list := e.tools.ListTools()
	out := make([]*valobj.ToolInfo, 0, len(list))
	for _, t := range list {
		out = append(out, &valobj.ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
		})
	}
	return out
}

func isContinue(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "继续" || s == "继续执行" || s == "continue" || s == "go on" || s == "ok" || s == "批准后继续"
}

func parseToolCalls(response string) []port.ToolCall {
	response = strings.TrimSpace(response)
	if response == "" {
		return nil
	}
	var single struct {
		Name string                 `json:"name"`
		Args map[string]interface{} `json:"args"`
	}
	if err := json.Unmarshal([]byte(response), &single); err == nil && single.Name != "" {
		return []port.ToolCall{{Name: single.Name, Args: ensureArgs(single.Args)}}
	}
	var multi []struct {
		Name string                 `json:"name"`
		Args map[string]interface{} `json:"args"`
	}
	if err := json.Unmarshal([]byte(response), &multi); err == nil && len(multi) > 0 {
		var calls []port.ToolCall
		for _, m := range multi {
			if m.Name != "" {
				calls = append(calls, port.ToolCall{Name: m.Name, Args: ensureArgs(m.Args)})
			}
		}
		if len(calls) > 0 {
			return calls
		}
	}
	if idx := strings.Index(response, "```json"); idx >= 0 {
		rest := response[idx+7:]
		if end := strings.Index(rest, "```"); end >= 0 {
			return parseToolCalls(strings.TrimSpace(rest[:end]))
		}
	}
	if idx := strings.Index(response, "```"); idx >= 0 {
		rest := response[idx+3:]
		if end := strings.Index(rest, "```"); end >= 0 {
			block := strings.TrimSpace(rest[:end])
			if strings.HasPrefix(block, "json") {
				block = strings.TrimSpace(block[4:])
			}
			return parseToolCalls(block)
		}
	}
	if start := strings.Index(response, "{"); start >= 0 {
		if end := strings.LastIndex(response, "}"); end > start {
			return parseToolCalls(response[start : end+1])
		}
	}
	return nil
}

func ensureArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return map[string]interface{}{}
	}
	return args
}

func toolSignature(calls []port.ToolCall) string {
	h := sha256.New()
	for _, c := range calls {
		h.Write([]byte(c.Name))
		b, _ := json.Marshal(c.Args)
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func formatToolCallsJSON(calls []port.ToolCall) string {
	b, _ := json.Marshal(calls)
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
