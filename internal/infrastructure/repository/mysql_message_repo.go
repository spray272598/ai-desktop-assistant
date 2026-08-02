package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
	"github.com/ai-desktop/assistant/internal/types/enums"
)

type MySQLMessageRepository struct {
	db *sql.DB
}

func NewMySQLMessageRepository(db *sql.DB) *MySQLMessageRepository {
	return &MySQLMessageRepository{db: db}
}

func (r *MySQLMessageRepository) Save(ctx context.Context, msg *entity.MessageEntity) error {
	if msg == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO chat_message (id, session_id, role, content, tool_name, tool_call_id, step, token_count, priority, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE content=VALUES(content), tool_name=VALUES(tool_name), step=VALUES(step), token_count=VALUES(token_count)
`, msg.ID, msg.SessionID, string(msg.Role), msg.Content, msg.ToolName, msg.ToolCallID,
		msg.Step, msg.TokenCount, int(msg.Priority), msg.CreatedAt)
	return err
}

func (r *MySQLMessageRepository) SaveBatch(ctx context.Context, msgs []*entity.MessageEntity) error {
	for _, m := range msgs {
		if err := r.Save(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLMessageRepository) ListBySession(ctx context.Context, sessionID string, limit int) ([]*entity.MessageEntity, error) {
	if limit <= 0 {
		limit = 200
	}
	// 取最近 limit 条，再按时间正序返回
	rows, err := r.db.QueryContext(ctx, `
SELECT id, session_id, role, content, tool_name, tool_call_id, step, token_count, priority, created_at
FROM (
  SELECT id, session_id, role, content, tool_name, tool_call_id, step, token_count, priority, created_at
  FROM chat_message WHERE session_id=?
  ORDER BY created_at DESC LIMIT ?
) t ORDER BY created_at ASC`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*entity.MessageEntity
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MySQLMessageRepository) ListAsMaps(ctx context.Context, sessionID string, limit int) ([]map[string]interface{}, error) {
	msgs, err := r.ListBySession(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.ToMap())
	}
	return out, nil
}

func (r *MySQLMessageRepository) DeleteBySession(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM chat_message WHERE session_id=?`, sessionID)
	return err
}

func (r *MySQLMessageRepository) CountBySession(ctx context.Context, sessionID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM chat_message WHERE session_id=?`, sessionID).Scan(&n)
	return n, err
}

func scanMessage(rows *sql.Rows) (*entity.MessageEntity, error) {
	var (
		id, sessionID, role, content, toolName, toolCallID string
		step, tokenCount, priority                         int
		createdAt                                          time.Time
	)
	if err := rows.Scan(&id, &sessionID, &role, &content, &toolName, &toolCallID, &step, &tokenCount, &priority, &createdAt); err != nil {
		return nil, err
	}
	return &entity.MessageEntity{
		ID:         id,
		SessionID:  sessionID,
		Role:       enums.MessageRole(role),
		Content:    content,
		ToolName:   toolName,
		ToolCallID: toolCallID,
		Step:       step,
		TokenCount: tokenCount,
		Priority:   enums.Priority(priority),
		CreatedAt:  createdAt,
	}, nil
}
