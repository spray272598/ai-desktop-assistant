package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/repository"
)

// CoreMemoryProvider 注入长期记忆到 Prompt 上下文
type CoreMemoryProvider struct {
	repo repository.ICoreMemoryRepository
}

func NewCoreMemoryProvider(repo repository.ICoreMemoryRepository) *CoreMemoryProvider {
	return &CoreMemoryProvider{repo: repo}
}

func (p *CoreMemoryProvider) Name() string  { return "core_memory" }
func (p *CoreMemoryProvider) Order() int    { return 50 }
func (p *CoreMemoryProvider) Enabled() bool { return p.repo != nil }

func (p *CoreMemoryProvider) Provide(_, userID, _ string, _ []map[string]interface{}) map[string]interface{} {
	if p.repo == nil || userID == "" {
		return nil
	}
	items, err := p.repo.ListByUser(context.Background(), userID, 10)
	if err != nil || len(items) == 0 {
		return nil
	}
	var sb strings.Builder
	for _, it := range items {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", it.Category, it.Content))
	}
	return map[string]interface{}{
		"coreMemories": sb.String(),
	}
}
