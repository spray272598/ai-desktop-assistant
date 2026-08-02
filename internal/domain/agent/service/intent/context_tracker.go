package intent

import (
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

// ContextTracker 会话级意图上下文追踪。
// 所有对 ConversationContextVO 的读写均在 mu 保护下完成，避免 Get/Update/Hint 竞态。
type ContextTracker struct {
	mu       sync.RWMutex
	contexts map[string]*valobj.ConversationContextVO
}

func NewContextTracker() *ContextTracker {
	return &ContextTracker{
		contexts: make(map[string]*valobj.ConversationContextVO),
	}
}

// snapshot 返回上下文的深拷贝，供只读使用，避免外部持有内部指针。
func snapshot(src *valobj.ConversationContextVO) *valobj.ConversationContextVO {
	if src == nil {
		return nil
	}
	cp := &valobj.ConversationContextVO{
		SessionID:     src.SessionID,
		LastIntent:    src.LastIntent,
		LastUserMsg:   src.LastUserMsg,
		TurnCount:     src.TurnCount,
		LastEntities:  copyStrMap(src.LastEntities),
		RecentIntents: append([]string(nil), src.RecentIntents...),
		EntityMemory:  copyStrMap(src.EntityMemory),
	}
	return cp
}

func copyStrMap(src map[string]string) map[string]string {
	if src == nil {
		return make(map[string]string)
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// GetContext 返回会话上下文快照（拷贝），并发安全。
func (t *ContextTracker) GetContext(sessionID string) *valobj.ConversationContextVO {
	t.mu.Lock()
	defer t.mu.Unlock()
	ctx, ok := t.contexts[sessionID]
	if !ok {
		ctx = valobj.NewConversationContext(sessionID)
		t.contexts[sessionID] = ctx
	}
	return snapshot(ctx)
}

// UpdateContext 在写锁下更新会话意图上下文。
func (t *ContextTracker) UpdateContext(sessionID string, result *valobj.IntentResult, userMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ctx, ok := t.contexts[sessionID]
	if !ok {
		ctx = valobj.NewConversationContext(sessionID)
		t.contexts[sessionID] = ctx
	}
	ctx.UpdateFromIntent(result, userMsg)
}

func (t *ContextTracker) Clear(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.contexts, sessionID)
}

// Hint 在读锁下生成上下文字符串，避免读到半更新状态。
func (t *ContextTracker) Hint(sessionID string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ctx, ok := t.contexts[sessionID]
	if !ok || ctx == nil || ctx.LastIntent == "" {
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
