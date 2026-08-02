package intent

import (
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

// ContextTracker 会话级意图上下文追踪
type ContextTracker struct {
	mu       sync.RWMutex
	contexts map[string]*valobj.ConversationContextVO
}

func NewContextTracker() *ContextTracker {
	return &ContextTracker{
		contexts: make(map[string]*valobj.ConversationContextVO),
	}
}

func (t *ContextTracker) GetContext(sessionID string) *valobj.ConversationContextVO {
	t.mu.RLock()
	ctx, ok := t.contexts[sessionID]
	t.mu.RUnlock()
	if ok {
		return ctx
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if ctx, ok = t.contexts[sessionID]; ok {
		return ctx
	}
	ctx = valobj.NewConversationContext(sessionID)
	t.contexts[sessionID] = ctx
	return ctx
}

func (t *ContextTracker) UpdateContext(sessionID string, result *valobj.IntentResult, userMsg string) {
	ctx := t.GetContext(sessionID)
	t.mu.Lock()
	defer t.mu.Unlock()
	ctx.UpdateFromIntent(result, userMsg)
}

func (t *ContextTracker) Clear(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.contexts, sessionID)
}

func (t *ContextTracker) Hint(sessionID string) string {
	ctx := t.GetContext(sessionID)
	if ctx.LastIntent == "" {
		return ""
	}
	hint := "上轮意图: " + ctx.LastIntent
	if path := ctx.EntityMemory["path"]; path != "" {
		hint += "; 已知路径: " + path
	}
	if cmd := ctx.EntityMemory["command"]; cmd != "" {
		hint += "; 已知命令: " + cmd
	}
	return hint
}
