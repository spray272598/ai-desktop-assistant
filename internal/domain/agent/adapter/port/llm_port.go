package port

import "context"

// ChatMessage LLM 对话消息
type ChatMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ChatRequest LLM 请求
type ChatRequest struct {
	SystemPrompt string
	Messages     []ChatMessage
	Temperature  float64
	MaxTokens    int
}

// ChatResponse LLM 响应
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	PromptTokens int
	CompletionTokens int
	TotalTokens  int
	Raw          string
}

// ToolCall 模型请求的工具调用
type ToolCall struct {
	ID   string
	Name string
	Args map[string]interface{}
}

// ILLMPort LLM 网关端口
type ILLMPort interface {
	// Generate 生成回复；若模型以 JSON 工具调用格式返回，则解析到 ToolCalls
	Generate(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// ClassifyIntent 意图分类专用轻量调用
	ClassifyIntent(ctx context.Context, input string, contextHint string) (intent string, confidence float64, entities map[string]string, err error)
}
