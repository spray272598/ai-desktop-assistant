package entity

import (
	"sync"
	"time"
)

type SessionEntity struct {
	ID           string
	AgentID      string
	UserID       string
	Title        string
	Status       string
	MessageCount int
	TokenUsed    int
	WorkingDir   string
	Metadata     map[string]string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	mu           sync.RWMutex
}

func NewSessionEntity(id, agentID, userID, title string) *SessionEntity {
	now := time.Now()
	return &SessionEntity{
		ID:        id,
		AgentID:   agentID,
		UserID:    userID,
		Title:     title,
		Status:    "ACTIVE",
		Metadata:  make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (s *SessionEntity) AddMessage(tokenCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MessageCount++
	s.TokenUsed += tokenCount
	s.UpdatedAt = time.Now()
}

func (s *SessionEntity) Complete() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = "COMPLETED"
	s.UpdatedAt = time.Now()
}

func (s *SessionEntity) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status == "ACTIVE"
}

func (s *SessionEntity) SetMeta(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Metadata == nil {
		s.Metadata = make(map[string]string)
	}
	s.Metadata[key] = value
	s.UpdatedAt = time.Now()
}

func (s *SessionEntity) GetMeta(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Metadata == nil {
		return ""
	}
	return s.Metadata[key]
}

func (s *SessionEntity) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdatedAt = time.Now()
}
