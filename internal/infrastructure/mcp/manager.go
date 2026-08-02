package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/mcp/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

// Manager 管理多个 MCP 客户端
type Manager struct {
	mu      sync.RWMutex
	clients map[string]port.IMCPClient
	configs []entity.ServerConfig
	// toolName -> serverName（同名冲突时加前缀 server__tool）
	toolRoute map[string]string
	toolDefs  []entity.ToolDef
}

func NewManager(configs []entity.ServerConfig) *Manager {
	return &Manager{
		clients:   make(map[string]port.IMCPClient),
		configs:   configs,
		toolRoute: make(map[string]string),
	}
}

func (m *Manager) Start(ctx context.Context) error {
	for _, cfg := range m.configs {
		if !cfg.Enabled {
			continue
		}
		var client port.IMCPClient
		switch strings.ToLower(cfg.Transport) {
		case "stdio":
			client = NewStdioClient(cfg)
		case "sse", "http", "streamable":
			client = NewSSEClient(cfg)
		default:
			log.Printf("[mcp] unknown transport %s for %s, skip\n", cfg.Transport, cfg.Name)
			continue
		}
		if err := client.Initialize(ctx); err != nil {
			log.Printf("[mcp] init %s failed: %v (continue)\n", cfg.Name, err)
			_ = client.Close()
			continue
		}
		m.mu.Lock()
		m.clients[cfg.Name] = client
		m.mu.Unlock()
	}
	_, err := m.refreshTools(ctx)
	return err
}

func (m *Manager) refreshTools(ctx context.Context) ([]entity.ToolDef, error) {
	m.mu.RLock()
	clients := make([]port.IMCPClient, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.mu.RUnlock()

	route := make(map[string]string)
	var defs []entity.ToolDef
	nameCount := map[string]int{}

	for _, c := range clients {
		tools, err := c.ListTools(ctx)
		if err != nil {
			log.Printf("[mcp] list tools %s: %v\n", c.Name(), err)
			continue
		}
		for _, t := range tools {
			nameCount[t.Name]++
		}
	}
	// 二次遍历，冲突则加前缀
	for _, c := range clients {
		tools, err := c.ListTools(ctx)
		if err != nil {
			continue
		}
		for _, t := range tools {
			name := t.Name
			if nameCount[t.Name] > 1 {
				name = c.Name() + "__" + t.Name
			}
			// 注册路由：对外名 -> 服务内真实名 存 server
			route[name] = c.Name() + "\x00" + t.Name
			td := t
			td.Name = name
			td.ServerName = c.Name()
			if name != t.Name {
				td.Description = fmt.Sprintf("[%s] %s", c.Name(), t.Description)
			}
			defs = append(defs, td)
		}
	}
	m.mu.Lock()
	m.toolRoute = route
	m.toolDefs = defs
	m.mu.Unlock()
	return defs, nil
}

func (m *Manager) ListTools(ctx context.Context) ([]entity.ToolDef, error) {
	m.mu.RLock()
	if len(m.toolDefs) > 0 {
		out := append([]entity.ToolDef{}, m.toolDefs...)
		m.mu.RUnlock()
		return out, nil
	}
	m.mu.RUnlock()
	return m.refreshTools(ctx)
}

func (m *Manager) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	m.mu.RLock()
	route, ok := m.toolRoute[name]
	m.mu.RUnlock()
	if !ok {
		// 尝试直接按 server 内名称
		m.mu.RLock()
		for _, c := range m.clients {
			// 尝试调用
			m.mu.RUnlock()
			if text, err := c.CallTool(ctx, name, args); err == nil {
				return text, nil
			}
			m.mu.RLock()
		}
		m.mu.RUnlock()
		return "", fmt.Errorf("mcp tool not found: %s", name)
	}
	parts := strings.SplitN(route, "\x00", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid route for %s", name)
	}
	serverName, realName := parts[0], parts[1]
	m.mu.RLock()
	client := m.clients[serverName]
	m.mu.RUnlock()
	if client == nil {
		return "", fmt.Errorf("mcp server not found: %s", serverName)
	}
	return client.CallTool(ctx, realName, args)
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, c := range m.clients {
		_ = c.Close()
		delete(m.clients, name)
	}
	return nil
}

func (m *Manager) ClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}
