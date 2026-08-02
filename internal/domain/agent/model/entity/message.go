package entity

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/ai-desktop/assistant/internal/types/enums"
)

type MessageEntity struct {
	ID         string
	SessionID  string
	Role       enums.MessageRole
	Content    string
	ToolName   string
	ToolCallID string
	Step       int
	TokenCount int
	Priority   enums.Priority
	CreatedAt  time.Time
}

func NewUserMessage(sessionID, content string) *MessageEntity {
	return &MessageEntity{
		ID:        generateID(),
		SessionID: sessionID,
		Role:      enums.RoleUser,
		Content:   content,
		Priority:  enums.PriorityHigh,
		CreatedAt: time.Now(),
	}
}

func NewAssistantMessage(sessionID, content string, step int) *MessageEntity {
	return &MessageEntity{
		ID:        generateID(),
		SessionID: sessionID,
		Role:      enums.RoleAssistant,
		Content:   content,
		Step:      step,
		Priority:  enums.PriorityHigh,
		CreatedAt: time.Now(),
	}
}

func NewToolMessage(sessionID, toolName, toolCallID, content string, step int) *MessageEntity {
	return &MessageEntity{
		ID:         generateID(),
		SessionID:  sessionID,
		Role:       enums.RoleTool,
		Content:    content,
		ToolName:   toolName,
		ToolCallID: toolCallID,
		Step:       step,
		Priority:   enums.PriorityMedium,
		CreatedAt:  time.Now(),
	}
}

func NewSystemMessage(sessionID, content string) *MessageEntity {
	return &MessageEntity{
		ID:        generateID(),
		SessionID: sessionID,
		Role:      enums.RoleSystem,
		Content:   content,
		Priority:  enums.PriorityCritical,
		CreatedAt: time.Now(),
	}
}

func (m *MessageEntity) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":          m.ID,
		"sessionId":   m.SessionID,
		"role":        string(m.Role),
		"content":     m.Content,
		"toolName":    m.ToolName,
		"toolCallId":  m.ToolCallID,
		"step":        m.Step,
		"tokenCount":  m.TokenCount,
		"priority":    int(m.Priority),
		"createdAt":   m.CreatedAt.Format(time.RFC3339),
	}
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(b)
}
