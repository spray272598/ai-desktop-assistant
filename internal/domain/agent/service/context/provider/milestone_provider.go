package provider

import (
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
	"github.com/ai-desktop/assistant/internal/domain/agent/service/prompt"
)

// MilestoneProvider 注入里程碑
type MilestoneProvider struct {
	tracker *prompt.MilestoneTracker
}

func NewMilestoneProvider(tracker *prompt.MilestoneTracker) *MilestoneProvider {
	return &MilestoneProvider{tracker: tracker}
}

func (p *MilestoneProvider) Name() string  { return "milestone" }
func (p *MilestoneProvider) Order() int    { return 30 }
func (p *MilestoneProvider) Enabled() bool { return p.tracker != nil }

func (p *MilestoneProvider) Provide(sessionID, _, _ string, _ []map[string]interface{}) map[string]interface{} {
	if p.tracker == nil {
		return nil
	}
	ms := p.tracker.GetMilestones(sessionID)
	// 只取最近 8 条
	if len(ms) > 8 {
		ms = ms[len(ms)-8:]
	}
	// 拷贝避免外部修改
	out := make([]*valobj.MilestoneVO, len(ms))
	copy(out, ms)
	return map[string]interface{}{
		"milestoneVOS": out,
	}
}
