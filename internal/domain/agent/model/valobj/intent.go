package valobj

type IntentResult struct {
	Intent      string            `json:"intent"`
	Confidence  float64           `json:"confidence"`
	Entities    map[string]string `json:"entities,omitempty"`
	RawResponse string            `json:"rawResponse,omitempty"`
	IsAction    bool              `json:"isAction"`
	Source      string            `json:"source,omitempty"` // rule | llm | cache | fallback
}

func NewIntentResult(intent string, confidence float64, entities map[string]string) *IntentResult {
	if entities == nil {
		entities = make(map[string]string)
	}
	return &IntentResult{
		Intent:     intent,
		Confidence: confidence,
		Entities:   entities,
		IsAction:   isActionIntent(intent),
	}
}

func isActionIntent(intent string) bool {
	actionIntents := map[string]bool{
		"READ_FILE": true, "WRITE_FILE": true, "LIST_FILES": true,
		"DELETE_FILE": true, "CREATE_DIR": true,
		"RUN_COMMAND": true, "RUN_SCRIPT": true, "START_APP": true,
		"SCREENSHOT": true, "ANALYZE_SCREEN": true,
		"OPEN_URL": true, "BROWSER_ACTION": true,
		"GET_CLIPBOARD": true, "SET_CLIPBOARD": true,
		"SYSTEM_INFO": true, "TASK_PLAN": true,
	}
	return actionIntents[intent]
}

func (r *IntentResult) HasHighConfidence(threshold float64) bool {
	return r != nil && r.Confidence >= threshold
}

func (r *IntentResult) GetEntity(key string) string {
	if r == nil || r.Entities == nil {
		return ""
	}
	return r.Entities[key]
}

// ConversationContextVO 会话级意图上下文（指代消解）
type ConversationContextVO struct {
	SessionID      string
	LastIntent     string
	LastEntities   map[string]string
	RecentIntents  []string
	EntityMemory   map[string]string
	LastUserMsg    string
	TurnCount      int
}

func NewConversationContext(sessionID string) *ConversationContextVO {
	return &ConversationContextVO{
		SessionID:    sessionID,
		LastEntities: make(map[string]string),
		RecentIntents: make([]string, 0),
		EntityMemory: make(map[string]string),
	}
}

func (c *ConversationContextVO) PutEntity(k, v string) {
	if c.EntityMemory == nil {
		c.EntityMemory = make(map[string]string)
	}
	if v != "" {
		c.EntityMemory[k] = v
	}
}

func (c *ConversationContextVO) UpdateFromIntent(result *IntentResult, userMsg string) {
	if result == nil {
		return
	}
	c.LastIntent = result.Intent
	c.LastUserMsg = userMsg
	c.TurnCount++
	c.RecentIntents = append(c.RecentIntents, result.Intent)
	if len(c.RecentIntents) > 10 {
		c.RecentIntents = c.RecentIntents[len(c.RecentIntents)-10:]
	}
	if result.Entities != nil {
		c.LastEntities = result.Entities
		for k, v := range result.Entities {
			c.PutEntity(k, v)
		}
	}
}
