package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/repository"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
	contextsvc "github.com/ai-desktop/assistant/internal/domain/agent/service/context"
	"github.com/ai-desktop/assistant/internal/domain/agent/service"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/intent"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/orchestrator"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/prompt"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/security"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/task"
	skillmodel "github.com/ai-desktop/assistant/internal/domain/skill/model"
	skillsvc "github.com/ai-desktop/assistant/internal/domain/skill/service"
	"github.com/ai-desktop/assistant/internal/types/common"
	"github.com/ai-desktop/assistant/internal/types/enums"
)

// 工具结果写入上下文的字符预算（防 context 爆炸）
const maxToolResultChars = 4000

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
	orchestrator   *orchestrator.MultiAgentOrchestrator
	skills         *skillsvc.SkillService
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
func (e *AgentEngine) SetOrchestrator(o *orchestrator.MultiAgentOrchestrator) {
	e.orchestrator = o
}
func (e *AgentEngine) SetSkillService(s *skillsvc.SkillService) { e.skills = s }
func (e *AgentEngine) PermissionGuard() *security.PermissionGuard { return e.permGuard }
func (e *AgentEngine) SkillService() *skillsvc.SkillService       { return e.skills }

// RunOptions 运行选项
type RunOptions struct {
	AutoApprove bool
	// ResumeAfterPermission 批准后直接恢复执行（API 自动 continue）
	ResumeAfterPermission bool
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

	continuing := isContinue(userInput) || opts.ResumeAfterPermission

	// 1. 意图识别（「继续」时轻量处理）
	var intentResult *valobj.IntentResult
	if continuing {
		intentResult = valobj.NewIntentResult(string(enums.IntentTaskPlan), 0.9, nil)
		intentResult.Source = "continue"
	} else {
		intentResult, _ = e.intentService.Recognize(ctx, session.ID, session.UserID, userInput)
		if intentResult == nil {
			intentResult = valobj.NewIntentResult(string(enums.IntentChat), 0.5, nil)
		}
	}
	publish(&AgentEvent{
		Type: EventIntent, Content: intentResult.Intent,
		Data: intentResult, Timestamp: time.Now().UnixMilli(),
	})

	loopCfg := LoopConfigFromIntent(intentResult.Intent, e.baseLoopConfig)
	if e.agent != nil && e.agent.MaxSteps > 0 {
		loopCfg.MaxRounds = e.agent.MaxSteps
	}

	// 2. Skill 匹配
	var activeSkill *skillmodel.Skill
	if e.skills != nil && !continuing {
		activeSkill = e.skills.Match(userInput)
		if activeSkill != nil {
			publish(&AgentEvent{
				Type: EventSkill, SubType: activeSkill.ID, Content: "激活 Skill: " + activeSkill.Name,
				Data: activeSkill, Timestamp: time.Now().UnixMilli(),
			})
		}
	}

	// 3. 多 Agent 编排 + 任务拆解
	var plan *valobj.TaskPlan
	var orchHint string
	if !continuing {
		if e.orchestrator != nil {
			orch := e.orchestrator.Orchestrate(ctx, session.ID, userInput)
			if orch != nil {
				publish(&AgentEvent{
					Type: EventRoute, Content: orch.Route, Data: orch,
					Timestamp: time.Now().UnixMilli(),
				})
				plan = orch.Plan
				orchHint = orch.FinalHint
			}
		}
		if plan == nil && e.breakdown != nil {
			plan = e.breakdown.Breakdown(ctx, session.ID, userInput)
		}
	} else if e.breakdown != nil {
		plan = e.breakdown.GetPlan(session.ID)
	}
	if plan != nil && len(plan.SubTasks) > 0 {
		// 启动第一个 pending
		if plan.PendingCount() > 0 && plan.CurrentIndex == 0 {
			_ = plan.StartNext()
		}
		publish(&AgentEvent{
			Type: EventPlan, Content: plan.Summary, Data: plan,
			Timestamp: time.Now().UnixMilli(),
		})
		e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneTaskStart, "计划: "+plan.Summary, 0))
	}

	// 4. 里程碑 / 任务上下文
	taskText := userInput
	if continuing {
		taskText = "继续执行已批准操作"
	}
	e.milestones.DetectAndRecordWithUser(session.ID, session.UserID, "user", taskText, 0)
	e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneTaskStart, taskText, 0))
	if e.chatContext != nil {
		e.chatContext.SetTask(session.ID, taskText)
	}

	// 5. 历史裁剪
	historyMaps, _ := e.messageRepo.ListAsMaps(ctx, session.ID, 100)
	if e.chatContext != nil {
		historyMaps = e.chatContext.TrimHistory(historyMaps, loopCfg.MaxTokenBudget/2)
	}

	// 6. Prompt 上下文（动态工具列表：MCP 热装后立刻可见）
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
	toolInfos := e.toolInfos()
	if activeSkill != nil && e.skills != nil && len(activeSkill.Tools) > 0 {
		toolInfos = e.filterToolInfos(activeSkill, toolInfos)
	}
	pctx.SetTools(toolInfos)
	pctx.UserInput = userInput
	if plan != nil {
		pctx.TaskDescription = plan.StringForPrompt()
	}

	systemPrompt := e.promptService.BuildSystemPrompt(pctx)
	if activeSkill != nil && e.skills != nil {
		systemPrompt += "\n" + e.skills.PromptSection(activeSkill)
	}
	if orchHint != "" {
		systemPrompt += "\n## 多 Agent 编排提示\n" + orchHint + "\n"
	}
	if plan != nil {
		systemPrompt += "\n## 执行计划\n请按下列子任务顺序推进，每步完成后简要标记进度：\n" + plan.StringForPrompt()
	}
	userPrompt := e.promptService.BuildUserPrompt(userInput, pctx)
	if continuing {
		userPrompt = e.promptService.BuildUserPrompt("请继续执行：若有已批准的待执行工具请立刻调用；否则根据历史工具结果完成任务。", pctx)
	}

	// 7. 持久化用户消息
	userMsg := entity.NewUserMessage(session.ID, userInput)
	userTokens := common.EstimateTokens(userInput)
	userMsg.TokenCount = userTokens
	_ = e.messageRepo.Save(ctx, userMsg)
	session.AddMessage(userTokens)

	// 8. 组装 LLM 消息
	messages := make([]port.ChatMessage, 0, len(historyMaps)+8)
	for _, m := range historyMaps {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if role == "" || content == "" || role == "system" {
			continue
		}
		// 历史 tool 结果再截断一次，防止预算膨胀
		if role == "tool" {
			content = budgetToolResult(content)
		}
		cm := port.ChatMessage{Role: role, Content: content}
		if name, ok := m["toolName"].(string); ok {
			cm.Name = name
		}
		if id, ok := m["toolCallId"].(string); ok {
			cm.ToolCallID = id
		}
		messages = append(messages, cm)
	}
	messages = append(messages, port.ChatMessage{Role: "user", Content: userPrompt})

	publish(NewEvent(EventThought, 0, "开始处理："+truncate(userInput, 80)))

	totalTokens := 0
	totalToolCalls := 0
	var finalAnswer string
	var lastToolSig string
	sameSigCount := 0
	var pendingPerm *PendingPermissionInfo
	var errClass ErrorClass
	skillID := ""
	if activeSkill != nil {
		skillID = activeSkill.ID
	}

	// 9. 权限恢复：已批准则先执行 awaiting 工具，再进入 ReAct
	if e.permGuard != nil && (continuing || opts.ResumeAfterPermission) {
		if resume := e.permGuard.TakeReadyResume(session.ID); resume != nil {
			publish(&AgentEvent{
				Type: EventResume, SubType: resume.Tool,
				Content: fmt.Sprintf("恢复执行已批准工具: %s", resume.Tool),
				Data:    resume, Timestamp: time.Now().UnixMilli(),
			})
			tc := port.ToolCall{Name: resume.Tool, Args: resume.Args}
			callID := ensureToolCallID(tc)
			// 合成 assistant tool call 痕迹
			asst := formatToolCallsJSON([]port.ToolCall{tc})
			messages = append(messages, port.ChatMessage{Role: "assistant", Content: asst})
			_ = e.messageRepo.Save(ctx, entity.NewAssistantMessage(session.ID, asst, 0))

			publish(&AgentEvent{
				Type: EventToolCall, SubType: tc.Name, Step: 0,
				Content: fmt.Sprintf("调用工具: %s", tc.Name),
				Data:    tc.Args, Timestamp: time.Now().UnixMilli(),
			})
			resultText := budgetToolResult(e.executeTool(ctx, tc))
			totalToolCalls++
			if e.chatContext != nil {
				e.chatContext.PushToolResult(session.ID, tc.Name, resultText)
			}
			if plan != nil {
				if plan.AdvanceWithTool(tc.Name, resultText) {
					publish(&AgentEvent{
						Type: EventPlanUpdate, Content: "计划推进", Data: plan,
						Timestamp: time.Now().UnixMilli(),
					})
					// 启动下一子任务
					_ = plan.StartNext()
				}
			}
			publish(&AgentEvent{
				Type: EventToolResult, SubType: tc.Name, Step: 0,
				Content: truncate(resultText, 800), Timestamp: time.Now().UnixMilli(),
			})
			toolMsg := entity.NewToolMessage(session.ID, tc.Name, callID, resultText, 0)
			toolMsg.TokenCount = common.EstimateTokens(resultText)
			_ = e.messageRepo.Save(ctx, toolMsg)
			messages = append(messages, port.ChatMessage{
				Role: "tool", Content: resultText, Name: tc.Name, ToolCallID: callID,
			})
			messages = append(messages, port.ChatMessage{
				Role: "user", Content: e.promptService.BuildStepPrompt(1, "继续完成任务"),
			})
		}
	}

	// 10. ReAct 循环
	for step := 1; step <= loopCfg.MaxRounds; step++ {
		select {
		case <-ctx.Done():
			errClass = ErrClassCancel
			publish(NewErrorEvent(step, ErrClassCancel, "context cancelled", true))
			return &AgentResult{
				SessionID: session.ID, Response: "请求已取消", Intent: intentResult.Intent,
				Steps: step - 1, TokenUsed: totalTokens, ToolCalls: totalToolCalls,
				TaskPlan: plan, SkillID: skillID, ErrorClass: errClass,
			}, ctx.Err()
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
			errClass = ErrClassLLM
			publish(NewErrorEvent(step, ErrClassLLM, err.Error(), true))
			e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneError, err.Error(), step))
			finalAnswer = "LLM 调用失败: " + err.Error()
			break
		}

		if resp.TotalTokens > 0 {
			totalTokens += resp.TotalTokens
		} else {
			totalTokens += common.EstimateTokens(resp.Content)
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
			asstTokens := resp.TotalTokens
			if asstTokens <= 0 {
				asstTokens = common.EstimateTokens(finalAnswer)
			}
			asst.TokenCount = asstTokens
			_ = e.messageRepo.Save(ctx, asst)
			session.AddMessage(asstTokens)
			e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneTaskComplete, truncate(finalAnswer, 120), step))
			publish(&AgentEvent{Type: EventAnswer, Step: step, Content: finalAnswer, Completed: true, Timestamp: time.Now().UnixMilli()})
			break
		}

		sig := toolSignature(toolCalls)
		if sig == lastToolSig {
			sameSigCount++
			if sameSigCount >= loopCfg.DiminishingReturnsThreshold {
				errClass = ErrClassLoop
				finalAnswer = "检测到重复工具调用，已停止循环。\n\n最后模型输出：\n" + resp.Content
				publish(NewErrorEvent(step, ErrClassLoop, "diminishing returns", true))
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
					callID := ensureToolCallID(tc)
					messages = append(messages, port.ChatMessage{
						Role: "tool", Content: resultText, Name: tc.Name, ToolCallID: callID,
					})
					continue
				case security.ActionConfirm:
					p := e.permGuard.CreatePending(session.ID, tc.Name, tc.Args, dec)
					pendingPerm = &PendingPermissionInfo{
						ID: p.ID, Tool: p.Tool, Args: p.Args, Reason: p.Reason, RuleID: p.RuleID,
					}
					msg := fmt.Sprintf("⚠️ 操作需要确认\n工具: %s\n原因: %s\n确认ID: %s\n\n请在界面点击「批准」或调用 POST /api/v1/permission/approve（可设 continue=true 自动恢复），也可发送「继续」。",
						tc.Name, dec.Reason, p.ID)
					errClass = ErrClassPermission
					publish(&AgentEvent{
						Type: EventPermission, SubType: "confirm", Step: step,
						Content: msg, Data: pendingPerm, Completed: true, Timestamp: time.Now().UnixMilli(),
					})
					finalAnswer = msg
					needBreakForPerm = true
					break
				}
			}

			callID := ensureToolCallID(tc)
			publish(&AgentEvent{
				Type: EventToolCall, SubType: tc.Name, Step: step,
				Content: fmt.Sprintf("调用工具: %s", tc.Name),
				Data:    tc.Args, Timestamp: time.Now().UnixMilli(),
			})
			e.milestones.AddMilestone(session.ID, valobj.NewMilestone(valobj.MilestoneToolCalled, tc.Name, step))

			rawResult := e.executeTool(ctx, tc)
			resultText := budgetToolResult(rawResult)
			// 工具/MCP 错误分类
			if isToolError(rawResult) {
				cls := ErrClassTool
				if strings.Contains(strings.ToLower(rawResult), "mcp") {
					cls = ErrClassMCP
				}
				publish(NewErrorEvent(step, cls, truncate(rawResult, 300), false))
			}
			if e.chatContext != nil {
				e.chatContext.PushToolResult(session.ID, tc.Name, resultText)
			}
			e.milestones.DetectAndRecord(session.ID, "tool", resultText, step)

			if plan != nil {
				if plan.AdvanceWithTool(tc.Name, resultText) {
					publish(&AgentEvent{
						Type: EventPlanUpdate, Step: step, Content: "计划推进: " + tc.Name,
						Data: plan, Timestamp: time.Now().UnixMilli(),
					})
					_ = plan.StartNext()
				}
			}

			publish(&AgentEvent{
				Type: EventToolResult, SubType: tc.Name, Step: step,
				Content: truncate(resultText, 800), Timestamp: time.Now().UnixMilli(),
			})

			toolMsg := entity.NewToolMessage(session.ID, tc.Name, callID, resultText, step)
			toolMsg.TokenCount = common.EstimateTokens(resultText)
			_ = e.messageRepo.Save(ctx, toolMsg)
			messages = append(messages, port.ChatMessage{
				Role:       "tool",
				Content:    resultText,
				Name:       tc.Name,
				ToolCallID: callID,
			})
		}
		if needBreakForPerm {
			break
		}

		// 动态刷新工具列表进下一步提示（MCP 热装后同会话可见）
		stepPrompt := e.promptService.BuildStepPrompt(step, userInput)
		if plan != nil {
			stepPrompt += "\n当前计划状态:\n" + plan.StringForPrompt()
		}
		messages = append(messages, port.ChatMessage{
			Role:    "user",
			Content: stepPrompt,
		})

		if step >= loopCfg.MaxRounds {
			errClass = ErrClassLoop
			finalAnswer = "已达到最大执行步数，请根据已有结果继续或重新提问。"
			publish(NewErrorEvent(step, ErrClassLoop, finalAnswer, true))
		}
	}

	if finalAnswer == "" {
		finalAnswer = "未能生成最终答案。"
		if errClass == "" {
			errClass = ErrClassSystem
		}
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
		SkillID:   skillID,
		ErrorClass: errClass,
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

func (e *AgentEngine) filterToolInfos(sk *skillmodel.Skill, all []*valobj.ToolInfo) []*valobj.ToolInfo {
	if sk == nil || e.skills == nil || len(sk.Tools) == 0 {
		return all
	}
	names := make([]string, 0, len(all))
	byName := map[string]*valobj.ToolInfo{}
	for _, t := range all {
		names = append(names, t.Name)
		byName[t.Name] = t
	}
	filtered := e.skills.FilterTools(sk, names)
	out := make([]*valobj.ToolInfo, 0, len(filtered))
	for _, n := range filtered {
		if t, ok := byName[n]; ok {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return all
	}
	return out
}

func isContinue(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "继续" || s == "继续执行" || s == "continue" || s == "go on" || s == "ok" || s == "批准后继续" || s == "resume"
}

func isToolError(result string) bool {
	lower := strings.ToLower(result)
	return strings.Contains(result, "执行失败") ||
		strings.Contains(result, "不存在") ||
		strings.Contains(lower, "error") ||
		strings.Contains(lower, "failed")
}

func budgetToolResult(s string) string {
	if s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= maxToolResultChars {
		return s
	}
	r := []rune(s)
	return string(r[:maxToolResultChars]) + "\n...[truncated for context budget]"
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

func ensureToolCallID(tc port.ToolCall) string {
	if tc.ID != "" {
		return tc.ID
	}
	h := sha256.Sum256([]byte(tc.Name + formatToolCallsJSON([]port.ToolCall{tc})))
	return "call_" + hex.EncodeToString(h[:8])
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
