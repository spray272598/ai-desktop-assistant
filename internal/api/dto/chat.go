package dto

type ChatRequest struct {
	AgentID     string                 `json:"agentId"`
	UserID      string                 `json:"userId"`
	SessionID   string                 `json:"sessionId,omitempty"`
	Message     string                 `json:"message"`
	AutoApprove bool                   `json:"autoApprove,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

type ChatResponse struct {
	SessionID         string      `json:"sessionId"`
	Response          string      `json:"response"`
	Intent            string      `json:"intent"`
	ToolCalls         int         `json:"toolCalls"`
	Steps             int         `json:"steps"`
	TokenUsed         int         `json:"tokenUsed"`
	TaskPlan          interface{} `json:"taskPlan,omitempty"`
	NeedPermission    bool        `json:"needPermission,omitempty"`
	PendingPermission interface{} `json:"pendingPermission,omitempty"`
}

type ChatStreamEvent struct {
	Type      string      `json:"type"`
	SubType   string      `json:"subType,omitempty"`
	Step      int         `json:"step,omitempty"`
	Content   string      `json:"content,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Completed bool        `json:"completed"`
	Timestamp int64       `json:"timestamp"`
}

type PermissionApproveRequest struct {
	ID    string `json:"id"`
	Scope string `json:"scope"` // once | session
}

type MCPServerRequest struct {
	Name       string            `json:"name"`
	Transport  string            `json:"transport"`
	Command    string            `json:"command"`
	Args       []string          `json:"args"`
	Env        map[string]string `json:"env"`
	URL        string            `json:"url"`
	Enabled    *bool             `json:"enabled"`
	TimeoutSec int               `json:"timeoutSec"`
}
