package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ai-desktop/assistant/internal/api/dto"
	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/repository"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/engine"
	"github.com/ai-desktop/assistant/internal/types/common"
)

// AgentApp 应用服务：会话 + 对话用例
type AgentApp struct {
	engine      *engine.AgentEngine
	sessionRepo repository.ISessionRepository
	messageRepo repository.IMessageRepository
	defaultAgentID string
	timeoutSec  int
	workDir     string
}

func NewAgentApp(
	eng *engine.AgentEngine,
	sessionRepo repository.ISessionRepository,
	messageRepo repository.IMessageRepository,
	defaultAgentID string,
	timeoutSec int,
	workDir string,
) *AgentApp {
	if timeoutSec <= 0 {
		timeoutSec = common.DefaultTimeout
	}
	return &AgentApp{
		engine:         eng,
		sessionRepo:    sessionRepo,
		messageRepo:    messageRepo,
		defaultAgentID: defaultAgentID,
		timeoutSec:     timeoutSec,
		workDir:        workDir,
	}
}

func (app *AgentApp) CreateSession(req dto.CreateSessionRequest) *dto.CreateSessionResponse {
	agentID := req.AgentID
	if agentID == "" {
		agentID = app.defaultAgentID
	}
	userID := req.UserID
	if userID == "" {
		userID = "anonymous"
	}
	title := req.Title
	if title == "" {
		title = "新会话"
	}

	session := entity.NewSessionEntity(generateSessionID(), agentID, userID, title)
	session.WorkingDir = app.workDir
	_ = app.sessionRepo.Save(context.Background(), session)

	return &dto.CreateSessionResponse{SessionID: session.ID}
}

func (app *AgentApp) Chat(req dto.ChatRequest) (*dto.ChatResponse, error) {
	session, err := app.resolveSession(req)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(app.timeoutSec)*time.Second)
	defer cancel()

	result, err := app.engine.Run(ctx, session, req.Message, nil)
	if err != nil && result == nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("empty result")
	}

	return &dto.ChatResponse{
		SessionID: result.SessionID,
		Response:  result.Response,
		Intent:    result.Intent,
		ToolCalls: result.ToolCalls,
		Steps:     result.Steps,
		TokenUsed: result.TokenUsed,
	}, nil
}

// ChatStream 流式对话，返回事件 channel（调用方负责读取直到 completed）
func (app *AgentApp) ChatStream(req dto.ChatRequest) (<-chan dto.ChatStreamEvent, error) {
	session, err := app.resolveSession(req)
	if err != nil {
		return nil, err
	}

	eventCh := make(chan *engine.AgentEvent, 64)
	out := make(chan dto.ChatStreamEvent, 64)

	go func() {
		defer close(out)
		for ev := range eventCh {
			out <- dto.ChatStreamEvent{
				Type:      string(ev.Type),
				SubType:   ev.SubType,
				Step:      ev.Step,
				Content:   ev.Content,
				Data:      ev.Data,
				Completed: ev.Completed,
				Timestamp: ev.Timestamp,
			}
			if ev.Completed {
				// 排空剩余
				for remaining := range eventCh {
					out <- dto.ChatStreamEvent{
						Type: string(remaining.Type), SubType: remaining.SubType,
						Step: remaining.Step, Content: remaining.Content,
						Data: remaining.Data, Completed: remaining.Completed,
						Timestamp: remaining.Timestamp,
					}
				}
				return
			}
		}
	}()

	go func() {
		defer close(eventCh)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(app.timeoutSec)*time.Second)
		defer cancel()
		_, err := app.engine.Run(ctx, session, req.Message, eventCh)
		if err != nil {
			select {
			case eventCh <- &engine.AgentEvent{
				Type: engine.EventError, Content: err.Error(),
				Completed: true, Timestamp: time.Now().UnixMilli(),
			}:
			default:
			}
		}
	}()

	return out, nil
}

func (app *AgentApp) GetSessionInfo(sessionID string) *dto.SessionInfo {
	session, err := app.sessionRepo.FindByID(context.Background(), sessionID)
	if err != nil || session == nil {
		return nil
	}
	return toSessionInfo(session)
}

func (app *AgentApp) ListSessionsByUser(userID string) ([]*dto.SessionInfo, error) {
	list, err := app.sessionRepo.FindByUser(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.SessionInfo, 0, len(list))
	for _, s := range list {
		out = append(out, toSessionInfo(s))
	}
	return out, nil
}

func toSessionInfo(session *entity.SessionEntity) *dto.SessionInfo {
	if session == nil {
		return nil
	}
	return &dto.SessionInfo{
		SessionID:    session.ID,
		AgentID:      session.AgentID,
		UserID:       session.UserID,
		Title:        session.Title,
		Status:       session.Status,
		MessageCount: session.MessageCount,
		CreatedAt:    session.CreatedAt.Format(time.RFC3339),
	}
}

func (app *AgentApp) ListMessages(sessionID string, limit int) ([]map[string]interface{}, error) {
	return app.messageRepo.ListAsMaps(context.Background(), sessionID, limit)
}

func (app *AgentApp) resolveSession(req dto.ChatRequest) (*entity.SessionEntity, error) {
	if req.SessionID != "" {
		s, err := app.sessionRepo.FindByID(context.Background(), req.SessionID)
		if err != nil {
			return nil, err
		}
		if s != nil && s.IsActive() {
			return s, nil
		}
	}
	// 自动创建
	agentID := req.AgentID
	if agentID == "" {
		agentID = app.defaultAgentID
	}
	userID := req.UserID
	if userID == "" {
		userID = "anonymous"
	}
	session := entity.NewSessionEntity(generateSessionID(), agentID, userID, "自动会话")
	session.WorkingDir = app.workDir
	_ = app.sessionRepo.Save(context.Background(), session)
	return session, nil
}

func generateSessionID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("sess-%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}
