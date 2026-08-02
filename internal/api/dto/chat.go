package dto

type ChatRequest struct {
	AgentID   string                 `json:"agentId"`
	UserID    string                 `json:"userId"`
	SessionID string                 `json:"sessionId,omitempty"`
	Message   string                 `json:"message"`
	Params    map[string]interface{} `json:"params,omitempty"`
}

type ChatResponse struct {
	SessionID string `json:"sessionId"`
	Response  string `json:"response"`
	Intent    string `json:"intent"`
	ToolCalls int    `json:"toolCalls"`
	Steps     int    `json:"steps"`
	TokenUsed int    `json:"tokenUsed"`
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
