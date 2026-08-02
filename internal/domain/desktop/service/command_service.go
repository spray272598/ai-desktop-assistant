package service

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/desktop/model/valobj"
)

// CommandService 带安全检查的命令执行
type CommandService struct {
	denyList   []string
	maxTimeout int
}

func NewCommandService() *CommandService {
	return NewCommandServiceWithPolicy(60, []string{
		"rm -rf /", "format c:", "mkfs", "shutdown", "reboot",
		"del /f /s /q", ":(){", "dd if=", "mkfs.", "diskpart",
	})
}

func NewCommandServiceWithPolicy(maxTimeout int, denyList []string) *CommandService {
	if maxTimeout <= 0 {
		maxTimeout = 60
	}
	return &CommandService{denyList: denyList, maxTimeout: maxTimeout}
}

func (s *CommandService) ExecuteCommand(op *valobj.CommandOperation) (*valobj.CommandResult, error) {
	if op == nil || strings.TrimSpace(op.Command) == "" {
		return &valobj.CommandResult{Success: false, Stderr: "empty command", ExitCode: -1}, nil
	}

	if err := s.checkDenied(op.Command, op.Args); err != nil {
		return &valobj.CommandResult{Success: false, Stderr: err.Error(), ExitCode: -1}, nil
	}

	timeout := time.Duration(op.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxT := time.Duration(s.maxTimeout) * time.Second
	if timeout > maxT {
		timeout = maxT
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch op.Type {
	case valobj.CmdScript:
		// Command = interpreter, Args = script + args
		cmd = exec.CommandContext(ctx, op.Command, op.Args...)
	default:
		// Shell 执行：Windows 用 cmd /c，其他用 sh -c
		full := op.Command
		if len(op.Args) > 0 {
			full = op.Command + " " + strings.Join(op.Args, " ")
		}
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/c", full)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", full)
		}
	}

	if op.WorkDir != "" {
		cmd.Dir = op.WorkDir
	}
	if len(op.Env) > 0 {
		envVars := make([]string, 0, len(op.Env))
		for k, v := range op.Env {
			envVars = append(envVars, k+"="+v)
		}
		cmd.Env = append(cmd.Environ(), envVars...)
	}

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start).Milliseconds()

	result := &valobj.CommandResult{
		Success:  err == nil,
		Stdout:   string(output),
		Duration: duration,
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Stderr = err.Error()
		}
		if ctx.Err() == context.DeadlineExceeded {
			result.Stderr = "command timeout"
		}
	}
	return result, nil
}

func (s *CommandService) checkDenied(command string, args []string) error {
	full := strings.ToLower(command + " " + strings.Join(args, " "))
	for _, d := range s.denyList {
		if d == "" {
			continue
		}
		if strings.Contains(full, strings.ToLower(d)) {
			return fmt.Errorf("危险命令已拦截: 匹配规则 %q", d)
		}
	}
	return nil
}
