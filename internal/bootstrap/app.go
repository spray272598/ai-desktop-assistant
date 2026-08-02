package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ai-desktop/assistant/internal/application"
	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/repository"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
	"github.com/ai-desktop/assistant/internal/domain/agent/service"
	contextsvc "github.com/ai-desktop/assistant/internal/domain/agent/service/context"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/context/provider"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/engine"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/intent"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/orchestrator"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/prompt"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/security"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/task"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/tools"
	mcprepo "github.com/ai-desktop/assistant/internal/domain/mcp/adapter/repository"
	mcpentity "github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
	mcpsvc "github.com/ai-desktop/assistant/internal/domain/mcp/service"
	desktopService "github.com/ai-desktop/assistant/internal/domain/desktop/service"
	"github.com/ai-desktop/assistant/internal/infrastructure/config"
	"github.com/ai-desktop/assistant/internal/infrastructure/llm"
	inframcp "github.com/ai-desktop/assistant/internal/infrastructure/mcp"
	"github.com/ai-desktop/assistant/internal/infrastructure/mysql"
	redisx "github.com/ai-desktop/assistant/internal/infrastructure/redis"
	infraRepo "github.com/ai-desktop/assistant/internal/infrastructure/repository"
	"github.com/ai-desktop/assistant/internal/trigger/ws"
)

// App 组装后的应用根
type App struct {
	Config      *config.Config
	AgentApp    *application.AgentApp
	Engine      *engine.AgentEngine
	Tools       *service.ToolRegistry
	MCP         *inframcp.Manager
	MCPService  *mcpsvc.MCPService
	Marketplace *mcpsvc.Marketplace
	Export      *application.ExportService
	Redis       *redisx.Client
	WSHub       *ws.Hub
	ModelRouter *orchestrator.ModelRouter
	PermGuard   *security.PermissionGuard
	Closer      func()
}

func Build(cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	sessionRepo, messageRepo, milestoneRepo, memoryRepo, mcpRepo, closer, err := buildRepos(cfg)
	if err != nil {
		return nil, err
	}

	// Redis
	rdb := redisx.New(cfg.Redis)

	llmPort := llm.NewFromConfig(cfg)
	tracker := intent.NewContextTracker()
	intentService := intent.NewIntentService(intent.NewRuleClassifier(), llmPort, tracker)

	promptService := prompt.NewPromptService()
	milestoneTracker := prompt.NewMilestoneTracker(100)
	milestoneTracker.SetRepository(milestoneRepo)
	milestoneTracker.SetMemoryRepository(memoryRepo)

	envProvider := provider.NewEnvProvider(cfg.Desktop.Workspace)
	taskProvider := provider.NewTaskProvider()
	milestoneProvider := provider.NewMilestoneProvider(milestoneTracker)
	toolResultProvider := provider.NewToolResultProvider()
	memoryProvider := provider.NewCoreMemoryProvider(memoryRepo)
	chatContext := contextsvc.NewChatContextService(
		[]provider.ContextProvider{envProvider, taskProvider, milestoneProvider, toolResultProvider, memoryProvider},
		toolResultProvider, taskProvider, cfg.Agent.TokenBudget,
	)

	fileService := desktopService.NewFileService(cfg.Tools.File.BaseDir)
	cmdService := desktopService.NewCommandServiceWithPolicy(cfg.Tools.Command.MaxTimeout, cfg.Tools.Command.DenyList)
	shotService := desktopService.NewScreenshotService(cfg.Desktop.ScreenshotDir)
	browserService := desktopService.NewBrowserService(cfg.Desktop.ScreenshotDir)
	sandboxService := desktopService.NewSandboxService(cfg.Sandbox.WorkDir, cfg.Sandbox.UseDocker)

	toolRegistry := service.NewToolRegistry()
	if cfg.Tools.File.Enabled {
		toolRegistry.Register(tools.NewReadFileTool(fileService))
		toolRegistry.Register(tools.NewWriteFileTool(fileService))
		toolRegistry.Register(tools.NewListFilesTool(fileService))
		toolRegistry.Register(tools.NewDeleteFileTool(fileService))
	}
	if cfg.Tools.Command.Enabled {
		toolRegistry.Register(tools.NewRunCommandTool(cmdService))
		toolRegistry.Register(tools.NewRunScriptTool(cmdService))
	}
	if cfg.Tools.Screenshot.Enabled {
		toolRegistry.Register(tools.NewScreenshotTool(shotService))
	}
	if cfg.Tools.Browser.Enabled {
		toolRegistry.Register(tools.NewOpenURLTool(browserService))
		toolRegistry.Register(tools.NewBrowserScreenshotTool(browserService))
		toolRegistry.Register(tools.NewBrowserEvalTool(browserService))
		toolRegistry.Register(tools.NewBrowserHTMLTool(browserService))
	}
	if cfg.Tools.Sandbox.Enabled || cfg.Sandbox.Enabled {
		toolRegistry.Register(tools.NewRunCodeTool(sandboxService))
	}

	permGuard := security.NewPermissionGuard()
	breakdown := task.NewBreakdownService(llmPort)

	// Model A/B router
	modelRouter := orchestrator.NewModelRouter(cfg.LLM.Model)
	for name, m := range cfg.LLM.Models {
		modelRouter.Register(orchestrator.ModelProfile{
			Name: name, APIBase: m.APIBase, Weight: m.Weight, Scenarios: m.Scenarios,
		})
	}
	orch := orchestrator.NewMultiAgentOrchestrator(llmPort, breakdown, modelRouter)

	// MCP
	var mcpManager *inframcp.Manager
	var mcpService *mcpsvc.MCPService
	var market *mcpsvc.Marketplace
	if cfg.MCP.Enabled {
		mcpCfgs := resolveMCPConfigs(cfg)
		if mcpRepo != nil {
			if dbList, err := mcpRepo.ListEnabled(context.Background()); err == nil && len(dbList) > 0 {
				byName := map[string]mcpentity.ServerConfig{}
				for _, c := range mcpCfgs {
					byName[c.Name] = c
				}
				for _, c := range dbList {
					byName[c.Name] = c
				}
				mcpCfgs = nil
				for _, c := range byName {
					mcpCfgs = append(mcpCfgs, c)
				}
			}
			for _, c := range resolveMCPConfigs(cfg) {
				if existing, _ := mcpRepo.FindByName(context.Background(), c.Name); existing == nil {
					_ = mcpRepo.Save(context.Background(), &c)
				}
			}
		}
		mcpManager = inframcp.NewManager(mcpCfgs)
		mcpService = mcpsvc.NewMCPService(mcpManager, mcpRepo, toolRegistry)
		market = mcpsvc.NewMarketplace(mcpService)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = mcpManager.Start(ctx)
		cancel()
		if toolsList, err := mcpManager.ListTools(context.Background()); err == nil {
			for _, t := range toolsList {
				toolRegistry.Register(tools.NewMCPTool(t, mcpManager))
			}
			log.Printf("[bootstrap] MCP tools=%d\n", len(toolsList))
		}
	}

	agent := entity.NewAgentEntity("desktop-agent", cfg.Agent.Name,
		"桌面智能助手：文件/命令/浏览器/沙箱/MCP/多Agent",
		"你是桌面 AI 助手。能力：文件、命令、浏览器、代码沙箱、MCP 扩展。复杂任务先规划再执行，危险操作等待确认。")
	agent.MaxSteps = cfg.Agent.MaxSteps

	loopCfg := engine.DefaultLoopConfig()
	loopCfg.MaxRounds = cfg.Agent.MaxSteps
	loopCfg.MaxTokenBudget = cfg.Agent.TokenBudget

	eng := engine.NewAgentEngine(agent, intentService, promptService, milestoneTracker, chatContext,
		llmPort, toolRegistry, sessionRepo, messageRepo, loopCfg)
	eng.SetBreakdown(breakdown)
	eng.SetPermissionGuard(permGuard)
	eng.SetOrchestrator(orch)

	agentApp := application.NewAgentApp(eng, sessionRepo, messageRepo, "desktop-agent", cfg.Agent.Timeout, cfg.Desktop.Workspace)
	agentApp.SetMCPService(mcpService)
	agentApp.SetPermissionGuard(permGuard)
	agentApp.SetMarketplace(market)
	agentApp.SetRedis(rdb)
	agentApp.SetRateLimit(cfg.RateLimit.Enabled, cfg.RateLimit.PerMinute)

	exportSvc := application.NewExportService(sessionRepo, messageRepo, "./exports")
	agentApp.SetExport(exportSvc)

	wsHub := ws.NewHub()

	log.Printf("[bootstrap] agent=%s db=%s tools=%d redis=%v browser=%v sandbox=%v\n",
		cfg.Agent.Name, cfg.Database.Type, len(toolRegistry.ListTools()), rdb.Enabled(),
		cfg.Tools.Browser.Enabled, cfg.Sandbox.Enabled)

	return &App{
		Config: cfg, AgentApp: agentApp, Engine: eng, Tools: toolRegistry,
		MCP: mcpManager, MCPService: mcpService, Marketplace: market,
		Export: exportSvc, Redis: rdb, WSHub: wsHub, ModelRouter: modelRouter,
		PermGuard: permGuard,
		Closer: func() {
			if mcpManager != nil {
				_ = mcpManager.Close()
			}
			_ = rdb.Close()
			if closer != nil {
				closer()
			}
		},
	}, nil
}

func buildRepos(cfg *config.Config) (
	repository.ISessionRepository,
	repository.IMessageRepository,
	repository.IMilestoneRepository,
	repository.ICoreMemoryRepository,
	mcprepo.IMCPServerRepository,
	func(),
	error,
) {
	if strings.ToLower(cfg.Database.Type) == "mysql" {
		db, err := mysql.Open(cfg.Database.MySQL, cfg.Database.AutoMigrate, cfg.Database.SchemaPath)
		if err != nil {
			log.Printf("[bootstrap] mysql unavailable (%v), fallback memory\n", err)
		} else {
			return infraRepo.NewMySQLSessionRepository(db),
				infraRepo.NewMySQLMessageRepository(db),
				infraRepo.NewMySQLMilestoneRepository(db),
				infraRepo.NewMySQLCoreMemoryRepository(db),
				infraRepo.NewMySQLMCPServerRepository(db),
				func() { _ = db.Close() }, nil
		}
	}
	cfg.Database.Type = "memory"
	return infraRepo.NewMemorySessionRepository(),
		infraRepo.NewMemoryMessageRepository(),
		infraRepo.NewMemoryMilestoneRepository(),
		infraRepo.NewMemoryCoreMemoryRepository(),
		infraRepo.NewMemoryMCPServerRepository(),
		func() {}, nil
}

func resolveMCPConfigs(cfg *config.Config) []mcpentity.ServerConfig {
	var out []mcpentity.ServerConfig
	for _, s := range cfg.MCP.Servers {
		enabled := true
		if s.Enabled != nil {
			enabled = *s.Enabled
		}
		if !enabled {
			continue
		}
		cmd := s.Command
		if strings.EqualFold(s.Transport, "stdio") && cmd == "" {
			cmd = findMCPDemoBinary()
			if cmd == "" {
				continue
			}
		}
		out = append(out, mcpentity.ServerConfig{
			Name: s.Name, Transport: s.Transport, Command: cmd,
			Args: s.Args, Env: s.Env, URL: s.URL, Enabled: true, TimeoutSec: s.TimeoutSec,
		})
	}
	return out
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
