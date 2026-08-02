package provider

import (
	"os"
	"runtime"
)

// EnvProvider 系统环境上下文
type EnvProvider struct {
	defaultWorkDir string
}

func NewEnvProvider(workDir string) *EnvProvider {
	return &EnvProvider{defaultWorkDir: workDir}
}

func (p *EnvProvider) Name() string  { return "env" }
func (p *EnvProvider) Order() int    { return 10 }
func (p *EnvProvider) Enabled() bool { return true }

func (p *EnvProvider) Provide(sessionID, userID, workingDir string, _ []map[string]interface{}) map[string]interface{} {
	dir := workingDir
	if dir == "" {
		dir = p.defaultWorkDir
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}
	hostname, _ := os.Hostname()
	return map[string]interface{}{
		"osInfo":          runtime.GOOS + "/" + runtime.GOARCH,
		"currentUser":     userID,
		"currentDirectory": dir,
		"serverInfo":      hostname,
		"sessionId":       sessionID,
	}
}
