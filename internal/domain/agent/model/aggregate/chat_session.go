package aggregate

import (
	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

// ChatSessionAggregate 会话聚合根：会话 + 消息 + 意图快照
type ChatSessionAggregate struct {
	Session      *entity.SessionEntity
	Messages     []*entity.MessageEntity
	LastIntent   *valobj.IntentResult
	Milestones   []*valobj.MilestoneVO
}

func NewChatSessionAggregate(session *entity.SessionEntity) *ChatSessionAggregate {
	return &ChatSessionAggregate{
		Session:  session,
		Messages: make([]*entity.MessageEntity, 0),
	}
}

func (a *ChatSessionAggregate) AppendMessage(msg *entity.MessageEntity) {
	if msg == nil {
		return
	}
	a.Messages = append(a.Messages, msg)
	if a.Session != nil {
		a.Session.AddMessage(msg.TokenCount)
	}
}

func (a *ChatSessionAggregate) SetIntent(intent *valobj.IntentResult) {
	a.LastIntent = intent
}
