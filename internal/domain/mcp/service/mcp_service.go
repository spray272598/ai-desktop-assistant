package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/agent/service"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/tools"
	mcprepo "github.com/ai-desktop/assistant/internal/domain/mcp/adapter/repository"
	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
	inframcp "github.com/ai-desktop/assistant/internal/infrastructure/mcp"
)

// MCPService 运行时 MCP 增删与工具同步
type MCPService struct {
	manager  *inframcp.Manager
	repo     mcprepo.IMCPServerRepository
	registry *service.ToolRegistry
	// 记录 MCP 工具名，便于热更新时先卸载再注册
	mcpToolNames map[string]bool
}

func NewMCPService(manager *inframcp.Manager, repo mcprepo.IMCPServerRepository, registry *service.ToolRegistry) *MCPService {
	s := &MCPService{
		manager: manager, repo: repo, registry: registry,
		mcpToolNames: make(map[string]bool),
	}
	if manager != nil {
		manager.OnToolsChanged(s.syncTools)
	}
	return s
}

func (s *MCPService) syncTools(defs []entity.ToolDef) {
	if s.registry == nil {
		return
	}
	// 移除旧 MCP 工具
	for name := range s.mcpToolNames {
		s.registry.Unregister(name)
	}
	s.mcpToolNames = make(map[string]bool)
	for _, d := range defs {
		s.registry.Register(tools.NewMCPTool(d, s.manager))
		s.mcpToolNames[d.Name] = true
	}
	log.Printf("[mcp-service] synced %d MCP tools into registry\n", len(defs))
}

func (s *MCPService) ListServers(ctx context.Context) ([]map[string]interface{}, error) {
	var configs []entity.ServerConfig
	if s.repo != nil {
		if list, err := s.repo.List(ctx); err == nil && len(list) > 0 {
			configs = list
		}
	}
	if len(configs) == 0 && s.manager != nil {
		configs = s.manager.ListServers()
	}
	out := make([]map[string]interface{}, 0, len(configs))
	for _, c := range configs {
		online := false
		if s.manager != nil {
			online = s.manager.IsOnline(c.Name)
		}
		out = append(out, map[string]interface{}{
			"name": c.Name, "transport": c.Transport, "command": c.Command,
			"args": c.Args, "url": c.URL, "enabled": c.Enabled,
			"timeoutSec": c.TimeoutSec, "online": online,
		})
	}
	return out, nil
}

func (s *MCPService) UpsertServer(ctx context.Context, cfg entity.ServerConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("name required")
	}
	if cfg.Transport == "" {
		cfg.Transport = "stdio"
	}
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 60
	}
	// 默认 enabled
	if !cfg.Enabled && cfg.Command == "" && cfg.URL == "" {
		// keep as is
	}
	if s.repo != nil {
		if err := s.repo.Save(ctx, &cfg); err != nil {
			return err
		}
	}
	if s.manager == nil {
		return fmt.Errorf("mcp manager not available")
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.manager.AddOrUpdate(cctx, cfg)
}

func (s *MCPService) DeleteServer(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("name required")
	}
	if s.repo != nil {
		_ = s.repo.Delete(ctx, name)
	}
	if s.manager != nil {
		return s.manager.Remove(name)
	}
	return nil
}

func (s *MCPService) ReloadFromDB(ctx context.Context) error {
	if s.repo == nil || s.manager == nil {
		return fmt.Errorf("repo or manager not available")
	}
	list, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return s.manager.ReloadAll(cctx, list)
}

func (s *MCPService) ListTools(ctx context.Context) ([]entity.ToolDef, error) {
	if s.manager == nil {
		return nil, nil
	}
	return s.manager.ListTools(ctx)
}
