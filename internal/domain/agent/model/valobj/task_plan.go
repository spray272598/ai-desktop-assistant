package valobj

// TaskPlan 任务拆解计划
type TaskPlan struct {
	OriginalRequest string     `json:"originalRequest"`
	Summary         string     `json:"summary"`
	NeedConfirm     bool       `json:"needConfirm"`
	SubTasks        []SubTask  `json:"subTasks"`
	Source          string     `json:"source"` // rule | llm | cache
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
	n := 0
	for _, t := range p.SubTasks {
		if t.Status == "" || t.Status == "pending" || t.Status == "running" {
			n++
		}
	}
	return n
}

func (p *TaskPlan) MarkDone(index int, result string) {
	for i := range p.SubTasks {
		if p.SubTasks[i].Index == index {
			p.SubTasks[i].Status = "done"
			p.SubTasks[i].Result = result
			return
		}
	}
}

func (p *TaskPlan) StringForPrompt() string {
	if p == nil || len(p.SubTasks) == 0 {
		return ""
	}
	s := "任务计划: " + p.Summary + "\n"
	for _, t := range p.SubTasks {
		s += "- [" + t.Status + "] " + t.Title
		if t.ExpectedTools != "" {
			s += " (工具: " + t.ExpectedTools + ")"
		}
		s += "\n  " + t.Description + "\n"
	}
	return s
}
