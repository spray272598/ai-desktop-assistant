package provider

import (
	"fmt"
	"strings"
	"sync"
)

// ToolResultProvider 最近工具结果摘要
type ToolResultProvider struct {
	mu      sync.RWMutex
	results map[string][]toolEntry // sessionID -> entries
	maxKeep int
}

type toolEntry struct {
	Name   string
	Result string
}

func NewToolResultProvider() *ToolResultProvider {
	return &ToolResultProvider{
		results: make(map[string][]toolEntry),
		maxKeep: 10,
	}
}

func (p *ToolResultProvider) Name() string  { return "tool_result" }
func (p *ToolResultProvider) Order() int    { return 40 }
func (p *ToolResultProvider) Enabled() bool { return true }

func (p *ToolResultProvider) Push(sessionID, toolName, result string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	list := p.results[sessionID]
	// 截断过长结果
	if len(result) > 500 {
		result = result[:500] + "..."
	}
	list = append(list, toolEntry{Name: toolName, Result: result})
	if len(list) > p.maxKeep {
		list = list[len(list)-p.maxKeep:]
	}
	p.results[sessionID] = list
}

func (p *ToolResultProvider) Provide(sessionID, _, _ string, _ []map[string]interface{}) map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	list := p.results[sessionID]
	if len(list) == 0 {
		return nil
	}
	var sb strings.Builder
	for _, e := range list {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", e.Name, e.Result))
	}
	// 最近命令
	var cmds []string
	for _, e := range list {
		if e.Name == "run_command" || e.Name == "run_script" {
			cmds = append(cmds, e.Name+": "+truncate(e.Result, 80))
		}
	}
	return map[string]interface{}{
		"toolResultSummary": sb.String(),
		"recentCommands":    cmds,
	}
}

func (p *ToolResultProvider) Clear(sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.results, sessionID)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
