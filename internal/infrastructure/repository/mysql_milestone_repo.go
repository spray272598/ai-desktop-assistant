package repository

import (
	"context"
	"database/sql"
	"time"

	domrepo "github.com/ai-desktop/assistant/internal/domain/agent/adapter/repository"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/valobj"
)

type MySQLMilestoneRepository struct {
	db *sql.DB
}

func NewMySQLMilestoneRepository(db *sql.DB) *MySQLMilestoneRepository {
	return &MySQLMilestoneRepository{db: db}
}

func (r *MySQLMilestoneRepository) Save(ctx context.Context, sessionID string, m *valobj.MilestoneVO) error {
	if m == nil {
		return nil
	}
	content := m.Content
	if len(content) > 1000 {
		content = content[:1000]
	}
	ts := m.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO chat_milestone (session_id, type, content, step, created_at) VALUES (?, ?, ?, ?, ?)`,
		sessionID, string(m.Type), content, m.Step, ts)
	return err
}

func (r *MySQLMilestoneRepository) ListBySession(ctx context.Context, sessionID string, limit int) ([]*valobj.MilestoneVO, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT type, content, step, created_at FROM chat_milestone
WHERE session_id=? ORDER BY created_at DESC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tmp []*valobj.MilestoneVO
	for rows.Next() {
		var typ, content string
		var step int
		var created time.Time
		if err := rows.Scan(&typ, &content, &step, &created); err != nil {
			return nil, err
		}
		tmp = append(tmp, &valobj.MilestoneVO{
			Type: valobj.MilestoneType(typ), Content: content, Step: step, Timestamp: created,
		})
	}
	out := make([]*valobj.MilestoneVO, 0, len(tmp))
	for i := len(tmp) - 1; i >= 0; i-- {
		out = append(out, tmp[i])
	}
	return out, rows.Err()
}

func (r *MySQLMilestoneRepository) DeleteBySession(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM chat_milestone WHERE session_id=?`, sessionID)
	return err
}

// ---- Core Memory ----

type MySQLCoreMemoryRepository struct {
	db *sql.DB
}

func NewMySQLCoreMemoryRepository(db *sql.DB) *MySQLCoreMemoryRepository {
	return &MySQLCoreMemoryRepository{db: db}
}

func (r *MySQLCoreMemoryRepository) Save(ctx context.Context, userID, sessionID, category, content, source string) error {
	if content == "" {
		return nil
	}
	if category == "" {
		category = "general"
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO core_memory (user_id, session_id, category, content, source) VALUES (?, ?, ?, ?, ?)`,
		userID, sessionID, category, content, source)
	return err
}

func (r *MySQLCoreMemoryRepository) ListByUser(ctx context.Context, userID string, limit int) ([]domrepo.CoreMemoryItem, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, category, content, source FROM core_memory
WHERE user_id=? ORDER BY updated_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domrepo.CoreMemoryItem
	for rows.Next() {
		var it domrepo.CoreMemoryItem
		if err := rows.Scan(&it.ID, &it.UserID, &it.Category, &it.Content, &it.Source); err != nil {
			return nil, err
		}
		list = append(list, it)
	}
	return list, rows.Err()
}
