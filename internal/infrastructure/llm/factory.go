package llm

import (
	"log"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/port"
	"github.com/ai-desktop/assistant/internal/infrastructure/config"
)

// NewFromConfig 根据配置创建 LLM 网关
func NewFromConfig(cfg *config.Config) port.ILLMPort {
	if cfg == nil || cfg.LLM.UseMock || cfg.LLM.APIKey == "" {
		log.Println("[llm] using MockGateway (set LLM_API_KEY for real model)")
		return NewMockGateway()
	}
	log.Printf("[llm] using OpenAI-compatible gateway model=%s base=%s\n", cfg.LLM.Model, cfg.LLM.APIBase)
	return NewOpenAIGateway(cfg.LLM.APIKey, cfg.LLM.APIBase, cfg.LLM.Model)
}
