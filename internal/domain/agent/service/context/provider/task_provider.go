package provider

import (
	"strings"
	"sync"
)

// TaskProvider 当前任务描述
type TaskProvider struct {
	mu    sync.RWMutex
	tasks map[string]string
}

func NewTaskProvider() *TaskProvider {
	return &TaskProvider{tasks: make(map[string]string)}
}

func (p *TaskProvider) Name() string  { return "task" }
func (p *TaskProvider) Order() int    { return 20 }
func (p *TaskProvider) Enabled() bool { return true }

func (p *TaskProvider) SetTask(sessionID, desc string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tasks[sessionID] = strings.TrimSpace(desc)
}

func (p *TaskProvider) Provide(sessionID, _, _ string, history []map[string]interface{}) map[string]interface{} {
	p.mu.RLock()
	task := p.tasks[sessionID]
	p.mu.RUnlock()

	// 若未显式设置，取最近用户消息作为任务
	if task == "" {
		for i := len(history) - 1; i >= 0; i-- {
			if role, _ := history[i]["role"].(string); role == "user" {
				if c, ok := history[i]["content"].(string); ok && c != "" {
					task = c
					if len(task) > 200 {
						task = task[:200] + "..."
					}
					break
				}
			}
		}
	}
	if task == "" {
		return nil
	}
	return map[string]interface{}{
		"taskDescription": task,
	}
}
