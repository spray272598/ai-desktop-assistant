package entity

import "time"

type AgentEntity struct {
	ID           string
	Name         string
	Description  string
	SystemPrompt string
	MaxSteps     int
	Tools        []*ToolConfig
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ToolConfig struct {
	ID          string
	Name        string
	Description string
	Enabled     bool
	Config      map[string]interface{}
}

func NewAgentEntity(id, name, description, systemPrompt string) *AgentEntity {
	return &AgentEntity{
		ID:           id,
		Name:         name,
		Description:  description,
		SystemPrompt: systemPrompt,
		MaxSteps:     15,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func (a *AgentEntity) AddTool(tool *ToolConfig) {
	a.Tools = append(a.Tools, tool)
	a.UpdatedAt = time.Now()
}

func (a *AgentEntity) RemoveTool(toolID string) {
	for i, t := range a.Tools {
		if t.ID == toolID {
			a.Tools = append(a.Tools[:i], a.Tools[i+1:]...)
			break
		}
	}
	a.UpdatedAt = time.Now()
}
