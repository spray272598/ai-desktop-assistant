package port

import (
	"context"

	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

// IMCPClient 单个 MCP 服务客户端
type IMCPClient interface {
	Name() string
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]entity.ToolDef, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error)
	Close() error
}

// IMCPManager 管理多个 MCP 客户端
type IMCPManager interface {
	Start(ctx context.Context) error
	ListTools(ctx context.Context) ([]entity.ToolDef, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error)
	Close() error
}
