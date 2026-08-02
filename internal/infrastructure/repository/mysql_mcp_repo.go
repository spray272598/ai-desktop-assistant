package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

type MySQLMCPServerRepository struct {
	db *sql.DB
}

func NewMySQLMCPServerRepository(db *sql.DB) *MySQLMCPServerRepository {
	return &MySQLMCPServerRepository{db: db}
}

func (r *MySQLMCPServerRepository) Save(ctx context.Context, cfg *entity.ServerConfig) error {
	if cfg == nil {
		return nil
	}
	argsJSON, err := json.Marshal(cfg.Args)
	if err != nil {
		return err
	}
	envJSON, err := json.Marshal(cfg.Env)
	if err != nil {
		return err
	}
	enabled := 0
	if cfg.Enabled {
		enabled = 1
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO mcp_server_config (name, transport, command, args_json, env_json, url, enabled, timeout_sec)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  transport=VALUES(transport), command=VALUES(command), args_json=VALUES(args_json),
  env_json=VALUES(env_json), url=VALUES(url), enabled=VALUES(enabled), timeout_sec=VALUES(timeout_sec)
`, cfg.Name, cfg.Transport, cfg.Command, argsJSON, envJSON, cfg.URL, enabled, cfg.TimeoutSec)
	return err
}

func (r *MySQLMCPServerRepository) Delete(ctx context.Context, name string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM mcp_server_config WHERE name=?`, name)
	return err
}

func (r *MySQLMCPServerRepository) FindByName(ctx context.Context, name string) (*entity.ServerConfig, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT name, transport, command, args_json, env_json, url, enabled, timeout_sec
FROM mcp_server_config WHERE name=?`, name)
	return scanMCP(row)
}

func (r *MySQLMCPServerRepository) List(ctx context.Context) ([]entity.ServerConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT name, transport, command, args_json, env_json, url, enabled, timeout_sec
FROM mcp_server_config ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMCPs(rows)
}

func (r *MySQLMCPServerRepository) ListEnabled(ctx context.Context) ([]entity.ServerConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT name, transport, command, args_json, env_json, url, enabled, timeout_sec
FROM mcp_server_config WHERE enabled=1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMCPs(rows)
}

type mcpScanner interface {
	Scan(dest ...any) error
}

func scanMCP(row mcpScanner) (*entity.ServerConfig, error) {
	var name, transport, command, url string
	var argsRaw, envRaw sql.NullString
	var enabled, timeout int
	err := row.Scan(&name, &transport, &command, &argsRaw, &envRaw, &url, &enabled, &timeout)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cfg := &entity.ServerConfig{
		Name: name, Transport: transport, Command: command, URL: url,
		Enabled: enabled == 1, TimeoutSec: timeout,
	}
	if argsRaw.Valid && argsRaw.String != "" {
		_ = json.Unmarshal([]byte(argsRaw.String), &cfg.Args)
	}
	if envRaw.Valid && envRaw.String != "" {
		_ = json.Unmarshal([]byte(envRaw.String), &cfg.Env)
	}
	return cfg, nil
}

func scanMCPs(rows *sql.Rows) ([]entity.ServerConfig, error) {
	var list []entity.ServerConfig
	for rows.Next() {
		c, err := scanMCP(rows)
		if err != nil {
			return nil, err
		}
		if c != nil {
			list = append(list, *c)
		}
	}
	return list, rows.Err()
}
