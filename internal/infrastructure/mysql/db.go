package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/ai-desktop/assistant/internal/infrastructure/config"
)

// Open 打开 MySQL 连接并可选自动建表
func Open(cfg config.MySQLConfig, autoMigrate bool, schemaPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local&multiStatements=true",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	if cfg.Database == "" {
		// 先连无库创建
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=true&loc=Local&multiStatements=true",
			cfg.Username, cfg.Password, cfg.Host, cfg.Port)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// 等待可用
	var lastErr error
	for i := 0; i < 30; i++ {
		if err := db.Ping(); err == nil {
			lastErr = nil
			break
		} else {
			lastErr = err
			time.Sleep(time.Second)
		}
	}
	if lastErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql ping: %w", lastErr)
	}

	if autoMigrate {
		if err := Migrate(db, cfg.Database, schemaPath); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	log.Printf("[mysql] connected %s:%d/%s\n", cfg.Host, cfg.Port, cfg.Database)
	return db, nil
}

// Migrate 执行 schema SQL
func Migrate(db *sql.DB, database, schemaPath string) error {
	sqlText := embeddedSchema
	if schemaPath != "" {
		if b, err := os.ReadFile(schemaPath); err == nil {
			sqlText = string(b)
		} else {
			// 尝试相对路径
			if abs, e2 := filepath.Abs(schemaPath); e2 == nil {
				if b, e3 := os.ReadFile(abs); e3 == nil {
					sqlText = string(b)
				}
			}
		}
	}

	// 确保库存在
	if database != "" {
		if _, err := db.Exec("CREATE DATABASE IF NOT EXISTS `" + database + "` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
			return fmt.Errorf("create database: %w", err)
		}
		if _, err := db.Exec("USE `" + database + "`"); err != nil {
			return fmt.Errorf("use database: %w", err)
		}
	}

	// 按语句拆分执行（跳过注释与空行）
	stmts := splitSQL(sqlText)
	for _, stmt := range stmts {
		s := strings.TrimSpace(stmt)
		if s == "" || strings.HasPrefix(s, "--") {
			continue
		}
		// 已在上面处理 USE/CREATE DATABASE
		upper := strings.ToUpper(s)
		if strings.HasPrefix(upper, "CREATE DATABASE") || strings.HasPrefix(upper, "USE ") {
			continue
		}
		if _, err := db.Exec(s); err != nil {
			// 忽略已存在
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			return fmt.Errorf("migrate stmt failed: %w\nSQL: %s", err, truncate(s, 120))
		}
	}
	log.Println("[mysql] schema migrated")
	return nil
}

func splitSQL(sqlText string) []string {
	// 简单按分号拆分，忽略字符串内复杂情况（本 schema 无复杂字符串）
	parts := strings.Split(sqlText, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		// 去掉行注释
		lines := strings.Split(p, "\n")
		var cleaned []string
		for _, line := range lines {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "--") {
				continue
			}
			cleaned = append(cleaned, line)
		}
		out = append(out, strings.Join(cleaned, "\n"))
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// 内嵌默认 schema，避免路径依赖
const embeddedSchema = `
CREATE TABLE IF NOT EXISTS chat_session (
  id            VARCHAR(64)  NOT NULL,
  agent_id      VARCHAR(64)  NOT NULL DEFAULT 'desktop-agent',
  user_id       VARCHAR(64)  NOT NULL,
  title         VARCHAR(255) NOT NULL DEFAULT '',
  status        VARCHAR(32)  NOT NULL DEFAULT 'ACTIVE',
  message_count INT          NOT NULL DEFAULT 0,
  token_used    INT          NOT NULL DEFAULT 0,
  working_dir   VARCHAR(512) NOT NULL DEFAULT '',
  metadata_json JSON         NULL,
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_user_status (user_id, status),
  KEY idx_updated (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS chat_message (
  id           VARCHAR(64)  NOT NULL,
  session_id   VARCHAR(64)  NOT NULL,
  role         VARCHAR(32)  NOT NULL,
  content      MEDIUMTEXT   NOT NULL,
  tool_name    VARCHAR(128) NOT NULL DEFAULT '',
  tool_call_id VARCHAR(128) NOT NULL DEFAULT '',
  step         INT          NOT NULL DEFAULT 0,
  token_count  INT          NOT NULL DEFAULT 0,
  priority     INT          NOT NULL DEFAULT 1,
  created_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_session_created (session_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS chat_milestone (
  id         BIGINT       NOT NULL AUTO_INCREMENT,
  session_id VARCHAR(64)  NOT NULL,
  type       VARCHAR(64)  NOT NULL,
  content    VARCHAR(1024) NOT NULL DEFAULT '',
  step       INT          NOT NULL DEFAULT 0,
  created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_session_created (session_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS core_memory (
  id         BIGINT       NOT NULL AUTO_INCREMENT,
  user_id    VARCHAR(64)  NOT NULL,
  session_id VARCHAR(64)  NOT NULL DEFAULT '',
  category   VARCHAR(64)  NOT NULL DEFAULT 'general',
  content    TEXT         NOT NULL,
  source     VARCHAR(64)  NOT NULL DEFAULT '',
  created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_user (user_id),
  KEY idx_user_cat (user_id, category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS mcp_server_config (
  id           BIGINT       NOT NULL AUTO_INCREMENT,
  name         VARCHAR(64)  NOT NULL,
  transport    VARCHAR(32)  NOT NULL,
  command      VARCHAR(512) NOT NULL DEFAULT '',
  args_json    JSON         NULL,
  env_json     JSON         NULL,
  url          VARCHAR(1024) NOT NULL DEFAULT '',
  enabled      TINYINT(1)   NOT NULL DEFAULT 1,
  timeout_sec  INT          NOT NULL DEFAULT 60,
  created_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
