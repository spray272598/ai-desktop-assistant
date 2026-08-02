package port

import "context"

// IMCPPort MCP 网关端口
type IMCPPort interface {
	ListTools(ctx context.Context) ([]map[string]string, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error)
}
