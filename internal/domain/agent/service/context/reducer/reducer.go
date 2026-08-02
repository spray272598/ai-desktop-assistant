package reducer

// MessageReducer 历史消息裁剪接口
type MessageReducer interface {
	Reduce(messages []map[string]interface{}, tokenBudget int) []map[string]interface{}
}

// EstimateTokens 粗略估算 token（中文约 1.5 字符/token，英文约 4 字符/token）
func EstimateTokens(messages []map[string]interface{}) int {
	total := 0
	for _, m := range messages {
		if c, ok := m["content"].(string); ok {
			total += estimateString(c)
		}
	}
	return total
}

func estimateString(s string) int {
	if s == "" {
		return 0
	}
	// 简单：rune 数 * 0.7 约等于 token
	n := len([]rune(s))
	t := int(float64(n) * 0.7)
	if t < 1 {
		return 1
	}
	return t
}

func contentOf(m map[string]interface{}) string {
	if c, ok := m["content"].(string); ok {
		return c
	}
	return ""
}

func roleOf(m map[string]interface{}) string {
	if r, ok := m["role"].(string); ok {
		return r
	}
	return ""
}

func priorityOf(m map[string]interface{}) int {
	switch v := m["priority"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		// 默认按角色
		switch roleOf(m) {
		case "system":
			return 3
		case "user":
			return 2
		case "assistant":
			return 2
		case "tool":
			return 1
		default:
			return 0
		}
	}
}
