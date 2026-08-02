package reducer

// HybridReducer = Priority ∪ SlidingWindow（对标 walicode HybridReducer）
type HybridReducer struct {
	priority *PriorityReducer
	sliding  *SlidingWindowReducer
}

func NewHybridReducer() *HybridReducer {
	return &HybridReducer{
		priority: NewPriorityReducer(),
		sliding:  NewSlidingWindowReducer(),
	}
}

func (r *HybridReducer) Reduce(messages []map[string]interface{}, tokenBudget int) []map[string]interface{} {
	if len(messages) == 0 {
		return messages
	}
	if tokenBudget <= 0 {
		tokenBudget = 8000
	}

	// 总 token 未超预算则不裁剪
	if EstimateTokens(messages) <= tokenBudget {
		return messages
	}

	priKeep := indexSet(r.priority.Reduce(messages, tokenBudget), messages)
	slideKeep := indexSet(r.sliding.Reduce(messages, tokenBudget), messages)

	keep := make(map[int]bool)
	for i := range priKeep {
		keep[i] = true
	}
	for i := range slideKeep {
		keep[i] = true
	}
	// 至少最近 2 条
	minKeep := 2
	if len(messages) < minKeep {
		minKeep = len(messages)
	}
	for i := len(messages) - minKeep; i < len(messages); i++ {
		keep[i] = true
	}

	out := make([]map[string]interface{}, 0, len(keep))
	for i, m := range messages {
		if keep[i] {
			out = append(out, m)
		}
	}

	// 若仍超预算，再做一次滑动窗口收紧
	if EstimateTokens(out) > tokenBudget {
		out = r.sliding.Reduce(out, tokenBudget)
	}
	return out
}

func indexSet(subset, all []map[string]interface{}) map[int]bool {
	// 用指针身份匹配（同一 map 引用）
	set := make(map[int]bool)
	for _, s := range subset {
		for i, a := range all {
			// 比较 id 或内容+role
			if sameMessage(s, a) {
				set[i] = true
				break
			}
		}
	}
	return set
}

func sameMessage(a, b map[string]interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	// 优先 id
	aid, aok := a["id"].(string)
	bid, bok := b["id"].(string)
	if aok && bok && aid != "" && aid == bid {
		return true
	}
	return roleOf(a) == roleOf(b) && contentOf(a) == contentOf(b)
}
