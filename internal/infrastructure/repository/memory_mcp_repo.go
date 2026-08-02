package repository

import (
	"context"
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

type MemoryMCPServerRepository struct {
	mu   sync.RWMutex
	data map[string]entity.ServerConfig
}

func NewMemoryMCPServerRepository() *MemoryMCPServerRepository {
	return &MemoryMCPServerRepository{data: make(map[string]entity.ServerConfig)}
}

func (r *MemoryMCPServerRepository) Save(_ context.Context, cfg *entity.ServerConfig) error {
	if cfg == nil {
		return nil
	}
	r.mu.Lock()
	r.data[cfg.Name] = *cfg
	r.mu.Unlock()
	return nil
}

func (r *MemoryMCPServerRepository) Delete(_ context.Context, name string) error {
	r.mu.Lock()
	delete(r.data, name)
	r.mu.Unlock()
	return nil
}

func (r *MemoryMCPServerRepository) FindByName(_ context.Context, name string) (*entity.ServerConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.data[name]
	if !ok {
		return nil, nil
	}
	cp := c
	return &cp, nil
}

func (r *MemoryMCPServerRepository) List(_ context.Context) ([]entity.ServerConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]entity.ServerConfig, 0, len(r.data))
	for _, c := range r.data {
		out = append(out, c)
	}
	return out, nil
}

func (r *MemoryMCPServerRepository) ListEnabled(ctx context.Context) ([]entity.ServerConfig, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []entity.ServerConfig
	for _, c := range all {
		if c.Enabled {
			out = append(out, c)
		}
	}
	return out, nil
}
