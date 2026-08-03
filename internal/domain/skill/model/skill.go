package model

// Skill 可安装工作流（SKILL.md 解析结果）
// 工具是原子能力；Skill 是可复用步骤与约束，注入 Prompt 后仍由 ReAct 执行。
type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Triggers    []string `json:"triggers,omitempty"`
	Tools       []string `json:"tools,omitempty"` // 推荐/限制工具名；空=不限制
	Body        string   `json:"body,omitempty"`  // Markdown 执行指南
	Path        string   `json:"path,omitempty"`  // 技能目录
	Source      string   `json:"source,omitempty"` // builtin | installed
	Enabled     bool     `json:"enabled"`
}

// MatchScore 简单触发匹配分（越高越优先）
func (s *Skill) MatchScore(userInput string) int {
	if s == nil || !s.Enabled {
		return 0
	}
	input := toLower(userInput)
	score := 0
	name := toLower(s.Name)
	id := toLower(s.ID)
	if name != "" && contains(input, name) {
		score += 30
	}
	if id != "" && contains(input, id) {
		score += 25
	}
	for _, t := range s.Triggers {
		t = toLower(t)
		if t == "" {
			continue
		}
		if contains(input, t) {
			score += 20 + len([]rune(t))/2
		}
	}
	// 描述关键词弱匹配
	if s.Description != "" {
		for _, w := range splitWords(s.Description) {
			if len(w) >= 2 && contains(input, w) {
				score += 2
			}
		}
	}
	return score
}

func toLower(s string) string {
	b := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b = append(b, r+32)
		} else {
			b = append(b, r)
		}
	}
	return string(b)
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	// simple substring
	n, m := len(s), len(sub)
	if m == 0 || m > n {
		if m == 0 {
			return 0
		}
		return -1
	}
	for i := 0; i <= n-m; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

func splitWords(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = cur[:0]
		}
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r > 127 {
			if r >= 'A' && r <= 'Z' {
				r += 32
			}
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
