package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ai-desktop/assistant/internal/api/dto"
	"github.com/ai-desktop/assistant/internal/application"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/engine"
)

// DesktopApp 桌面客户端触发适配
type DesktopApp struct {
	agentApp *application.AgentApp
}

func NewDesktopApp(agentApp *application.AgentApp) *DesktopApp {
	return &DesktopApp{agentApp: agentApp}
}

func (d *DesktopApp) CreateSession(_ context.Context, agentID string) (string, error) {
	resp := d.agentApp.CreateSession(dto.CreateSessionRequest{
		AgentID: agentID,
		UserID:  "desktop-user",
		Title:   fmt.Sprintf("会话-%d", time.Now().Unix()),
	})
	return resp.SessionID, nil
}

func (d *DesktopApp) Chat(_ context.Context, sessionID string, message string) (string, error) {
	resp, err := d.agentApp.Chat(dto.ChatRequest{
		SessionID: sessionID,
		Message:   message,
		AgentID:   "desktop-agent",
		UserID:    "desktop-user",
	})
	if err != nil {
		return "", err
	}
	result := map[string]interface{}{
		"response":  resp.Response,
		"intent":    resp.Intent,
		"steps":     resp.Steps,
		"toolCalls": resp.ToolCalls,
	}
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonBytes), nil
}

func (d *DesktopApp) ChatStream(_ context.Context, sessionID string, message string, onEvent func(event *engine.AgentEvent)) error {
	eventChan, err := d.agentApp.ChatStream(dto.ChatRequest{
		SessionID: sessionID,
		Message:   message,
		AgentID:   "desktop-agent",
		UserID:    "desktop-user",
	})
	if err != nil {
		return err
	}

	for event := range eventChan {
		agentEvent := &engine.AgentEvent{
			Type:      engine.EventType(event.Type),
			SubType:   event.SubType,
			Step:      event.Step,
			Content:   event.Content,
			Data:      event.Data,
			Completed: event.Completed,
			Timestamp: event.Timestamp,
		}
		if onEvent != nil {
			onEvent(agentEvent)
		}
	}
	return nil
}

func (d *DesktopApp) GetSessionHistory(_ context.Context, sessionID string) (string, error) {
	info := d.agentApp.GetSessionInfo(sessionID)
	if info == nil {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}
	jsonBytes, _ := json.MarshalIndent(info, "", "  ")
	return string(jsonBytes), nil
}
