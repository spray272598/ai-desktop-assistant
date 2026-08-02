package repository

import (
	"context"
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
)

type MemorySessionRepository struct {
	mu       sync.RWMutex
	sessions map[string]*entity.SessionEntity
}

func NewMemorySessionRepository() *MemorySessionRepository {
	return &MemorySessionRepository{
		sessions: make(map[string]*entity.SessionEntity),
	}
}

func (r *MemorySessionRepository) Save(_ context.Context, session *entity.SessionEntity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// shallow copy to avoid external mutation surprises
	cp := *session
	r.sessions[session.ID] = &cp
	return nil
}

func (r *MemorySessionRepository) FindByID(_ context.Context, id string) (*entity.SessionEntity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	if !ok {
		return nil, nil
	}
	cp := *s
	return &cp, nil
}

func (r *MemorySessionRepository) FindByUser(_ context.Context, userID string) ([]*entity.SessionEntity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*entity.SessionEntity
	for _, s := range r.sessions {
		if s.UserID == userID {
			cp := *s
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (r *MemorySessionRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
	return nil
}

func (r *MemorySessionRepository) ListActive(_ context.Context, limit int) ([]*entity.SessionEntity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		limit = 50 // 与 common.DefaultListActiveLimit 对齐；memory 包避免循环依赖
	}
	var list []*entity.SessionEntity
	for _, s := range r.sessions {
		if s.Status == "ACTIVE" {
			cp := *s
			list = append(list, &cp)
			if len(list) >= limit {
				break
			}
		}
	}
	return list, nil
}
