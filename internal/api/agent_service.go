package api

import "github.com/ai-desktop/assistant/internal/api/dto"

type IAgentService interface {
	CreateSession(req dto.CreateSessionRequest) *dto.CreateSessionResponse
	Chat(req dto.ChatRequest) (*dto.ChatResponse, error)
	ChatStream(req dto.ChatRequest) (<-chan dto.ChatStreamEvent, error)
}
