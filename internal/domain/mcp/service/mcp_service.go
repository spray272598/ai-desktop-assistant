package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	lastErrors   map[string]string // server -> last error
}

func NewMCPService(manager *inframcp.Manager, repo mcprepo.IMCPServerRepository, registry *service.ToolRegistry) *MCPService {
	s := &MCPService{
		manager: manager, repo: repo, registry: registry,
		mcpToolNames: make(map[string]bool),
		lastErrors:   make(map[string]string),
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

// HealthStatus 单服务健康
type HealthStatus struct {
	Name      string `json:"name"`
	Online    bool   `json:"online"`
	Transport string `json:"transport,omitempty"`
	ToolCount int    `json:"toolCount"`
	LastError string `json:"lastError,omitempty"`
	Enabled   bool   `json:"enabled"`
}

func (s *MCPService) Health(ctx context.Context) []HealthStatus {
	views := []HealthStatus{}
	seen := map[string]bool{}
	if s.manager != nil {
		for _, v := range s.manager.ListServerViews() {
			tools, _ := s.ListToolsByServer(ctx, v.Name)
			errMsg := ""
			if s.lastErrors != nil {
				errMsg = s.lastErrors[v.Name]
			}
			views = append(views, HealthStatus{
				Name: v.Name, Online: v.Online, Transport: v.Transport,
				ToolCount: len(tools), LastError: errMsg, Enabled: v.Enabled,
			})
			seen[v.Name] = true
		}
	}
	if s.repo != nil {
		if list, err := s.repo.List(ctx); err == nil {
			for _, c := range list {
				if seen[c.Name] {
					continue
				}
				online := false
				if s.manager != nil {
					online = s.manager.IsOnline(c.Name)
				}
				views = append(views, HealthStatus{
					Name: c.Name, Online: online, Transport: c.Transport,
					Enabled: c.Enabled, LastError: s.lastErrors[c.Name],
				})
			}
		}
	}
	return views
}

func (s *MCPService) ListServers(ctx context.Context) ([]map[string]interface{}, error) {
	// 优先 Manager 视图（含在线状态）；再合并 repo 中未启动的配置
	byName := map[string]map[string]interface{}{}
	if s.manager != nil {
		for _, v := range s.manager.ListServerViews() {
			tools, _ := s.ListToolsByServer(ctx, v.Name)
			toolNames := make([]string, 0, len(tools))
			for _, t := range tools {
				toolNames = append(toolNames, t.Name)
			}
			byName[v.Name] = map[string]interface{}{
				"name": v.Name, "transport": v.Transport, "command": v.Command,
				"args": v.Args, "url": v.URL, "enabled": v.Enabled,
				"timeoutSec": v.TimeoutSec, "online": v.Online,
				"toolCount": len(tools), "tools": toolNames,
				"lastError": s.lastErrors[v.Name],
			}
		}
	}
	if s.repo != nil {
		list, err := s.repo.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list mcp servers from db: %w", err)
		}
		for _, c := range list {
			if _, ok := byName[c.Name]; ok {
				continue
			}
			online := false
			if s.manager != nil {
				online = s.manager.IsOnline(c.Name)
			}
			byName[c.Name] = map[string]interface{}{
				"name": c.Name, "transport": c.Transport, "command": c.Command,
				"args": c.Args, "url": c.URL, "enabled": c.Enabled,
				"timeoutSec": c.TimeoutSec, "online": online,
				"lastError": s.lastErrors[c.Name],
			}
		}
	}
	out := make([]map[string]interface{}, 0, len(byName))
	for _, v := range byName {
		out = append(out, v)
	}
	return out, nil
}

func (s *MCPService) IsOnline(name string) bool {
	if s.manager == nil {
		return false
	}
	return s.manager.IsOnline(name)
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
	// demo 空 command 时尝试解析本地 mcp-demo 二进制
	if strings.EqualFold(cfg.Name, "demo") && cfg.Command == "" {
		cfg.Command = findMCPDemoBinary()
	}
	if strings.EqualFold(cfg.Transport, "stdio") && cfg.Command == "" && cfg.URL == "" {
		return fmt.Errorf("stdio transport requires command (for demo: go build -o mcp-demo.exe ./cmd/mcp-demo)")
	}
	if (strings.EqualFold(cfg.Transport, "sse") || strings.EqualFold(cfg.Transport, "http")) && cfg.URL == "" {
		return fmt.Errorf("sse/http transport requires url")
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
	if err := s.manager.AddOrUpdate(cctx, cfg); err != nil {
		s.lastErrors[cfg.Name] = err.Error()
		return err
	}
	delete(s.lastErrors, cfg.Name)
	return nil
}

// InstallCustom 自定义安装（npx / 本地 binary / SSE），返回当前工具列表
func (s *MCPService) InstallCustom(ctx context.Context, cfg entity.ServerConfig) (map[string]interface{}, error) {
	if err := s.UpsertServer(ctx, cfg); err != nil {
		return nil, err
	}
	tools, _ := s.ListToolsByServer(ctx, cfg.Name)
	online := s.manager != nil && s.manager.IsOnline(cfg.Name)
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	return map[string]interface{}{
		"ok": true, "name": cfg.Name, "online": online,
		"toolCount": len(tools), "tools": names,
	}, nil
}

func (s *MCPService) DeleteServer(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("name required")
	}
	if s.repo != nil {
		_ = s.repo.Delete(ctx, name)
	}
	delete(s.lastErrors, name)
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
	if err := s.manager.ReloadAll(cctx, list); err != nil {
		return err
	}
	// 清错误缓存中已上线的
	for _, c := range list {
		if s.manager.IsOnline(c.Name) {
			delete(s.lastErrors, c.Name)
		}
	}
	return nil
}

func (s *MCPService) ListTools(ctx context.Context) ([]entity.ToolDef, error) {
	if s.manager == nil {
		return nil, nil
	}
	return s.manager.ListTools(ctx)
}

// ListToolsByServer 列出某 MCP server 暴露的工具（含前缀名）
func (s *MCPService) ListToolsByServer(ctx context.Context, serverName string) ([]entity.ToolDef, error) {
	all, err := s.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	if serverName == "" {
		return all, nil
	}
	var out []entity.ToolDef
	for _, t := range all {
		if t.ServerName == serverName || strings.HasPrefix(t.Name, serverName+"__") {
			out = append(out, t)
		}
	}
	return out, nil
}

// IsServerInstalled 名称是否在 manager 或 DB 中
func (s *MCPService) IsServerInstalled(ctx context.Context, name string) bool {
	if s.manager != nil {
		for _, v := range s.manager.ListServers() {
			if v.Name == name {
				return true
			}
		}
	}
	if s.repo != nil {
		if c, _ := s.repo.FindByName(ctx, name); c != nil {
			return true
		}
	}
	return false
}

func findMCPDemoBinary() string {
	candidates := []string{"./mcp-demo", "./mcp-demo.exe", "./bin/mcp-demo", "./bin/mcp-demo.exe"}
	if ex, err := os.Executable(); err == nil {
		dir := filepath.Dir(ex)
		candidates = append(candidates, filepath.Join(dir, "mcp-demo"), filepath.Join(dir, "mcp-demo.exe"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	if runtime.GOOS == "windows" {
		log.Println("[mcp] tip: go build -o mcp-demo.exe ./cmd/mcp-demo")
	}
	return ""
}
