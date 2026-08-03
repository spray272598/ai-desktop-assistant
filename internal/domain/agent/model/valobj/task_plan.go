package valobj

import "strings"

// TaskPlan 任务拆解计划
type TaskPlan struct {
	OriginalRequest string    `json:"originalRequest"`
	Summary         string    `json:"summary"`
	NeedConfirm     bool      `json:"needConfirm"`
	SubTasks        []SubTask `json:"subTasks"`
	Source          string    `json:"source"` // rule | llm | cache
	CurrentIndex    int       `json:"currentIndex,omitempty"`
}

// SubTask 子任务
type SubTask struct {
	Index         int    `json:"index"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	ExpectedTools string `json:"expectedTools"`
	Status        string `json:"status"` // pending | running | done | failed | skipped
	Result        string `json:"result,omitempty"`
}

func (p *TaskPlan) PendingCount() int {
	if p == nil {
		return 0
	}
	n := 0
	for _, t := range p.SubTasks {
		if t.Status == "" || t.Status == "pending" || t.Status == "running" {
			n++
		}
	}
	return n
}

func (p *TaskPlan) MarkDone(index int, result string) {
	if p == nil {
		return
	}
	for i := range p.SubTasks {
		if p.SubTasks[i].Index == index {
			p.SubTasks[i].Status = "done"
			p.SubTasks[i].Result = truncateResult(result, 200)
			return
		}
	}
}

// MarkRunning 将指定子任务标为 running，并取消其他 running
func (p *TaskPlan) MarkRunning(index int) {
	if p == nil {
		return
	}
	for i := range p.SubTasks {
		if p.SubTasks[i].Index == index {
			p.SubTasks[i].Status = "running"
			p.CurrentIndex = index
		} else if p.SubTasks[i].Status == "running" {
			p.SubTasks[i].Status = "pending"
		}
	}
}

// AdvanceWithTool 根据工具名推进计划：匹配 expectedTools 的首个 pending，否则推进首个 pending
func (p *TaskPlan) AdvanceWithTool(toolName, result string) (advanced bool) {
	if p == nil || len(p.SubTasks) == 0 {
		return false
	}
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	// 1) 若有 running，先完成它
	for i := range p.SubTasks {
		if p.SubTasks[i].Status == "running" {
			p.SubTasks[i].Status = "done"
			p.SubTasks[i].Result = truncateResult(result, 200)
			return true
		}
	}
	// 2) expectedTools 命中
	for i := range p.SubTasks {
		st := p.SubTasks[i]
		if st.Status != "" && st.Status != "pending" {
			continue
		}
		if st.ExpectedTools != "" && toolMatchesExpected(toolName, st.ExpectedTools) {
			p.SubTasks[i].Status = "done"
			p.SubTasks[i].Result = truncateResult(result, 200)
			p.CurrentIndex = st.Index
			return true
		}
	}
	// 3) 推进第一个 pending
	for i := range p.SubTasks {
		st := p.SubTasks[i]
		if st.Status == "" || st.Status == "pending" {
			p.SubTasks[i].Status = "done"
			p.SubTasks[i].Result = truncateResult(result, 200)
			p.CurrentIndex = st.Index
			return true
		}
	}
	return false
}

// StartNext 将下一个 pending 标为 running
func (p *TaskPlan) StartNext() *SubTask {
	if p == nil {
		return nil
	}
	for i := range p.SubTasks {
		if p.SubTasks[i].Status == "" || p.SubTasks[i].Status == "pending" {
			p.SubTasks[i].Status = "running"
			p.CurrentIndex = p.SubTasks[i].Index
			return &p.SubTasks[i]
		}
	}
	return nil
}

// AllDone 是否全部完成
func (p *TaskPlan) AllDone() bool {
	if p == nil || len(p.SubTasks) == 0 {
		return true
	}
	for _, t := range p.SubTasks {
		if t.Status != "done" && t.Status != "skipped" && t.Status != "failed" {
			return false
		}
	}
	return true
}

func (p *TaskPlan) StringForPrompt() string {
	if p == nil || len(p.SubTasks) == 0 {
		return ""
	}
	s := "任务计划: " + p.Summary + "\n"
	for _, t := range p.SubTasks {
		status := t.Status
		if status == "" {
			status = "pending"
		}
		s += "- [" + status + "] " + t.Title
		if t.ExpectedTools != "" {
			s += " (工具: " + t.ExpectedTools + ")"
		}
		s += "\n  " + t.Description + "\n"
		if t.Result != "" {
			s += "  结果: " + t.Result + "\n"
		}
	}
	if p.AllDone() {
		s += "（计划已全部完成，请汇总回答）\n"
	}
	return s
}

func toolMatchesExpected(tool, expected string) bool {
	for _, part := range strings.Split(expected, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		if part == tool || strings.Contains(tool, part) || strings.Contains(part, tool) {
			return true
		}
	}
	return false
}

func truncateResult(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
