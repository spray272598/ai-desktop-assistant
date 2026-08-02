package repository

import (
	"context"

	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

// IMCPServerRepository MCP 服务配置仓储
type IMCPServerRepository interface {
	Save(ctx context.Context, cfg *entity.ServerConfig) error
	Delete(ctx context.Context, name string) error
	FindByName(ctx context.Context, name string) (*entity.ServerConfig, error)
	List(ctx context.Context) ([]entity.ServerConfig, error)
	ListEnabled(ctx context.Context) ([]entity.ServerConfig, error)
}
