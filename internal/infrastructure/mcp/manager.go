package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/mcp/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

// ToolsChangedCallback 工具变更回调（用于同步 Agent ToolRegistry）
type ToolsChangedCallback func(tools []entity.ToolDef)

// Manager 管理多个 MCP 客户端，支持热增删
type Manager struct {
	mu        sync.RWMutex
	clients   map[string]port.IMCPClient
	configs   map[string]entity.ServerConfig
	toolRoute map[string]string
	toolDefs  []entity.ToolDef
	onChange  ToolsChangedCallback
}

func NewManager(configs []entity.ServerConfig) *Manager {
	m := &Manager{
		clients:   make(map[string]port.IMCPClient),
		configs:   make(map[string]entity.ServerConfig),
		toolRoute: make(map[string]string),
	}
	for _, c := range configs {
		m.configs[c.Name] = c
	}
	return m
}

func (m *Manager) OnToolsChanged(cb ToolsChangedCallback) {
	m.mu.Lock()
	m.onChange = cb
	m.mu.Unlock()
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	cfgs := make([]entity.ServerConfig, 0, len(m.configs))
	for _, c := range m.configs {
		cfgs = append(cfgs, c)
	}
	m.mu.RUnlock()

	for _, cfg := range cfgs {
		if !cfg.Enabled {
			continue
		}
		if err := m.startOne(ctx, cfg); err != nil {
			log.Printf("[mcp] init %s failed: %v (continue)\n", cfg.Name, err)
		}
	}
	_, err := m.refreshTools(ctx)
	return err
}

func (m *Manager) startOne(ctx context.Context, cfg entity.ServerConfig) error {
	m.mu.Lock()
	if old, ok := m.clients[cfg.Name]; ok {
		_ = old.Close()
		delete(m.clients, cfg.Name)
	}
	m.mu.Unlock()

	var client port.IMCPClient
	switch strings.ToLower(cfg.Transport) {
	case "stdio":
		client = NewStdioClient(cfg)
	case "sse", "http", "streamable":
		client = NewSSEClient(cfg)
	default:
		return fmt.Errorf("unknown transport %s", cfg.Transport)
	}
	if err := client.Initialize(ctx); err != nil {
		_ = client.Close()
		return err
	}
	m.mu.Lock()
	m.clients[cfg.Name] = client
	m.configs[cfg.Name] = cfg
	m.mu.Unlock()
	return nil
}

// AddOrUpdate 热加载：新增或更新并立即连接
func (m *Manager) AddOrUpdate(ctx context.Context, cfg entity.ServerConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("mcp server name required")
	}
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 60
	}
	m.mu.Lock()
	m.configs[cfg.Name] = cfg
	m.mu.Unlock()

	if !cfg.Enabled {
		return m.Remove(cfg.Name)
	}
	if err := m.startOne(ctx, cfg); err != nil {
		return err
	}
	_, err := m.refreshTools(ctx)
	return err
}

// Remove 热卸载
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	if c, ok := m.clients[name]; ok {
		_ = c.Close()
		delete(m.clients, name)
	}
	delete(m.configs, name)
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := m.refreshTools(ctx)
	return err
}

// ReloadAll 按配置列表全量重载
func (m *Manager) ReloadAll(ctx context.Context, configs []entity.ServerConfig) error {
	// 关闭不在新列表中的
	wanted := map[string]bool{}
	for _, c := range configs {
		wanted[c.Name] = true
	}
	m.mu.RLock()
	var toRemove []string
	for name := range m.clients {
		if !wanted[name] {
			toRemove = append(toRemove, name)
		}
	}
	m.mu.RUnlock()
	for _, name := range toRemove {
		_ = m.Remove(name)
	}
	for _, cfg := range configs {
		m.mu.Lock()
		m.configs[cfg.Name] = cfg
		m.mu.Unlock()
		if !cfg.Enabled {
			// 确保关闭
			m.mu.Lock()
			if c, ok := m.clients[cfg.Name]; ok {
				_ = c.Close()
				delete(m.clients, cfg.Name)
			}
			m.mu.Unlock()
			continue
		}
		if err := m.startOne(ctx, cfg); err != nil {
			log.Printf("[mcp] reload %s: %v\n", cfg.Name, err)
		}
	}
	_, err := m.refreshTools(ctx)
	return err
}

// ServerView MCP 服务视图（配置 + 在线状态）
type ServerView struct {
	entity.ServerConfig
	Online bool `json:"online"`
}

func (m *Manager) ListServers() []entity.ServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]entity.ServerConfig, 0, len(m.configs))
	for _, c := range m.configs {
		out = append(out, c)
	}
	return out
}

// ListServerViews 返回配置及是否在线（无死代码的在线状态）
func (m *Manager) ListServerViews() []ServerView {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ServerView, 0, len(m.configs))
	for _, c := range m.configs {
		_, online := m.clients[c.Name]
		out = append(out, ServerView{ServerConfig: c, Online: online})
	}
	return out
}

func (m *Manager) IsOnline(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[name]
	return ok
}

func (m *Manager) refreshTools(ctx context.Context) ([]entity.ToolDef, error) {
	m.mu.RLock()
	clients := make([]port.IMCPClient, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	cb := m.onChange
	m.mu.RUnlock()

	route := make(map[string]string)
	var defs []entity.ToolDef
	nameCount := map[string]int{}

	type toolPair struct {
		client port.IMCPClient
		tools  []entity.ToolDef
	}
	var pairs []toolPair
	for _, c := range clients {
		tools, err := c.ListTools(ctx)
		if err != nil {
			log.Printf("[mcp] list tools %s: %v\n", c.Name(), err)
			continue
		}
		for _, t := range tools {
			nameCount[t.Name]++
		}
		pairs = append(pairs, toolPair{client: c, tools: tools})
	}
	for _, p := range pairs {
		for _, t := range p.tools {
			name := t.Name
			if nameCount[t.Name] > 1 {
				name = p.client.Name() + "__" + t.Name
			}
			route[name] = p.client.Name() + "\x00" + t.Name
			td := t
			td.Name = name
			td.ServerName = p.client.Name()
			if name != t.Name {
				td.Description = fmt.Sprintf("[%s] %s", p.client.Name(), t.Description)
			}
			defs = append(defs, td)
		}
	}
	m.mu.Lock()
	m.toolRoute = route
	m.toolDefs = defs
	m.mu.Unlock()

	if cb != nil {
		cb(defs)
	}
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
		m.mu.RLock()
		for _, c := range m.clients {
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
