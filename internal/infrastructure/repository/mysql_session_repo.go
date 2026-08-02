package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
	"github.com/ai-desktop/assistant/internal/types/common"
)

type MySQLSessionRepository struct {
	db *sql.DB
}

func NewMySQLSessionRepository(db *sql.DB) *MySQLSessionRepository {
	return &MySQLSessionRepository{db: db}
}

func (r *MySQLSessionRepository) Save(ctx context.Context, session *entity.SessionEntity) error {
	if session == nil {
		return nil
	}
	meta, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("marshal session metadata: %w", err)
	}
	if len(meta) == 0 || string(meta) == "null" {
		meta = []byte("{}")
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO chat_session (id, agent_id, user_id, title, status, message_count, token_used, working_dir, metadata_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  agent_id=VALUES(agent_id), user_id=VALUES(user_id), title=VALUES(title), status=VALUES(status),
  message_count=VALUES(message_count), token_used=VALUES(token_used), working_dir=VALUES(working_dir),
  metadata_json=VALUES(metadata_json), updated_at=VALUES(updated_at)
`, session.ID, session.AgentID, session.UserID, session.Title, session.Status,
		session.MessageCount, session.TokenUsed, session.WorkingDir, meta,
		session.CreatedAt, session.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save session %s: %w", session.ID, err)
	}
	return nil
}

func (r *MySQLSessionRepository) FindByID(ctx context.Context, id string) (*entity.SessionEntity, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, agent_id, user_id, title, status, message_count, token_used, working_dir, metadata_json, created_at, updated_at
FROM chat_session WHERE id=?`, id)
	s, err := scanSession(row)
	if err != nil {
		return nil, fmt.Errorf("find session %s: %w", id, err)
	}
	return s, nil
}

func (r *MySQLSessionRepository) FindByUser(ctx context.Context, userID string) ([]*entity.SessionEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, agent_id, user_id, title, status, message_count, token_used, working_dir, metadata_json, created_at, updated_at
FROM chat_session WHERE user_id=? ORDER BY updated_at DESC LIMIT ?`, userID, common.DefaultFindByUserLimit)
	if err != nil {
		return nil, fmt.Errorf("find sessions by user %s: %w", userID, err)
	}
	defer rows.Close()
	list, err := scanSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("scan sessions by user %s: %w", userID, err)
	}
	return list, nil
}

func (r *MySQLSessionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM chat_session WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete session %s: %w", id, err)
	}
	return nil
}

func (r *MySQLSessionRepository) ListActive(ctx context.Context, limit int) ([]*entity.SessionEntity, error) {
	if limit <= 0 {
		limit = common.DefaultListActiveLimit
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, agent_id, user_id, title, status, message_count, token_used, working_dir, metadata_json, created_at, updated_at
FROM chat_session WHERE status='ACTIVE' ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()
	list, err := scanSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("scan active sessions: %w", err)
	}
	return list, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(row rowScanner) (*entity.SessionEntity, error) {
	var (
		id, agentID, userID, title, status, workDir string
		msgCount, tokenUsed                         int
		metaRaw                                     sql.NullString
		createdAt, updatedAt                        time.Time
	)
	err := row.Scan(&id, &agentID, &userID, &title, &status, &msgCount, &tokenUsed, &workDir, &metaRaw, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := entity.NewSessionEntity(id, agentID, userID, title)
	s.Status = status
	s.MessageCount = msgCount
	s.TokenUsed = tokenUsed
	s.WorkingDir = workDir
	s.CreatedAt = createdAt
	s.UpdatedAt = updatedAt
	if metaRaw.Valid && metaRaw.String != "" && metaRaw.String != "null" {
		if err := json.Unmarshal([]byte(metaRaw.String), &s.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal session metadata: %w", err)
		}
	}
	return s, nil
}

func scanSessions(rows *sql.Rows) ([]*entity.SessionEntity, error) {
	var list []*entity.SessionEntity
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
