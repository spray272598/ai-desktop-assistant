package intent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

type IIntentService interface {
	// Recognize 会话级意图识别（规则 → LLM 降级）
	Recognize(ctx context.Context, sessionID, userID, input string) (*valobj.IntentResult, error)
	RecognizeWithLLM(ctx context.Context, sessionID, input string) (*valobj.IntentResult, error)
}

type cacheEntry struct {
	result    *valobj.IntentResult
	expireAt  time.Time
}

type IntentService struct {
	ruleClassifier *RuleClassifier
	llmClassifier  *LLMClassifier
	tracker        *ContextTracker
	cache          map[string]cacheEntry
	cacheMu        sync.RWMutex
	cacheTTL       time.Duration
	maxCache       int
}

func NewIntentService(rule *RuleClassifier, llm port.ILLMPort, tracker *ContextTracker) *IntentService {
	var llmCls *LLMClassifier
	if llm != nil {
		llmCls = NewLLMClassifier(llm)
	}
	if tracker == nil {
		tracker = NewContextTracker()
	}
	return &IntentService{
		ruleClassifier: rule,
		llmClassifier:  llmCls,
		tracker:        tracker,
		cache:          make(map[string]cacheEntry),
		cacheTTL:       5 * time.Minute,
		maxCache:       200,
	}
}

func (s *IntentService) Recognize(ctx context.Context, sessionID, userID, input string) (*valobj.IntentResult, error) {
	_ = userID
	key := sessionID + ":" + hashKey(input)
	if cached := s.getCache(key); cached != nil {
		cp := *cached
		cp.Source = "cache"
		return &cp, nil
	}

	conv := s.tracker.GetContext(sessionID)
	ruleResult := s.ruleClassifier.Classify(input, conv)
	if ruleResult.HasHighConfidence(0.8) {
		s.tracker.UpdateContext(sessionID, ruleResult, input)
		s.putCache(key, ruleResult)
		return ruleResult, nil
	}

	// LLM 降级（安全：nil 时用规则结果）
	var final *valobj.IntentResult
	if s.llmClassifier != nil {
		hint := s.tracker.Hint(sessionID)
		llmResult := s.llmClassifier.Classify(ctx, input, hint)
		if llmResult.HasHighConfidence(0.5) {
			final = llmResult
		} else {
			final = ruleResult
			if final.Confidence < llmResult.Confidence {
				final = llmResult
			}
		}
	} else {
		final = ruleResult
		if final.Source == "" {
			final.Source = "rule-only"
		}
	}

	s.tracker.UpdateContext(sessionID, final, input)
	s.putCache(key, final)
	return final, nil
}

func (s *IntentService) RecognizeWithLLM(ctx context.Context, sessionID, input string) (*valobj.IntentResult, error) {
	if s.llmClassifier == nil {
		return s.ruleClassifier.Classify(input, s.tracker.GetContext(sessionID)), nil
	}
	hint := s.tracker.Hint(sessionID)
	result := s.llmClassifier.Classify(ctx, input, hint)
	s.tracker.UpdateContext(sessionID, result, input)
	return result, nil
}

func (s *IntentService) Tracker() *ContextTracker {
	return s.tracker
}

func (s *IntentService) getCache(key string) *valobj.IntentResult {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	e, ok := s.cache[key]
	if !ok || time.Now().After(e.expireAt) {
		return nil
	}
	return e.result
}

func (s *IntentService) putCache(key string, result *valobj.IntentResult) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if len(s.cache) >= s.maxCache {
		// 简单清空一半
		n := 0
		for k := range s.cache {
			delete(s.cache, k)
			n++
			if n >= s.maxCache/2 {
				break
			}
		}
	}
	cp := *result
	s.cache[key] = cacheEntry{result: &cp, expireAt: time.Now().Add(s.cacheTTL)}
}

func hashKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}
