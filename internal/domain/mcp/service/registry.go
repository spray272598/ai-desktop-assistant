package service

import (
	"context"
	"fmt"
	"sync"
)

// MCPTool MCP 工具描述
type MCPTool struct {
	Name        string
	Description string
	ServerName  string
}

// IMCPClient MCP 客户端端口
type IMCPClient interface {
	Name() string
	ListTools(ctx context.Context) ([]MCPTool, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error)
	Close() error
}

// Registry MCP 客户端注册表（骨架，便于后续接入 stdio/SSE）
type Registry struct {
	mu      sync.RWMutex
	clients map[string]IMCPClient
}

func NewRegistry() *Registry {
	return &Registry{clients: make(map[string]IMCPClient)}
}

func (r *Registry) Register(client IMCPClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[client.Name()] = client
}

func (r *Registry) Get(name string) IMCPClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clients[name]
}

func (r *Registry) ListTools(ctx context.Context) ([]MCPTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var all []MCPTool
	for _, c := range r.clients {
		tools, err := c.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("mcp %s: %w", c.Name(), err)
		}
		all = append(all, tools...)
	}
	return all, nil
}

func (r *Registry) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.clients {
		tools, err := c.ListTools(ctx)
		if err != nil {
			continue
		}
		for _, t := range tools {
			if t.Name == name {
				return c.CallTool(ctx, name, args)
			}
		}
	}
	return "", fmt.Errorf("mcp tool not found: %s", name)
}
