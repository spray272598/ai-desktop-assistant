package tools

import (
	"context"
	"fmt"

	"github.com/ai-desktop/assistant/internal/domain/mcp/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

// MCPTool 将 MCP 工具适配为 Agent ITool
type MCPTool struct {
	def     entity.ToolDef
	manager port.IMCPManager
}

func NewMCPTool(def entity.ToolDef, manager port.IMCPManager) *MCPTool {
	return &MCPTool{def: def, manager: manager}
}

func (t *MCPTool) Name() string { return t.def.Name }

func (t *MCPTool) Description() string {
	desc := t.def.Description
	if desc == "" {
		desc = "MCP tool"
	}
	if t.def.ServerName != "" {
		return fmt.Sprintf("[MCP:%s] %s", t.def.ServerName, desc)
	}
	return desc
}

func (t *MCPTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.manager == nil {
		return "", fmt.Errorf("mcp manager not available")
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	result, err := t.manager.CallTool(ctx, t.def.Name, args)
	if err != nil {
		return fmt.Sprintf("MCP 工具 %s 失败: %v", t.def.Name, err), nil
	}
	return result, nil
}
