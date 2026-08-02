package reducer

// SlidingWindowReducer 保留最近消息直到 token 预算用尽
type SlidingWindowReducer struct{}

func NewSlidingWindowReducer() *SlidingWindowReducer {
	return &SlidingWindowReducer{}
}

func (r *SlidingWindowReducer) Reduce(messages []map[string]interface{}, tokenBudget int) []map[string]interface{} {
	if len(messages) == 0 {
		return messages
	}
	if tokenBudget <= 0 {
		tokenBudget = 8000
	}

	// 从尾部往前累加
	used := 0
	start := len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		t := estimateString(contentOf(messages[i])) + 4
		if used+t > tokenBudget && start < len(messages) {
			break
		}
		used += t
		start = i
	}
	// 至少保留最近 2 条
	minKeep := 2
	if len(messages) < minKeep {
		minKeep = len(messages)
	}
	if start > len(messages)-minKeep {
		start = len(messages) - minKeep
	}
	if start < 0 {
		start = 0
	}
	out := make([]map[string]interface{}, len(messages)-start)
	copy(out, messages[start:])
	return out
}
