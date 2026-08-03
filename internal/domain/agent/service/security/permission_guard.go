package security

import (
	"fmt"
	"regexp"
	"sync"
	"time"
)

// Action 权限裁决
type Action string

const (
	ActionAllow   Action = "allow"
	ActionDeny    Action = "deny"
	ActionConfirm Action = "confirm"
)

// Decision 权限决策结果
type Decision struct {
	Action  Action `json:"action"`
	RuleID  string `json:"ruleId,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Tool    string `json:"tool"`
	Summary string `json:"summary"`
}

type rule struct {
	id     string
	re     *regexp.Regexp
	action Action
	reason string
}

// PermissionGuard 权限守卫（对标 walicode PermissionGuard）
type PermissionGuard struct {
	mu           sync.RWMutex
	sessionAllow map[string]map[string]bool // sessionID -> tool:sig -> allowed
	pending      map[string]*PendingConfirm
	awaiting     map[string]*AwaitingResume // sessionID -> 待恢复执行
	denyRules    []rule
	confirmRules []rule
	// 工具级策略
	writeTools   map[string]bool
	commandTools map[string]bool
	deleteTools  map[string]bool
	// 会话连续拒绝断路
	denyStreak   map[string]int
	circuitLimit int
}

// PendingConfirm 待用户确认的操作
type PendingConfirm struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"sessionId"`
	Tool      string                 `json:"tool"`
	Args      map[string]interface{} `json:"args"`
	Reason    string                 `json:"reason"`
	RuleID    string                 `json:"ruleId"`
	CreatedAt time.Time              `json:"createdAt"`
}

// AwaitingResume 已批准、等待「继续」时立刻执行的工具调用
type AwaitingResume struct {
	SessionID string                 `json:"sessionId"`
	Tool      string                 `json:"tool"`
	Args      map[string]interface{} `json:"args"`
	Reason    string                 `json:"reason,omitempty"`
	PermID    string                 `json:"permId,omitempty"`
	Ready     bool                   `json:"ready"` // true=已批准可执行
}

func NewPermissionGuard() *PermissionGuard {
	g := &PermissionGuard{
		sessionAllow: make(map[string]map[string]bool),
		pending:      make(map[string]*PendingConfirm),
		awaiting:     make(map[string]*AwaitingResume),
		writeTools:   map[string]bool{"write_file": true},
		commandTools: map[string]bool{"run_command": true, "run_script": true},
		deleteTools:  map[string]bool{"delete_file": true},
		denyStreak:   make(map[string]int),
		circuitLimit: 5,
	}
	g.initRules()
	return g
}

func (g *PermissionGuard) initRules() {
	// DENY
	denies := []struct {
		id, pattern, reason string
	}{
		{"rm_rf_root", `(?i)\brm\s+-rf?\s+/?(\s|$)`, "递归删除根目录"},
		{"rm_rf_star", `(?i)\brm\s+-rf?\s+\*`, "递归删除通配"},
		{"format_disk", `(?i)\b(format|mkfs)\b`, "格式化磁盘"},
		{"shutdown", `(?i)\b(shutdown|poweroff|halt|reboot)\b`, "关机/重启"},
		{"dd_disk", `(?i)\bdd\s+if=`, "磁盘写入 dd"},
		{"fork_bomb", `:\(\)\s*\{\s*:|:&`, "Fork 炸弹"},
		{"del_s_q", `(?i)\bdel\s+/[fFsS]*\s*/[sS]\s*/[qQ]`, "Windows 强制递归删除"},
		{"git_force_main", `(?i)\bgit\s+push\s+(-f|--force).*(main|master)`, "强制推送主分支"},
	}
	for _, d := range denies {
		g.denyRules = append(g.denyRules, rule{id: d.id, re: regexp.MustCompile(d.pattern), action: ActionDeny, reason: d.reason})
	}
	// CONFIRM
	confirms := []struct {
		id, pattern, reason string
	}{
		{"rm_any", `(?i)\brm\s+`, "删除文件/目录"},
		{"del_any", `(?i)\b(del|rmdir|Remove-Item)\b`, "删除文件/目录"},
		{"chmod", `(?i)\bchmod\b`, "修改权限"},
		{"git_push", `(?i)\bgit\s+push\b`, "Git 推送"},
		{"git_reset_hard", `(?i)\bgit\s+reset\s+--hard\b`, "Git 硬重置"},
		{"pip_install", `(?i)\bpip3?\s+install\b`, "安装 Python 包"},
		{"npm_g", `(?i)\bnpm\s+install\s+-g\b`, "全局 npm 安装"},
		{"docker_rm", `(?i)\bdocker\s+(rm|rmi|stop|volume\s+rm)\b`, "Docker 破坏性操作"},
		{"curl_pipe", `(?i)\b(curl|wget).*\|\s*(sh|bash)`, "管道执行远程脚本"},
		{"drop_db", `(?i)\bDROP\s+(DATABASE|TABLE)\b`, "删除数据库/表"},
	}
	for _, c := range confirms {
		g.confirmRules = append(g.confirmRules, rule{id: c.id, re: regexp.MustCompile(c.pattern), action: ActionConfirm, reason: c.reason})
	}
}

// Check 检查工具调用权限
func (g *PermissionGuard) Check(sessionID, tool string, args map[string]interface{}) Decision {
	summary := toolSummary(tool, args)

	// 断路器
	g.mu.RLock()
	streak := g.denyStreak[sessionID]
	g.mu.RUnlock()
	if streak >= g.circuitLimit {
		return Decision{Action: ActionDeny, RuleID: "circuit_breaker", Reason: "会话连续触发安全拒绝，已熔断", Tool: tool, Summary: summary}
	}

	// 会话级已批准签名
	sig := toolSig(tool, args)
	g.mu.RLock()
	if g.sessionAllow[sessionID] != nil && g.sessionAllow[sessionID][sig] {
		g.mu.RUnlock()
		return Decision{Action: ActionAllow, Tool: tool, Summary: summary}
	}
	// session 级 auto_all
	if g.sessionAllow[sessionID] != nil && g.sessionAllow[sessionID]["*"] {
		g.mu.RUnlock()
		return Decision{Action: ActionAllow, Tool: tool, Summary: summary, Reason: "session auto-approve"}
	}
	g.mu.RUnlock()

	// 读类工具默认放行
	if tool == "read_file" || tool == "list_files" || tool == "screenshot" ||
		tool == "get_time" || tool == "echo" || tool == "workspace_info" {
		return Decision{Action: ActionAllow, Tool: tool, Summary: summary}
	}

	// 命令内容检查
	cmd := extractCommand(tool, args)
	if cmd != "" {
		for _, r := range g.denyRules {
			if r.re.MatchString(cmd) {
				g.incDeny(sessionID)
				return Decision{Action: ActionDeny, RuleID: r.id, Reason: r.reason, Tool: tool, Summary: summary}
			}
		}
		for _, r := range g.confirmRules {
			if r.re.MatchString(cmd) {
				return Decision{Action: ActionConfirm, RuleID: r.id, Reason: r.reason, Tool: tool, Summary: summary}
			}
		}
	}

	// 写/删工具默认确认
	if g.writeTools[tool] || g.deleteTools[tool] {
		return Decision{Action: ActionConfirm, RuleID: "write_or_delete", Reason: "写/删操作需确认", Tool: tool, Summary: summary}
	}
	// 任意 shell 命令默认确认（安全优先）
	if g.commandTools[tool] {
		return Decision{Action: ActionConfirm, RuleID: "shell_command", Reason: "执行命令需确认", Tool: tool, Summary: summary}
	}

	return Decision{Action: ActionAllow, Tool: tool, Summary: summary}
}

// CreatePending 创建待确认项，并登记 awaiting（未批准）
func (g *PermissionGuard) CreatePending(sessionID, tool string, args map[string]interface{}, d Decision) *PendingConfirm {
	id := fmt.Sprintf("perm-%d", time.Now().UnixNano())
	p := &PendingConfirm{
		ID: id, SessionID: sessionID, Tool: tool, Args: args,
		Reason: d.Reason, RuleID: d.RuleID, CreatedAt: time.Now(),
	}
	g.mu.Lock()
	g.pending[id] = p
	g.awaiting[sessionID] = &AwaitingResume{
		SessionID: sessionID, Tool: tool, Args: args,
		Reason: d.Reason, PermID: id, Ready: false,
	}
	g.mu.Unlock()
	return p
}

// Approve 批准待确认操作（once 或 session），并标记 awaiting.Ready
func (g *PermissionGuard) Approve(id string, scope string) (*PendingConfirm, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.pending[id]
	if !ok {
		return nil, fmt.Errorf("pending permission not found: %s", id)
	}
	delete(g.pending, id)
	if g.sessionAllow[p.SessionID] == nil {
		g.sessionAllow[p.SessionID] = make(map[string]bool)
	}
	if scope == "session" || scope == "always" {
		g.sessionAllow[p.SessionID]["*"] = true
	} else {
		g.sessionAllow[p.SessionID][toolSig(p.Tool, p.Args)] = true
	}
	g.denyStreak[p.SessionID] = 0
	// 标记可恢复执行
	if a, ok := g.awaiting[p.SessionID]; ok && a.PermID == id {
		a.Ready = true
	} else {
		g.awaiting[p.SessionID] = &AwaitingResume{
			SessionID: p.SessionID, Tool: p.Tool, Args: p.Args,
			Reason: p.Reason, PermID: id, Ready: true,
		}
	}
	return p, nil
}

// Reject 拒绝
func (g *PermissionGuard) Reject(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.pending[id]
	if !ok {
		return fmt.Errorf("pending permission not found")
	}
	delete(g.pending, id)
	if a, ok := g.awaiting[p.SessionID]; ok && a.PermID == id {
		delete(g.awaiting, p.SessionID)
	}
	g.denyStreak[p.SessionID]++
	return nil
}

// PeekAwaiting 查看会话待恢复项（不消费）
func (g *PermissionGuard) PeekAwaiting(sessionID string) *AwaitingResume {
	g.mu.RLock()
	defer g.mu.RUnlock()
	a := g.awaiting[sessionID]
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

// TakeReadyResume 取出已批准的恢复项并清除；未批准返回 nil
func (g *PermissionGuard) TakeReadyResume(sessionID string) *AwaitingResume {
	g.mu.Lock()
	defer g.mu.Unlock()
	a, ok := g.awaiting[sessionID]
	if !ok || a == nil || !a.Ready {
		return nil
	}
	delete(g.awaiting, sessionID)
	cp := *a
	return &cp
}

// ClearAwaiting 清除会话恢复状态
func (g *PermissionGuard) ClearAwaiting(sessionID string) {
	g.mu.Lock()
	delete(g.awaiting, sessionID)
	g.mu.Unlock()
}

func (g *PermissionGuard) GetPending(id string) *PendingConfirm {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.pending[id]
}

func (g *PermissionGuard) ListPending(sessionID string) []*PendingConfirm {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []*PendingConfirm
	for _, p := range g.pending {
		if sessionID == "" || p.SessionID == sessionID {
			out = append(out, p)
		}
	}
	return out
}

func (g *PermissionGuard) incDeny(sessionID string) {
	g.mu.Lock()
	g.denyStreak[sessionID]++
	g.mu.Unlock()
}

func extractCommand(tool string, args map[string]interface{}) string {
	if args == nil {
		return ""
	}
	switch tool {
	case "run_command":
		if c, ok := args["command"].(string); ok {
			return c
		}
	case "run_script":
		if c, ok := args["scriptPath"].(string); ok {
			return c
		}
	case "write_file", "delete_file", "read_file":
		if p, ok := args["path"].(string); ok {
			return p
		}
	}
	return fmt.Sprint(args)
}

func toolSummary(tool string, args map[string]interface{}) string {
	if args == nil {
		return tool
	}
	switch tool {
	case "run_command":
		return tool + ": " + fmt.Sprint(args["command"])
	case "write_file", "delete_file", "read_file":
		return tool + ": " + fmt.Sprint(args["path"])
	default:
		return tool
	}
}

func toolSig(tool string, args map[string]interface{}) string {
	return tool + "|" + extractCommand(tool, args)
}
