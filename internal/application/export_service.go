package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/repository"
	"github.com/ai-desktop/assistant/internal/domain/agent/model/entity"
)

// SessionExport 导出会话包
type SessionExport struct {
	Version   string                   `json:"version"`
	ExportedAt string                  `json:"exportedAt"`
	Session   *entity.SessionEntity    `json:"session"`
	Messages  []*entity.MessageEntity  `json:"messages"`
}

// ExportService 会话导出/导入
type ExportService struct {
	sessions repository.ISessionRepository
	messages repository.IMessageRepository
	exportDir string
}

func NewExportService(sessions repository.ISessionRepository, messages repository.IMessageRepository, dir string) *ExportService {
	if dir == "" {
		dir = "./exports"
	}
	_ = os.MkdirAll(dir, 0755)
	return &ExportService{sessions: sessions, messages: messages, exportDir: dir}
}

func (s *ExportService) ExportJSON(ctx context.Context, sessionID string) (string, error) {
	sess, err := s.sessions.FindByID(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if sess == nil {
		return "", fmt.Errorf("session not found")
	}
	msgs, err := s.messages.ListBySession(ctx, sessionID, 5000)
	if err != nil {
		return "", err
	}
	pack := SessionExport{
		Version: "1.0", ExportedAt: time.Now().Format(time.RFC3339),
		Session: sess, Messages: msgs,
	}
	b, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(s.exportDir, fmt.Sprintf("%s.json", sessionID))
	if err := os.WriteFile(path, b, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *ExportService) ExportMarkdown(ctx context.Context, sessionID string) (string, error) {
	sess, err := s.sessions.FindByID(ctx, sessionID)
	if err != nil {
		return "", err
	}
	if sess == nil {
		return "", fmt.Errorf("session not found")
	}
	msgs, err := s.messages.ListBySession(ctx, sessionID, 5000)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# Session " + sess.ID + "\n\n")
	b.WriteString("- Title: " + sess.Title + "\n")
	b.WriteString("- User: " + sess.UserID + "\n")
	b.WriteString("- Exported: " + time.Now().Format(time.RFC3339) + "\n\n---\n\n")
	for _, m := range msgs {
		role := string(m.Role)
		b.WriteString("### " + role)
		if m.ToolName != "" {
			b.WriteString(" (" + m.ToolName + ")")
		}
		b.WriteString("\n\n")
		b.WriteString(m.Content)
		b.WriteString("\n\n")
	}
	path := filepath.Join(s.exportDir, fmt.Sprintf("%s.md", sessionID))
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *ExportService) ImportJSON(ctx context.Context, filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	var pack SessionExport
	if err := json.Unmarshal(data, &pack); err != nil {
		return "", err
	}
	if pack.Session == nil {
		return "", fmt.Errorf("invalid export: missing session")
	}
	// 新 ID 避免冲突
	oldID := pack.Session.ID
	pack.Session.ID = fmt.Sprintf("import-%d", time.Now().UnixMilli())
	pack.Session.Title = pack.Session.Title + " (imported)"
	if err := s.sessions.Save(ctx, pack.Session); err != nil {
		return "", err
	}
	for _, m := range pack.Messages {
		if m == nil {
			continue
		}
		m.SessionID = pack.Session.ID
		if m.ID == "" || strings.Contains(m.ID, oldID) {
			m.ID = fmt.Sprintf("%s-%d", pack.Session.ID, time.Now().UnixNano())
		}
		_ = s.messages.Save(ctx, m)
	}
	return pack.Session.ID, nil
}

func (s *ExportService) ExportDir() string { return s.exportDir }
