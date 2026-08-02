package engine

import "time"

// EventType 事件类型
type EventType string

const (
	EventThought    EventType = "thought"
	EventToolCall   EventType = "tool_call"
	EventToolResult EventType = "tool_result"
	EventAnswer     EventType = "answer"
	EventError      EventType = "error"
	EventComplete   EventType = "complete"
	EventIntent     EventType = "intent"
)

// AgentEvent 引擎事件（SSE 推送）
type AgentEvent struct {
	Type      EventType   `json:"type"`
	SubType   string      `json:"subType,omitempty"`
	Step      int         `json:"step,omitempty"`
	Content   string      `json:"content,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Completed bool        `json:"completed,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

func NewEvent(typ EventType, step int, content string) *AgentEvent {
	return &AgentEvent{
		Type:      typ,
		Step:      step,
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}
}

// AgentResult 运行结果
type AgentResult struct {
	SessionID string
	Response  string
	Intent    string
	Steps     int
	TokenUsed int
	ToolCalls int
}
