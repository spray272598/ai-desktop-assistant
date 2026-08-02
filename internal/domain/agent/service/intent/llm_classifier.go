package intent

import (
	"context"
	"strings"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
	"github.com/ai-desktop/assistant/internal/types/enums"
)

type LLMClassifier struct {
	llm port.ILLMPort
}

func NewLLMClassifier(llm port.ILLMPort) *LLMClassifier {
	return &LLMClassifier{llm: llm}
}

func (c *LLMClassifier) Classify(ctx context.Context, input string, hint string) *valobj.IntentResult {
	if c == nil || c.llm == nil {
		result := valobj.NewIntentResult(string(enums.IntentChat), 0.4, nil)
		result.Source = "fallback"
		return result
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	intent, confidence, entities, err := c.llm.ClassifyIntent(ctx, input, hint)
	if err != nil || intent == "" {
		result := valobj.NewIntentResult(string(enums.IntentChat), 0.4, nil)
		result.Source = "llm-error"
		return result
	}

	intent = normalizeIntent(intent)
	if confidence <= 0 {
		confidence = 0.6
	}
	if confidence > 1 {
		confidence = 1
	}
	result := valobj.NewIntentResult(intent, confidence, entities)
	result.Source = "llm"
	return result
}

func normalizeIntent(intent string) string {
	intent = strings.TrimSpace(strings.ToUpper(intent))
	intent = strings.ReplaceAll(intent, "-", "_")
	intent = strings.ReplaceAll(intent, " ", "_")
	valid := map[string]bool{
		"READ_FILE": true, "WRITE_FILE": true, "LIST_FILES": true,
		"DELETE_FILE": true, "CREATE_DIR": true, "RUN_COMMAND": true,
		"RUN_SCRIPT": true, "START_APP": true, "SCREENSHOT": true,
		"ANALYZE_SCREEN": true, "OPEN_URL": true, "BROWSER_ACTION": true,
		"GET_CLIPBOARD": true, "SET_CLIPBOARD": true, "SYSTEM_INFO": true,
		"CHAT": true, "TASK_PLAN": true, "UNKNOWN": true,
	}
	if valid[intent] {
		return intent
	}
	return string(enums.IntentChat)
}
