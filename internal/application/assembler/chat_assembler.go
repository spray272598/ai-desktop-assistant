package assembler

import (
	"github.com/ai-desktop/assistant/internal/api/dto"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/engine"
)

// ToChatResponse 引擎结果 → DTO
func ToChatResponse(r *engine.AgentResult) *dto.ChatResponse {
	if r == nil {
		return nil
	}
	return &dto.ChatResponse{
		SessionID: r.SessionID,
		Response:  r.Response,
		Intent:    r.Intent,
		ToolCalls: r.ToolCalls,
		Steps:     r.Steps,
		TokenUsed: r.TokenUsed,
	}
}

// ToStreamEvent 引擎事件 → DTO
func ToStreamEvent(ev *engine.AgentEvent) dto.ChatStreamEvent {
	if ev == nil {
		return dto.ChatStreamEvent{}
	}
	return dto.ChatStreamEvent{
		Type:      string(ev.Type),
		SubType:   ev.SubType,
		Step:      ev.Step,
		Content:   ev.Content,
		Data:      ev.Data,
		Completed: ev.Completed,
		Timestamp: ev.Timestamp,
	}
}
