package bootstrap

import (
	"fmt"
	"log"

	"github.com/ai-desktop/assistant/internal/application"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
	"github.com/ai-desktop/assistant/internal/domain/agent/service"
	contextsvc "github.com/ai-desktop/assistant/internal/domain/agent/service/context"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/context/provider"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/engine"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/intent"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/prompt"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/tools"
	desktopService "github.com/ai-desktop/assistant/internal/domain/desktop/service"
	"github.com/ai-desktop/assistant/internal/infrastructure/config"
	"github.com/ai-desktop/assistant/internal/infrastructure/llm"
	infraRepo "github.com/ai-desktop/assistant/internal/infrastructure/repository"
)

// App 组装后的应用根
type App struct {
	Config  *config.Config
	AgentApp *application.AgentApp
	Engine  *engine.AgentEngine
	Tools   *service.ToolRegistry
}

// Build 根据配置组装全部依赖
func Build(cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// 仓储
	sessionRepo := infraRepo.NewMemorySessionRepository()
	messageRepo := infraRepo.NewMemoryMessageRepository()

	// LLM
	llmPort := llm.NewFromConfig(cfg)

	// 意图
	tracker := intent.NewContextTracker()
	intentService := intent.NewIntentService(intent.NewRuleClassifier(), llmPort, tracker)

	// 提示词 / 里程碑
	promptService := prompt.NewPromptService()
	milestoneTracker := prompt.NewMilestoneTracker(100)

	// 上下文 providers
	envProvider := provider.NewEnvProvider(cfg.Desktop.Workspace)
	taskProvider := provider.NewTaskProvider()
	milestoneProvider := provider.NewMilestoneProvider(milestoneTracker)
	toolResultProvider := provider.NewToolResultProvider()
	chatContext := contextsvc.NewChatContextService(
		[]provider.ContextProvider{envProvider, taskProvider, milestoneProvider, toolResultProvider},
		toolResultProvider,
		taskProvider,
		cfg.Agent.TokenBudget,
	)

	// 桌面服务
	fileService := desktopService.NewFileService(cfg.Tools.File.BaseDir)
	cmdService := desktopService.NewCommandServiceWithPolicy(
		cfg.Tools.Command.MaxTimeout,
		cfg.Tools.Command.DenyList,
	)
	shotService := desktopService.NewScreenshotService(cfg.Desktop.ScreenshotDir)

	// 工具注册
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

	// Agent 实体
	agent := entity.NewAgentEntity(
		"desktop-agent",
		cfg.Agent.Name,
		"桌面 AI 助手，支持文件/命令/截图",
		"你是一个强大的桌面AI助手。",
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

	log.Printf("[bootstrap] agent=%s maxSteps=%d tools=%d workspace=%s\n",
		cfg.Agent.Name, cfg.Agent.MaxSteps, len(toolRegistry.ListTools()), cfg.Desktop.Workspace)

	return &App{
		Config:   cfg,
		AgentApp: agentApp,
		Engine:   eng,
		Tools:    toolRegistry,
	}, nil
}
