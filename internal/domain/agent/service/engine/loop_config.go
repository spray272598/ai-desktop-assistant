package engine

// AgentMode 执行模式
type AgentMode string

const (
	ModeChat  AgentMode = "chat"
	ModeAgent AgentMode = "agent"
)

// LoopConfig Agent 循环配置（对标 walicode AgentLoopConfig）
type LoopConfig struct {
	Mode                       AgentMode
	MaxRounds                  int
	MaxToolCallsPerRound       int
	MaxTotalToolCalls          int
	MaxAiRetries               int
	MaxToolRetries             int
	RetryDelayBaseMs           int64
	MaxTokenBudget             int
	AutoContinue               bool
	DiminishingReturnsThreshold int
}

func DefaultLoopConfig() LoopConfig {
	return LoopConfig{
		Mode:                        ModeAgent,
		MaxRounds:                   15,
		MaxToolCallsPerRound:        3,
		MaxTotalToolCalls:           50,
		MaxAiRetries:                2,
		MaxToolRetries:              1,
		RetryDelayBaseMs:            500,
		MaxTokenBudget:              16000,
		AutoContinue:                true,
		DiminishingReturnsThreshold: 2,
	}
}

// LoopConfigFromIntent 根据意图选择模式
func LoopConfigFromIntent(intent string, base LoopConfig) LoopConfig {
	cfg := base
	switch intent {
	case "CHAT", "UNKNOWN", "":
		cfg.Mode = ModeChat
		cfg.MaxRounds = min(cfg.MaxRounds, 5)
		cfg.MaxTotalToolCalls = min(cfg.MaxTotalToolCalls, 5)
	case "TASK_PLAN":
		cfg.Mode = ModeAgent
		if cfg.MaxRounds < 20 {
			cfg.MaxRounds = 20
		}
	default:
		cfg.Mode = ModeAgent
	}
	return cfg
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
