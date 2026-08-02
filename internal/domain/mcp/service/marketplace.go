package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

// MarketPlugin 插件市场条目（可一键安装为 MCP 服务）
type MarketPlugin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Transport   string            `json:"transport"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// Marketplace 内置插件目录 + 安装到 MCPService
type Marketplace struct {
	mu       sync.RWMutex
	catalog  map[string]MarketPlugin
	mcp      *MCPService
	installed map[string]bool
}

func NewMarketplace(mcp *MCPService) *Marketplace {
	m := &Marketplace{
		catalog:   make(map[string]MarketPlugin),
		mcp:       mcp,
		installed: make(map[string]bool),
	}
	m.seed()
	return m
}

func (m *Marketplace) seed() {
	plugins := []MarketPlugin{
		{
			ID: "demo", Name: "Demo Tools", Description: "内置 get_time/echo/workspace_info",
			Transport: "stdio", Command: "", Tags: []string{"demo", "local"},
		},
		{
			ID: "fs", Name: "Filesystem MCP", Description: "官方 filesystem（需 npx）",
			Transport: "stdio", Command: "npx",
			Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "./workspace"},
			Tags: []string{"files", "official"},
		},
		{
			ID: "fetch", Name: "Fetch MCP", Description: "HTTP fetch 工具（需 npx）",
			Transport: "stdio", Command: "npx",
			Args: []string{"-y", "@modelcontextprotocol/server-fetch"},
			Tags: []string{"http", "official"},
		},
		{
			ID: "memory", Name: "Memory MCP", Description: "知识图谱记忆（需 npx）",
			Transport: "stdio", Command: "npx",
			Args: []string{"-y", "@modelcontextprotocol/server-memory"},
			Tags: []string{"memory"},
		},
	}
	for _, p := range plugins {
		m.catalog[p.ID] = p
	}
}

func (m *Marketplace) List() []MarketPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]MarketPlugin, 0, len(m.catalog))
	for _, p := range m.catalog {
		cp := p
		if m.installed[p.ID] {
			cp.Tags = append(append([]string{}, p.Tags...), "installed")
		}
		out = append(out, cp)
	}
	return out
}

func (m *Marketplace) Install(ctx context.Context, id string) error {
	m.mu.RLock()
	p, ok := m.catalog[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin not found: %s", id)
	}
	if m.mcp == nil {
		return fmt.Errorf("mcp service unavailable")
	}
	cfg := entity.ServerConfig{
		Name: p.Name, Transport: p.Transport, Command: p.Command,
		Args: p.Args, Env: p.Env, URL: p.URL, Enabled: true, TimeoutSec: 60,
	}
	// demo 特殊：name=demo 让 bootstrap findMCPDemoBinary 生效
	if id == "demo" {
		cfg.Name = "demo"
		cfg.Command = ""
	}
	if err := m.mcp.UpsertServer(ctx, cfg); err != nil {
		return err
	}
	m.mu.Lock()
	m.installed[id] = true
	m.mu.Unlock()
	return nil
}

func (m *Marketplace) Uninstall(ctx context.Context, id string) error {
	m.mu.RLock()
	p, ok := m.catalog[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin not found: %s", id)
	}
	if m.mcp == nil {
		return fmt.Errorf("mcp service unavailable")
	}
	name := p.Name
	if id == "demo" {
		name = "demo"
	}
	if err := m.mcp.DeleteServer(ctx, name); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.installed, id)
	m.mu.Unlock()
	return nil
}
