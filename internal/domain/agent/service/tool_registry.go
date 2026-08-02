package service

import (
	"context"
	"sync"
)

type ITool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]ITool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ITool),
	}
}

func (r *ToolRegistry) Register(tool ITool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) GetTool(name string) ITool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

func (r *ToolRegistry) ListTools() []ITool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]ITool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

func (r *ToolRegistry) GetToolDescriptions() []map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var descriptions []map[string]string
	for _, tool := range r.tools {
		descriptions = append(descriptions, map[string]string{
			"name":        tool.Name(),
			"description": tool.Description(),
		})
	}
	return descriptions
}

func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

func (r *ToolRegistry) HasTool(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}
