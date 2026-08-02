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
	"github.com/ai-desktop/assistant/internal/domain/agent/service/prompt"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/tools"
	mcpentity "github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
	desktopService "github.com/ai-desktop/assistant/internal/domain/desktop/service"
	"github.com/ai-desktop/assistant/internal/infrastructure/config"
	"github.com/ai-desktop/assistant/internal/infrastructure/llm"
	inframcp "github.com/ai-desktop/assistant/internal/infrastructure/mcp"
	"github.com/ai-desktop/assistant/internal/infrastructure/mysql"
	infraRepo "github.com/ai-desktop/assistant/internal/infrastructure/repository"
)

// App 组装后的应用根
type App struct {
	Config   *config.Config
	AgentApp *application.AgentApp
	Engine   *engine.AgentEngine
	Tools    *service.ToolRegistry
	MCP      *inframcp.Manager
	Closer   func()
}

// Build 根据配置组装全部依赖
func Build(cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// ---- 仓储：MySQL 优先，失败可降级 memory ----
	sessionRepo, messageRepo, milestoneRepo, memoryRepo, closer, err := buildRepos(cfg)
	if err != nil {
		return nil, err
	}

	// LLM
	llmPort := llm.NewFromConfig(cfg)

	// 意图
	tracker := intent.NewContextTracker()
	intentService := intent.NewIntentService(intent.NewRuleClassifier(), llmPort, tracker)

	// 提示词 / 里程碑（持久化）
	promptService := prompt.NewPromptService()
	milestoneTracker := prompt.NewMilestoneTracker(100)
	milestoneTracker.SetRepository(milestoneRepo)
	milestoneTracker.SetMemoryRepository(memoryRepo)

	// 上下文 providers
	envProvider := provider.NewEnvProvider(cfg.Desktop.Workspace)
	taskProvider := provider.NewTaskProvider()
	milestoneProvider := provider.NewMilestoneProvider(milestoneTracker)
	toolResultProvider := provider.NewToolResultProvider()
	memoryProvider := provider.NewCoreMemoryProvider(memoryRepo)
	chatContext := contextsvc.NewChatContextService(
		[]provider.ContextProvider{envProvider, taskProvider, milestoneProvider, toolResultProvider, memoryProvider},
		toolResultProvider,
		taskProvider,
		cfg.Agent.TokenBudget,
	)

	// 桌面服务（文件 + 命令为核心）
	fileService := desktopService.NewFileService(cfg.Tools.File.BaseDir)
	cmdService := desktopService.NewCommandServiceWithPolicy(
		cfg.Tools.Command.MaxTimeout,
		cfg.Tools.Command.DenyList,
	)
	shotService := desktopService.NewScreenshotService(cfg.Desktop.ScreenshotDir)

	// 工具注册：本地工具 + MCP 工具
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

	// MCP Manager
	var mcpManager *inframcp.Manager
	if cfg.MCP.Enabled {
		mcpCfgs := resolveMCPConfigs(cfg)
		mcpManager = inframcp.NewManager(mcpCfgs)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := mcpManager.Start(ctx); err != nil {
			log.Printf("[bootstrap] mcp start warn: %v\n", err)
		}
		cancel()
		// 注册 MCP 工具到 Agent
		if mcpTools, err := mcpManager.ListTools(context.Background()); err == nil {
			for _, t := range mcpTools {
				toolRegistry.Register(tools.NewMCPTool(t, mcpManager))
			}
			log.Printf("[bootstrap] registered %d MCP tools\n", len(mcpTools))
		}
	}

	// Agent 实体
	agent := entity.NewAgentEntity(
		"desktop-agent",
		cfg.Agent.Name,
		"桌面 AI 助手：会话持久化 + 本地文件/命令 + MCP 扩展工具",
		"你是一个强大的桌面AI助手。核心能力：理解用户意图、操作工作区文件、执行命令、调用 MCP 扩展工具、保持跨轮对话上下文。",
	)
	agent.MaxSteps = cfg.Agent.MaxSteps

	loopCfg := engine.DefaultLoopConfig()
	loopCfg.MaxRounds = cfg.Agent.MaxSteps
	loopCfg.MaxTokenBudget = cfg.Agent.TokenBudget

	eng := engine.NewAgentEngine(
		agent,
		intentService,
		promptService,
		milestoneTracker,
		chatContext,
		llmPort,
		toolRegistry,
		sessionRepo,
		messageRepo,
		loopCfg,
	)

	agentApp := application.NewAgentApp(
		eng,
		sessionRepo,
		messageRepo,
		"desktop-agent",
		cfg.Agent.Timeout,
		cfg.Desktop.Workspace,
	)

	log.Printf("[bootstrap] agent=%s db=%s maxSteps=%d tools=%d mcpClients=%d workspace=%s\n",
		cfg.Agent.Name, cfg.Database.Type, cfg.Agent.MaxSteps,
		len(toolRegistry.ListTools()),
		func() int {
			if mcpManager == nil {
				return 0
			}
			return mcpManager.ClientCount()
		}(),
		cfg.Desktop.Workspace)

	appCloser := func() {
		if mcpManager != nil {
			_ = mcpManager.Close()
		}
		if closer != nil {
			closer()
		}
	}

	return &App{
		Config:   cfg,
		AgentApp: agentApp,
		Engine:   eng,
		Tools:    toolRegistry,
		MCP:      mcpManager,
		Closer:   appCloser,
	}, nil
}

func buildRepos(cfg *config.Config) (
	repository.ISessionRepository,
	repository.IMessageRepository,
	repository.IMilestoneRepository,
	repository.ICoreMemoryRepository,
	func(),
	error,
) {
	dbType := strings.ToLower(cfg.Database.Type)
	if dbType == "mysql" {
		db, err := mysql.Open(cfg.Database.MySQL, cfg.Database.AutoMigrate, cfg.Database.SchemaPath)
		if err != nil {
			log.Printf("[bootstrap] mysql unavailable (%v), fallback to memory\n", err)
		} else {
			return infraRepo.NewMySQLSessionRepository(db),
				infraRepo.NewMySQLMessageRepository(db),
				infraRepo.NewMySQLMilestoneRepository(db),
				infraRepo.NewMySQLCoreMemoryRepository(db),
				func() { _ = db.Close() },
				nil
		}
	}
	// memory
	cfg.Database.Type = "memory"
	return infraRepo.NewMemorySessionRepository(),
		infraRepo.NewMemoryMessageRepository(),
		infraRepo.NewMemoryMilestoneRepository(),
		infraRepo.NewMemoryCoreMemoryRepository(),
		func() {},
		nil
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
				log.Printf("[mcp] skip %s: no command and mcp-demo not found\n", s.Name)
				continue
			}
		}
		out = append(out, mcpentity.ServerConfig{
			Name: s.Name, Transport: s.Transport, Command: cmd,
			Args: s.Args, Env: s.Env, URL: s.URL,
			Enabled: true, TimeoutSec: s.TimeoutSec,
		})
	}
	return out
}

func findMCPDemoBinary() string {
	candidates := []string{
		"./mcp-demo",
		"./mcp-demo.exe",
		"./bin/mcp-demo",
		"./bin/mcp-demo.exe",
	}
	// 相对可执行文件目录
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "mcp-demo"),
			filepath.Join(dir, "mcp-demo.exe"),
		)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	// Windows 上尝试 go run（较慢，仅开发）
	if runtime.GOOS == "windows" {
		// 不自动 go run，避免启动过慢；提示用户先 build
		log.Println("[mcp] tip: build demo server with: go build -o mcp-demo.exe ./cmd/mcp-demo")
	}
	return ""
}
