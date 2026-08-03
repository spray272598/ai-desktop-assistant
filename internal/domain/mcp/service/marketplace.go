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
	Installed   bool              `json:"installed,omitempty"`
	Online      bool              `json:"online,omitempty"`
}

// Marketplace 内置插件目录 + 安装到 MCPService
type Marketplace struct {
	mu      sync.RWMutex
	catalog map[string]MarketPlugin
	mcp     *MCPService
}

func NewMarketplace(mcp *MCPService) *Marketplace {
	m := &Marketplace{
		catalog: make(map[string]MarketPlugin),
		mcp:     mcp,
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
		{
			ID: "brave-search", Name: "Brave Search MCP", Description: "网页搜索（需 BRAVE_API_KEY + npx）",
			Transport: "stdio", Command: "npx",
			Args: []string{"-y", "@modelcontextprotocol/server-brave-search"},
			Env:  map[string]string{"BRAVE_API_KEY": ""},
			Tags: []string{"search", "official"},
		},
	}
	for _, p := range plugins {
		m.catalog[p.ID] = p
	}
}

func (m *Marketplace) serverNameFor(p MarketPlugin) string {
	if p.ID == "demo" {
		return "demo"
	}
	return p.ID
}

func (m *Marketplace) List(ctx context.Context) []MarketPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]MarketPlugin, 0, len(m.catalog))
	for _, p := range m.catalog {
		cp := p
		name := m.serverNameFor(p)
		if m.mcp != nil {
			cp.Installed = m.mcp.IsServerInstalled(ctx, name)
			cp.Online = m.mcp.IsOnline(name)
		}
		if cp.Installed {
			cp.Tags = append(append([]string{}, p.Tags...), "installed")
		}
		out = append(out, cp)
	}
	return out
}

func (m *Marketplace) Install(ctx context.Context, id string) (map[string]interface{}, error) {
	m.mu.RLock()
	p, ok := m.catalog[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", id)
	}
	if m.mcp == nil {
		return nil, fmt.Errorf("mcp service unavailable")
	}
	cfg := entity.ServerConfig{
		Name: m.serverNameFor(p), Transport: p.Transport, Command: p.Command,
		Args: p.Args, Env: p.Env, URL: p.URL, Enabled: true, TimeoutSec: 60,
	}
	if id == "demo" {
		cfg.Name = "demo"
		// command 空：InstallCustom 内对 demo 特判
	}
	return m.mcp.InstallCustom(ctx, cfg)
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
	return m.mcp.DeleteServer(ctx, m.serverNameFor(p))
}

// RegisterCatalog 运行时追加市场条目（测试/扩展）
func (m *Marketplace) RegisterCatalog(p MarketPlugin) {
	m.mu.Lock()
	m.catalog[p.ID] = p
	m.mu.Unlock()
}
