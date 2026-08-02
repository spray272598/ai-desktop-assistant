package contextsvc

import (
	"sort"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/context/provider"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/context/reducer"
)

const DefaultMaxContextTokens = 8000

// ChatContextService 组装 Prompt 上下文 + 历史裁剪
type ChatContextService struct {
	providers         []provider.ContextProvider
	hybrid            *reducer.HybridReducer
	toolResultProvider *provider.ToolResultProvider
	taskProvider      *provider.TaskProvider
	tokenBudget       int
}

func NewChatContextService(
	providers []provider.ContextProvider,
	toolResult *provider.ToolResultProvider,
	task *provider.TaskProvider,
	tokenBudget int,
) *ChatContextService {
	if tokenBudget <= 0 {
		tokenBudget = DefaultMaxContextTokens
	}
	// 按 order 排序
	sorted := make([]provider.ContextProvider, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order() < sorted[j].Order()
	})
	return &ChatContextService{
		providers:          sorted,
		hybrid:             reducer.NewHybridReducer(),
		toolResultProvider: toolResult,
		taskProvider:       task,
		tokenBudget:        tokenBudget,
	}
}

func (s *ChatContextService) BuildPromptContext(sessionID, userID, workingDir string, history []map[string]interface{}) *valobj.PromptContextVO {
	finalCtx := make(map[string]interface{})
	for _, p := range s.providers {
		if p == nil || !p.Enabled() {
			continue
		}
		partial := p.Provide(sessionID, userID, workingDir, history)
		for k, v := range partial {
			finalCtx[k] = v
		}
	}

	ctx := valobj.NewPromptContext("", userID, strVal(finalCtx, "currentDirectory"))
	if ctx.CurrentDir == "" {
		ctx.CurrentDir = workingDir
	}
	ctx.OsInfo = strVal(finalCtx, "osInfo")
	ctx.ServerInfo = strVal(finalCtx, "serverInfo")
	ctx.ToolResultSummary = strVal(finalCtx, "toolResultSummary")
	ctx.TaskDescription = strVal(finalCtx, "taskDescription")
	ctx.CoreMemories = strVal(finalCtx, "coreMemories")

	if ms, ok := finalCtx["milestoneVOS"].([]*valobj.MilestoneVO); ok {
		ctx.Milestones = ms
	}
	if cmds, ok := finalCtx["recentCommands"].([]string); ok {
		ctx.RecentCommands = cmds
	}
	return ctx
}

func (s *ChatContextService) TrimHistory(history []map[string]interface{}, tokenBudget int) []map[string]interface{} {
	if tokenBudget <= 0 {
		tokenBudget = s.tokenBudget
	}
	return s.hybrid.Reduce(history, tokenBudget)
}

func (s *ChatContextService) PushToolResult(sessionID, toolName, result string) {
	if s.toolResultProvider != nil {
		s.toolResultProvider.Push(sessionID, toolName, result)
	}
}

func (s *ChatContextService) SetTask(sessionID, desc string) {
	if s.taskProvider != nil {
		s.taskProvider.SetTask(sessionID, desc)
	}
}

func (s *ChatContextService) TokenBudget() int {
	return s.tokenBudget
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
