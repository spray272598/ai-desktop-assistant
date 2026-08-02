package valobj

import "time"

type MilestoneType string

const (
	MilestoneTaskStart      MilestoneType = "TASK_START"
	MilestoneTaskComplete   MilestoneType = "TASK_COMPLETE"
	MilestoneTaskChange     MilestoneType = "TASK_CHANGE"
	MilestoneToolCalled     MilestoneType = "TOOL_CALLED"
	MilestoneToolError      MilestoneType = "TOOL_ERROR"
	MilestoneStepReach      MilestoneType = "STEP_REACHED"
	MilestoneError          MilestoneType = "ERROR"
	MilestoneUserInput      MilestoneType = "USER_INPUT"
	MilestoneUserCorrection MilestoneType = "USER_CORRECTION"
)

type MilestoneVO struct {
	Type      MilestoneType `json:"type"`
	Content   string        `json:"content"`
	Step      int           `json:"step"`
	Timestamp time.Time     `json:"timestamp"`
}

func NewMilestone(typ MilestoneType, content string, step int) *MilestoneVO {
	return &MilestoneVO{
		Type:      typ,
		Content:   content,
		Step:      step,
		Timestamp: time.Now(),
	}
}

func (m *MilestoneVO) IsOfType(typ MilestoneType) bool {
	return m.Type == typ
}

func (m *MilestoneVO) String() string {
	return "[" + string(m.Type) + "] Step" + itoa(m.Step) + ": " + m.Content
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
