package repository

import (
	"context"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
)

// IMessageRepository 消息仓储
type IMessageRepository interface {
	Save(ctx context.Context, msg *entity.MessageEntity) error
	SaveBatch(ctx context.Context, msgs []*entity.MessageEntity) error
	ListBySession(ctx context.Context, sessionID string, limit int) ([]*entity.MessageEntity, error)
	ListAsMaps(ctx context.Context, sessionID string, limit int) ([]map[string]interface{}, error)
	DeleteBySession(ctx context.Context, sessionID string) error
	CountBySession(ctx context.Context, sessionID string) (int, error)
}
