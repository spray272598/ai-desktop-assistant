package repository

import (
	"context"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
)

// ISessionRepository 会话仓储
type ISessionRepository interface {
	Save(ctx context.Context, session *entity.SessionEntity) error
	FindByID(ctx context.Context, id string) (*entity.SessionEntity, error)
	FindByUser(ctx context.Context, userID string) ([]*entity.SessionEntity, error)
	Delete(ctx context.Context, id string) error
	ListActive(ctx context.Context, limit int) ([]*entity.SessionEntity, error)
}
