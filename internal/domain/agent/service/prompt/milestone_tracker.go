package prompt

import (
	"context"
	"regexp"
	"strings"
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/repository"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

// MilestoneTracker 里程碑追踪 + 可选持久化
type MilestoneTracker struct {
	mu         sync.RWMutex
	milestones map[string][]*valobj.MilestoneVO
	maxHistory int
	repo       repository.IMilestoneRepository
	memoryRepo repository.ICoreMemoryRepository
}

var (
	regretPattern     = regexp.MustCompile(`(?i)(不对|不是这样|不是这样做的|错了|搞错了|失误|后悔|应该|其实应该|本来应该)`)
	directionPattern  = regexp.MustCompile(`(?i)(换个思路|换种方式|换个方向|改一下|换个方案|试试另一种|不如|还是用|重新来|重来)`)
	correctionPattern = regexp.MustCompile(`(?i)(不要|别再|停下来|不用了|取消|算了|别这样)`)
	toolErrorPattern  = regexp.MustCompile(`(?i)(error|failed|exception|permission denied|not found|refused|timeout|crash|fatal|失败|错误)`)
	completePattern   = regexp.MustCompile(`(?i)(完成了|搞定|结束|好了|没问题|done|finished)`)
)

func NewMilestoneTracker(maxHistory int) *MilestoneTracker {
	if maxHistory <= 0 {
		maxHistory = 100
	}
	return &MilestoneTracker{
		milestones: make(map[string][]*valobj.MilestoneVO),
		maxHistory: maxHistory,
	}
}

func (t *MilestoneTracker) SetRepository(repo repository.IMilestoneRepository) {
	t.repo = repo
}

func (t *MilestoneTracker) SetMemoryRepository(repo repository.ICoreMemoryRepository) {
	t.memoryRepo = repo
}

func (t *MilestoneTracker) AddMilestone(sessionID string, milestone *valobj.MilestoneVO) {
	if milestone == nil {
		return
	}
	t.mu.Lock()
	history := append(t.milestones[sessionID], milestone)
	if len(history) > t.maxHistory {
		history = history[len(history)-t.maxHistory:]
	}
	t.milestones[sessionID] = history
	t.mu.Unlock()

	if t.repo != nil {
		_ = t.repo.Save(context.Background(), sessionID, milestone)
	}
}

func (t *MilestoneTracker) GetMilestones(sessionID string) []*valobj.MilestoneVO {
	t.mu.RLock()
	src := t.milestones[sessionID]
	if len(src) > 0 {
		out := make([]*valobj.MilestoneVO, len(src))
		copy(out, src)
		t.mu.RUnlock()
		return out
	}
	t.mu.RUnlock()

	// 冷启动从 DB 加载
	if t.repo != nil {
		list, err := t.repo.ListBySession(context.Background(), sessionID, t.maxHistory)
		if err == nil && len(list) > 0 {
			t.mu.Lock()
			t.milestones[sessionID] = list
			t.mu.Unlock()
			return list
		}
	}
	return nil
}

func (t *MilestoneTracker) GetLastStep(sessionID string) int {
	history := t.GetMilestones(sessionID)
	if len(history) == 0 {
		return 0
	}
	return history[len(history)-1].Step
}

func (t *MilestoneTracker) ClearSession(sessionID string) {
	t.mu.Lock()
	delete(t.milestones, sessionID)
	t.mu.Unlock()
	if t.repo != nil {
		_ = t.repo.DeleteBySession(context.Background(), sessionID)
	}
}

func (t *MilestoneTracker) GetMilestonesByType(sessionID string, milestoneType valobj.MilestoneType) []*valobj.MilestoneVO {
	var result []*valobj.MilestoneVO
	for _, m := range t.GetMilestones(sessionID) {
		if m.IsOfType(milestoneType) {
			result = append(result, m)
		}
	}
	return result
}

// DetectAndRecord 根据角色与内容自动识别里程碑；用户纠正时写入长期记忆
func (t *MilestoneTracker) DetectAndRecord(sessionID, role, content string, step int) *valobj.MilestoneVO {
	return t.DetectAndRecordWithUser(sessionID, "", role, content, step)
}

func (t *MilestoneTracker) DetectAndRecordWithUser(sessionID, userID, role, content string, step int) *valobj.MilestoneVO {
	if sessionID == "" || content == "" {
		return nil
	}
	var typ valobj.MilestoneType

	switch strings.ToLower(role) {
	case "user":
		switch {
		case regretPattern.MatchString(content) || correctionPattern.MatchString(content):
			typ = valobj.MilestoneUserCorrection
		case directionPattern.MatchString(content):
			typ = valobj.MilestoneTaskChange
		case completePattern.MatchString(content):
			typ = valobj.MilestoneTaskComplete
		default:
			typ = valobj.MilestoneUserInput
		}
	case "tool":
		if toolErrorPattern.MatchString(content) {
			typ = valobj.MilestoneToolError
		} else {
			typ = valobj.MilestoneToolCalled
		}
	case "assistant":
		if completePattern.MatchString(content) {
			typ = valobj.MilestoneTaskComplete
		} else {
			return nil
		}
	default:
		return nil
	}

	c := content
	if len(c) > 200 {
		c = c[:200] + "..."
	}
	m := valobj.NewMilestone(typ, c, step)
	t.AddMilestone(sessionID, m)

	// 纠正/错误写入长期记忆
	if t.memoryRepo != nil && userID != "" {
		switch typ {
		case valobj.MilestoneUserCorrection, valobj.MilestoneTaskChange, valobj.MilestoneToolError:
			_ = t.memoryRepo.Save(context.Background(), userID, sessionID, string(typ), c, "milestone")
		}
	}
	return m
}
