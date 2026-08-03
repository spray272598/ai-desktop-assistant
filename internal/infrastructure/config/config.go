package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Agent    AgentConfig    `yaml:"agent"`
	LLM      LLMConfig      `yaml:"llm"`
	Desktop  DesktopConfig  `yaml:"desktop"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Logging  LoggingConfig  `yaml:"logging"`
	Tools    ToolsConfig    `yaml:"tools"`
	MCP       MCPConfig       `yaml:"mcp"`
	Sandbox   SandboxConfig   `yaml:"sandbox"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Skills    SkillsConfig    `yaml:"skills"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type AgentConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	MaxSteps    int    `yaml:"max_steps"`
	Timeout     int    `yaml:"timeout"`
	TokenBudget int    `yaml:"token_budget"`
}

type LLMConfig struct {
	Provider   string            `yaml:"provider"`
	Model      string            `yaml:"model"`
	APIKey     string            `yaml:"api_key"`
	APIBase    string            `yaml:"api_base"`
	UseMock    bool              `yaml:"use_mock"`
	// Models A/B 模型档
	Models     map[string]ModelEntry `yaml:"models"`
}

type ModelEntry struct {
	Model    string   `yaml:"model"`
	APIBase  string   `yaml:"api_base"`
	Weight   int      `yaml:"weight"`
	Scenarios []string `yaml:"scenarios"`
}

type DesktopConfig struct {
	Workspace     string `yaml:"workspace"`
	TempDir       string `yaml:"temp_dir"`
	ScreenshotDir string `yaml:"screenshot_dir"`
}

type DatabaseConfig struct {
	Type        string      `yaml:"type"`
	AutoMigrate bool        `yaml:"auto_migrate"`
	SchemaPath  string      `yaml:"schema_path"`
	MySQL       MySQLConfig `yaml:"mysql"`
}

type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type RedisConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

type ToolsConfig struct {
	File       ToolToggle        `yaml:"file"`
	Command    CommandToolConfig `yaml:"command"`
	Screenshot ToolToggle        `yaml:"screenshot"`
	Browser    ToolToggle        `yaml:"browser"`
	Sandbox    ToolToggle        `yaml:"sandbox"`
}

type ToolToggle struct {
	Enabled bool   `yaml:"enabled"`
	BaseDir string `yaml:"base_dir"`
	SaveDir string `yaml:"save_dir"`
}

type CommandToolConfig struct {
	Enabled    bool     `yaml:"enabled"`
	MaxTimeout int      `yaml:"max_timeout"`
	AllowList  []string `yaml:"allow_list"`
	DenyList   []string `yaml:"deny_list"`
}

type MCPConfig struct {
	Enabled bool              `yaml:"enabled"`
	Servers []MCPServerConfig `yaml:"servers"`
}

type MCPServerConfig struct {
	Name       string            `yaml:"name"`
	Transport  string            `yaml:"transport"`
	Command    string            `yaml:"command"`
	Args       []string          `yaml:"args"`
	Env        map[string]string `yaml:"env"`
	URL        string            `yaml:"url"`
	Enabled    *bool             `yaml:"enabled"`
	TimeoutSec int               `yaml:"timeout_sec"`
}

type SandboxConfig struct {
	Enabled    bool   `yaml:"enabled"`
	UseDocker  bool   `yaml:"use_docker"`
	WorkDir    string `yaml:"work_dir"`
}

type RateLimitConfig struct {
	Enabled bool `yaml:"enabled"`
	// 每分钟每 user 请求上限
	PerMinute int `yaml:"per_minute"`
}

// SkillsConfig Skill 工作流目录
type SkillsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"` // 默认 ./skills
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 8080, Mode: "debug"},
		Agent: AgentConfig{
			Name: "AI Desktop Assistant", Version: "1.1.0",
			MaxSteps: 15, Timeout: 120, TokenBudget: 16000,
		},
		LLM: LLMConfig{
			Provider: "siliconflow",
			Model:    "deepseek-ai/DeepSeek-V3",
			APIBase:  "https://api.siliconflow.cn/v1",
			Models: map[string]ModelEntry{
				"fast":   {Model: "deepseek-ai/DeepSeek-V3", Weight: 2, Scenarios: []string{"chat", "file", "command", "browser"}},
				"strong": {Model: "deepseek-ai/DeepSeek-V3", Weight: 1, Scenarios: []string{"complex", "code"}},
				"default": {Model: "deepseek-ai/DeepSeek-V3", Weight: 1, Scenarios: []string{"chat"}},
			},
		},
		Desktop: DesktopConfig{
			Workspace: "./workspace", TempDir: "./temp", ScreenshotDir: "./screenshots",
		},
		Database: DatabaseConfig{
			Type: "mysql", AutoMigrate: true,
			SchemaPath: "docs/dev-ops/mysql/sql/01_schema.sql",
			MySQL: MySQLConfig{
				Host: "127.0.0.1", Port: 3306,
				Database: "ai_desktop_assistant",
				Username: "root", Password: "123456",
			},
		},
		Redis: RedisConfig{Enabled: false, Host: "127.0.0.1", Port: 6379},
		Logging: LoggingConfig{Level: "info"},
		Tools: ToolsConfig{
			File:       ToolToggle{Enabled: true, BaseDir: "./workspace"},
			Command:    CommandToolConfig{Enabled: true, MaxTimeout: 60, DenyList: defaultDenyRegex()},
			Screenshot: ToolToggle{Enabled: true, SaveDir: "./screenshots"},
			Browser:    ToolToggle{Enabled: true},
			Sandbox:    ToolToggle{Enabled: true},
		},
		Sandbox: SandboxConfig{Enabled: true, UseDocker: true, WorkDir: "./temp/sandbox"},
		RateLimit: RateLimitConfig{Enabled: true, PerMinute: 60},
		MCP: MCPConfig{
			Enabled: true,
			Servers: []MCPServerConfig{
				{Name: "demo", Transport: "stdio", Command: "", TimeoutSec: 30},
			},
		},
		Skills: SkillsConfig{Enabled: true, Dir: "./skills"},
	}
}

func defaultDenyRegex() []string {
	return []string{
		`(?i)\brm\s+-rf?\s+/?(\s|$)`,
		`(?i)\bformat\b`,
		`(?i)\bmkfs`,
		`(?i)\b(shutdown|reboot)\b`,
		`(?i)\bdel\s+/[fFsS]*\s*/[sS]\s*/[qQ]`,
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnv(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	applyEnv(cfg)
	normalize(cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("LLM_API_BASE"); v != "" {
		cfg.LLM.APIBase = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("LLM_USE_MOCK"); v != "" {
		cfg.LLM.UseMock = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("DESKTOP_WORKSPACE"); v != "" {
		cfg.Desktop.Workspace = v
		cfg.Tools.File.BaseDir = v
	}
	if v := os.Getenv("DB_TYPE"); v != "" {
		cfg.Database.Type = v
	}
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		cfg.Database.MySQL.Host = v
	}
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Database.MySQL.Port = p
		}
	}
	if v := os.Getenv("MYSQL_DATABASE"); v != "" {
		cfg.Database.MySQL.Database = v
	}
	if v := os.Getenv("MYSQL_USER"); v != "" {
		cfg.Database.MySQL.Username = v
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		cfg.Database.MySQL.Password = v
	}
	if v := os.Getenv("REDIS_ENABLED"); v != "" {
		cfg.Redis.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Redis.Port = p
		}
	}
	if v := os.Getenv("MCP_ENABLED"); v != "" {
		cfg.MCP.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("BROWSER_ENABLED"); v != "" {
		cfg.Tools.Browser.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("SANDBOX_DOCKER"); v != "" {
		cfg.Sandbox.UseDocker = strings.EqualFold(v, "true") || v == "1"
	}
}

func normalize(cfg *Config) {
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Agent.MaxSteps <= 0 {
		cfg.Agent.MaxSteps = 15
	}
	if cfg.Agent.TokenBudget <= 0 {
		cfg.Agent.TokenBudget = 16000
	}
	if cfg.Agent.Timeout <= 0 {
		cfg.Agent.Timeout = 120
	}
	if cfg.LLM.APIKey == "" {
		cfg.LLM.UseMock = true
	}
	if cfg.Desktop.Workspace == "" {
		cfg.Desktop.Workspace = "./workspace"
	}
	if cfg.Tools.File.BaseDir == "" {
		cfg.Tools.File.BaseDir = cfg.Desktop.Workspace
	}
	if cfg.Database.Type == "" {
		cfg.Database.Type = "mysql"
	}
	if cfg.Database.MySQL.Port <= 0 {
		cfg.Database.MySQL.Port = 3306
	}
	if cfg.Redis.Port <= 0 {
		cfg.Redis.Port = 6379
	}
	if cfg.RateLimit.PerMinute <= 0 {
		cfg.RateLimit.PerMinute = 60
	}
	if cfg.Sandbox.WorkDir == "" {
		cfg.Sandbox.WorkDir = "./temp/sandbox"
	}
	if cfg.Skills.Dir == "" {
		cfg.Skills.Dir = "./skills"
	}
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
