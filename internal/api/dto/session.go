package dto

type CreateSessionRequest struct {
	AgentID string `json:"agentId"`
	UserID  string `json:"userId"`
	Title   string `json:"title,omitempty"`
}

type CreateSessionResponse struct {
	SessionID string `json:"sessionId"`
}

type SessionInfo struct {
	SessionID    string `json:"sessionId"`
	AgentID      string `json:"agentId"`
	UserID       string `json:"userId"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	MessageCount int    `json:"messageCount"`
	CreatedAt    string `json:"createdAt"`
}
