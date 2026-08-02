package task

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

// BreakdownService 任务拆解：规则 + LLM（对标 walicode TaskBreakdownService）
type BreakdownService struct {
	llm   port.ILLMPort
	mu    sync.RWMutex
	cache map[string]*valobj.TaskPlan // sessionID -> plan
}

var complexTaskPattern = regexp.MustCompile(`(?i)(部署|安装并配置|搭建|迁移|重构|架构|CI/CD|持续集成|` +
	`deploy.*and.*config|set.*up|migrate|refactor|build.*pipeline|` +
	`从零|完整|全流程|step.by.step|然后|接着|之后|最后|并|同时|先.*再|一步步|分步骤)`)

func NewBreakdownService(llm port.ILLMPort) *BreakdownService {
	return &BreakdownService{
		llm:   llm,
		cache: make(map[string]*valobj.TaskPlan),
	}
}

func (s *BreakdownService) ShouldBreakdown(sessionID, userMessage string) bool {
	if strings.TrimSpace(userMessage) == "" {
		return false
	}
	s.mu.RLock()
	_, cached := s.cache[sessionID]
	s.mu.RUnlock()
	if cached {
		return false
	}
	if complexTaskPattern.MatchString(userMessage) {
		return true
	}
	// 多步骤信号
	signals := []string{"然后", "接着", "之后", "最后", "and then", "after that", "finally", "第一步", "第二步"}
	count := 0
	lower := strings.ToLower(userMessage)
	for _, sig := range signals {
		if strings.Contains(lower, strings.ToLower(sig)) {
			count++
		}
	}
	// 长请求也倾向拆解
	if len([]rune(userMessage)) > 80 {
		return true
	}
	return count >= 2
}

func (s *BreakdownService) GetPlan(sessionID string) *valobj.TaskPlan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[sessionID]
}

func (s *BreakdownService) Clear(sessionID string) {
	s.mu.Lock()
	delete(s.cache, sessionID)
	s.mu.Unlock()
}

func (s *BreakdownService) Breakdown(ctx context.Context, sessionID, userMessage string) *valobj.TaskPlan {
	if plan := s.GetPlan(sessionID); plan != nil {
		plan.Source = "cache"
		return plan
	}
	if !s.ShouldBreakdown(sessionID, userMessage) {
		return nil
	}

	// 规则层快速拆分
	rulePlan := s.ruleBreakdown(userMessage)
	if s.llm == nil {
		if rulePlan != nil {
			s.store(sessionID, rulePlan)
		}
		return rulePlan
	}

	// LLM 层
	llmPlan := s.llmBreakdown(ctx, userMessage)
	if llmPlan != nil && len(llmPlan.SubTasks) >= 2 {
		s.store(sessionID, llmPlan)
		return llmPlan
	}
	if rulePlan != nil {
		s.store(sessionID, rulePlan)
		return rulePlan
	}
	return nil
}

func (s *BreakdownService) store(sessionID string, plan *valobj.TaskPlan) {
	s.mu.Lock()
	s.cache[sessionID] = plan
	s.mu.Unlock()
}

func (s *BreakdownService) ruleBreakdown(msg string) *valobj.TaskPlan {
	// 按常见连接词拆分
	parts := splitSteps(msg)
	if len(parts) < 2 {
		// 通用模板：理解 → 执行 → 验证
		if !complexTaskPattern.MatchString(msg) && len([]rune(msg)) < 80 {
			return nil
		}
		parts = []string{
			"分析需求与当前环境",
			"执行核心操作: " + truncate(msg, 60),
			"验证结果并汇报",
		}
	}
	subs := make([]valobj.SubTask, 0, len(parts))
	for i, p := range parts {
		subs = append(subs, valobj.SubTask{
			Index: i + 1, Title: truncate(p, 24), Description: p,
			ExpectedTools: guessTools(p), Status: "pending",
		})
	}
	return &valobj.TaskPlan{
		OriginalRequest: msg,
		Summary:         "多步任务计划（规则拆解）",
		NeedConfirm:     false,
		SubTasks:        subs,
		Source:          "rule",
	}
}

func (s *BreakdownService) llmBreakdown(ctx context.Context, msg string) *valobj.TaskPlan {
	sys := `你是任务分析专家。将用户复杂桌面/开发请求拆为有序子任务。
只返回 JSON（不要 markdown）：
{"summary":"...","subTasks":[{"title":"...","description":"...","expectedTools":"list_files,run_command"}]}
原则：2-8 个子任务；检查环境→执行→验证；简单任务返回空 subTasks。`

	resp, err := s.llm.Generate(ctx, &port.ChatRequest{
		SystemPrompt: sys,
		Messages:     []port.ChatMessage{{Role: "user", Content: msg}},
		Temperature:  0.2,
		MaxTokens:    800,
	})
	if err != nil || resp == nil {
		return nil
	}
	content := strings.TrimSpace(resp.Content)
	if i := strings.Index(content, "{"); i >= 0 {
		if j := strings.LastIndex(content, "}"); j > i {
			content = content[i : j+1]
		}
	}
	var parsed struct {
		Summary  string `json:"summary"`
		SubTasks []struct {
			Title         string `json:"title"`
			Description   string `json:"description"`
			ExpectedTools string `json:"expectedTools"`
		} `json:"subTasks"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil
	}
	if len(parsed.SubTasks) < 2 {
		return nil
	}
	subs := make([]valobj.SubTask, 0, len(parsed.SubTasks))
	for i, t := range parsed.SubTasks {
		subs = append(subs, valobj.SubTask{
			Index: i + 1, Title: t.Title, Description: t.Description,
			ExpectedTools: t.ExpectedTools, Status: "pending",
		})
	}
	return &valobj.TaskPlan{
		OriginalRequest: msg,
		Summary:         parsed.Summary,
		SubTasks:        subs,
		Source:          "llm",
	}
}

func splitSteps(msg string) []string {
	seps := []string{"然后", "接着", "之后", "最后", "；", ";", "，并且", "并且", " and then ", " after that ", " finally "}
	parts := []string{msg}
	for _, sep := range seps {
		var next []string
		for _, p := range parts {
			chunks := strings.Split(p, sep)
			for _, c := range chunks {
				c = strings.TrimSpace(c)
				if c != "" {
					next = append(next, c)
				}
			}
		}
		parts = next
	}
	// 去重过短
	var out []string
	for _, p := range parts {
		if len([]rune(p)) >= 2 {
			out = append(out, p)
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func guessTools(step string) string {
	lower := strings.ToLower(step)
	var tools []string
	if strings.Contains(lower, "文件") || strings.Contains(lower, "file") || strings.Contains(lower, "写入") {
		tools = append(tools, "write_file", "read_file")
	}
	if strings.Contains(lower, "命令") || strings.Contains(lower, "执行") || strings.Contains(lower, "run") || strings.Contains(lower, "安装") {
		tools = append(tools, "run_command")
	}
	if strings.Contains(lower, "目录") || strings.Contains(lower, "列出") {
		tools = append(tools, "list_files")
	}
	if len(tools) == 0 {
		return "list_files,run_command"
	}
	return strings.Join(tools, ",")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
