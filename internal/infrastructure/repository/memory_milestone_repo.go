package repository

import (
	"context"
	"sync"

	domrepo "github.com/ai-desktop/assistant/internal/domain/agent/adapter/repository"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

type MemoryMilestoneRepository struct {
	mu   sync.RWMutex
	data map[string][]*valobj.MilestoneVO
}

func NewMemoryMilestoneRepository() *MemoryMilestoneRepository {
	return &MemoryMilestoneRepository{data: make(map[string][]*valobj.MilestoneVO)}
}

func (r *MemoryMilestoneRepository) Save(_ context.Context, sessionID string, m *valobj.MilestoneVO) error {
	if m == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *m
	r.data[sessionID] = append(r.data[sessionID], &cp)
	if len(r.data[sessionID]) > 100 {
		r.data[sessionID] = r.data[sessionID][len(r.data[sessionID])-100:]
	}
	return nil
}

func (r *MemoryMilestoneRepository) ListBySession(_ context.Context, sessionID string, limit int) ([]*valobj.MilestoneVO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	src := r.data[sessionID]
	if limit > 0 && len(src) > limit {
		src = src[len(src)-limit:]
	}
	out := make([]*valobj.MilestoneVO, len(src))
	for i, m := range src {
		cp := *m
		out[i] = &cp
	}
	return out, nil
}

func (r *MemoryMilestoneRepository) DeleteBySession(_ context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, sessionID)
	return nil
}

type MemoryCoreMemoryRepository struct {
	mu   sync.RWMutex
	data []domrepo.CoreMemoryItem
	seq  int64
}

func NewMemoryCoreMemoryRepository() *MemoryCoreMemoryRepository {
	return &MemoryCoreMemoryRepository{data: make([]domrepo.CoreMemoryItem, 0)}
}

func (r *MemoryCoreMemoryRepository) Save(_ context.Context, userID, sessionID, category, content, source string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	r.data = append(r.data, domrepo.CoreMemoryItem{
		ID: r.seq, UserID: userID, Category: category, Content: content, Source: source,
	})
	return nil
}

func (r *MemoryCoreMemoryRepository) ListByUser(_ context.Context, userID string, limit int) ([]domrepo.CoreMemoryItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []domrepo.CoreMemoryItem
	for i := len(r.data) - 1; i >= 0; i-- {
		if r.data[i].UserID == userID {
			out = append(out, r.data[i])
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
