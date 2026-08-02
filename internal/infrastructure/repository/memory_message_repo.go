package repository

import (
	"context"
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
)

type MemoryMessageRepository struct {
	mu       sync.RWMutex
	messages map[string][]*entity.MessageEntity // sessionID -> messages
}

func NewMemoryMessageRepository() *MemoryMessageRepository {
	return &MemoryMessageRepository{
		messages: make(map[string][]*entity.MessageEntity),
	}
}

func (r *MemoryMessageRepository) Save(_ context.Context, msg *entity.MessageEntity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *msg
	r.messages[msg.SessionID] = append(r.messages[msg.SessionID], &cp)
	return nil
}

func (r *MemoryMessageRepository) SaveBatch(_ context.Context, msgs []*entity.MessageEntity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, msg := range msgs {
		cp := *msg
		r.messages[msg.SessionID] = append(r.messages[msg.SessionID], &cp)
	}
	return nil
}

func (r *MemoryMessageRepository) ListBySession(_ context.Context, sessionID string, limit int) ([]*entity.MessageEntity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	msgs := r.messages[sessionID]
	if len(msgs) == 0 {
		return nil, nil
	}
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	out := make([]*entity.MessageEntity, len(msgs))
	for i, m := range msgs {
		cp := *m
		out[i] = &cp
	}
	return out, nil
}

func (r *MemoryMessageRepository) ListAsMaps(ctx context.Context, sessionID string, limit int) ([]map[string]interface{}, error) {
	msgs, err := r.ListBySession(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.ToMap())
	}
	return out, nil
}

func (r *MemoryMessageRepository) DeleteBySession(_ context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.messages, sessionID)
	return nil
}

func (r *MemoryMessageRepository) CountBySession(_ context.Context, sessionID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.messages[sessionID]), nil
}
