package entity

// ServerConfig MCP 服务配置
type ServerConfig struct {
	Name       string
	Transport  string // stdio | sse | http
	Command    string
	Args       []string
	Env        map[string]string
	URL        string
	Enabled    bool
	TimeoutSec int
}

// ToolDef MCP 工具定义
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	ServerName  string
}
