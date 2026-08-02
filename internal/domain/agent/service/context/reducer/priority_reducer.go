package reducer

import "sort"

// PriorityReducer 优先保留高优先级消息
type PriorityReducer struct{}

func NewPriorityReducer() *PriorityReducer {
	return &PriorityReducer{}
}

type scoredMsg struct {
	idx   int
	score int
	tokens int
	msg   map[string]interface{}
}

func (r *PriorityReducer) Reduce(messages []map[string]interface{}, tokenBudget int) []map[string]interface{} {
	if len(messages) == 0 {
		return messages
	}
	if tokenBudget <= 0 {
		tokenBudget = 8000
	}

	items := make([]scoredMsg, len(messages))
	for i, m := range messages {
		// 越新分数越高一点
		recency := i
		pri := priorityOf(m) * 1000
		items[i] = scoredMsg{
			idx:    i,
			score:  pri + recency,
			tokens: estimateString(contentOf(m)) + 4,
			msg:    m,
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	keep := make(map[int]bool)
	used := 0
	for _, it := range items {
		if used+it.tokens > tokenBudget && len(keep) >= 2 {
			continue
		}
		keep[it.idx] = true
		used += it.tokens
	}

	// 保证最近 2 条
	for i := len(messages) - 1; i >= 0 && i >= len(messages)-2; i-- {
		keep[i] = true
	}

	out := make([]map[string]interface{}, 0, len(keep))
	for i, m := range messages {
		if keep[i] {
			out = append(out, m)
		}
	}
	return out
}
