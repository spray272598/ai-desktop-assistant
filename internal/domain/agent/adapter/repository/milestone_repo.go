package repository

import (
	"context"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

// IMilestoneRepository 里程碑仓储
type IMilestoneRepository interface {
	Save(ctx context.Context, sessionID string, m *valobj.MilestoneVO) error
	ListBySession(ctx context.Context, sessionID string, limit int) ([]*valobj.MilestoneVO, error)
	DeleteBySession(ctx context.Context, sessionID string) error
}

// ICoreMemoryRepository 长期记忆
type ICoreMemoryRepository interface {
	Save(ctx context.Context, userID, sessionID, category, content, source string) error
	ListByUser(ctx context.Context, userID string, limit int) ([]CoreMemoryItem, error)
}

type CoreMemoryItem struct {
	ID       int64
	UserID   string
	Category string
	Content  string
	Source   string
}
