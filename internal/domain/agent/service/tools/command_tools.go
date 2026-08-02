package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-desktop/assistant/internal/domain/desktop/model/valobj"
	"github.com/ai-desktop/assistant/internal/domain/desktop/service"
)

type RunCommandTool struct {
	commandService *service.CommandService
}

func NewRunCommandTool(commandService *service.CommandService) *RunCommandTool {
	return &RunCommandTool{commandService: commandService}
}

func (t *RunCommandTool) Name() string        { return "run_command" }
func (t *RunCommandTool) Description() string { return "执行Shell命令，返回命令的标准输出和错误输出" }

func (t *RunCommandTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	command := getStringArg(args, "command")
	if command == "" {
		return "错误: 必须提供命令 command", nil
	}

	workDir := getStringArg(args, "workDir")
	timeout := getIntArg(args, "timeout", 30)

	op := &valobj.CommandOperation{
		Type:    valobj.CmdShell,
		Command: command,
		WorkDir: workDir,
		Timeout: timeout,
	}

	result, err := t.commandService.ExecuteCommand(op)
	if err != nil {
		return fmt.Sprintf("命令执行失败: %v", err), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("执行结果 (退出码: %d, 耗时: %dms):\n", result.ExitCode, result.Duration))
	sb.WriteString(strings.Repeat("-", 40) + "\n")

	if result.Stdout != "" {
		if len(result.Stdout) > 3000 {
			sb.WriteString(result.Stdout[:3000] + "\n... (输出已截断)")
		} else {
			sb.WriteString(result.Stdout)
		}
		sb.WriteString("\n")
	}

	if result.Stderr != "" {
		sb.WriteString("\n错误输出:\n")
		sb.WriteString(result.Stderr)
		sb.WriteString("\n")
	}

	if !result.Success {
		sb.WriteString("\n⚠️ 命令执行失败\n")
	} else {
		sb.WriteString("\n✅ 命令执行成功\n")
	}

	return sb.String(), nil
}

type RunScriptTool struct {
	commandService *service.CommandService
}

func NewRunScriptTool(commandService *service.CommandService) *RunScriptTool {
	return &RunScriptTool{commandService: commandService}
}

func (t *RunScriptTool) Name() string        { return "run_script" }
func (t *RunScriptTool) Description() string { return "执行脚本文件（支持Python、Shell等）" }

func (t *RunScriptTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	scriptPath := getStringArg(args, "scriptPath")
	interpreter := getStringArg(args, "interpreter")

	if scriptPath == "" {
		return "错误: 必须提供脚本路径 scriptPath", nil
	}

	if interpreter == "" {
		switch {
		case strings.HasSuffix(scriptPath, ".py"):
			interpreter = "python"
		case strings.HasSuffix(scriptPath, ".sh"):
			interpreter = "bash"
		case strings.HasSuffix(scriptPath, ".bat") || strings.HasSuffix(scriptPath, ".cmd"):
			interpreter = "cmd"
		default:
			interpreter = "bash"
		}
	}

	args_list := []string{scriptPath}
	if extraArgs, ok := args["args"]; ok {
		switch v := extraArgs.(type) {
		case []interface{}:
			for _, a := range v {
				args_list = append(args_list, fmt.Sprintf("%v", a))
			}
		case string:
			for _, a := range strings.Split(v, " ") {
				if a != "" {
					args_list = append(args_list, a)
				}
			}
		}
	}

	op := &valobj.CommandOperation{
		Type:    valobj.CmdScript,
		Command: interpreter,
		Args:    args_list,
		Timeout: getIntArg(args, "timeout", 60),
	}

	result, err := t.commandService.ExecuteCommand(op)
	if err != nil {
		return fmt.Sprintf("脚本执行失败: %v", err), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("脚本执行结果 (退出码: %d, 耗时: %dms):\n", result.ExitCode, result.Duration))
	sb.WriteString(strings.Repeat("-", 40) + "\n")

	if result.Stdout != "" {
		content := result.Stdout
		if len(content) > 3000 {
			content = content[:3000] + "\n... (输出已截断)"
		}
		sb.WriteString(content)
	}

	if result.Success {
		sb.WriteString("\n✅ 脚本执行成功\n")
	} else {
		sb.WriteString("\n⚠️ 脚本执行失败\n")
		if result.Stderr != "" {
			sb.WriteString("\n错误: " + result.Stderr + "\n")
		}
	}

	return sb.String(), nil
}

func getJsonValue(args map[string]interface{}, key string) (interface{}, bool) {
	val, ok := args[key]
	if !ok {
		return nil, false
	}
	return val, true
}
