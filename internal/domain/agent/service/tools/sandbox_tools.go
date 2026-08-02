package tools

import (
	"context"
	"fmt"

	"github.com/ai-desktop/assistant/internal/domain/desktop/service"
)

type RunCodeTool struct {
	sandbox *service.SandboxService
}

func NewRunCodeTool(s *service.SandboxService) *RunCodeTool {
	return &RunCodeTool{sandbox: s}
}

func (t *RunCodeTool) Name() string { return "run_code" }
func (t *RunCodeTool) Description() string {
	return "在沙箱中执行 Python 或 JavaScript 代码。参数: language(python|javascript), code。优先 Docker 隔离，否则本地受限执行。"
}

func (t *RunCodeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	lang := getStringArg(args, "language")
	if lang == "" {
		lang = getStringArg(args, "lang")
	}
	code := getStringArg(args, "code")
	if code == "" {
		return "错误: 需要 code 参数", nil
	}
	if lang == "" {
		lang = "python"
	}
	out, err := t.sandbox.Run(ctx, lang, code)
	if err != nil {
		return fmt.Sprintf("沙箱执行失败: %v", err), nil
	}
	return out, nil
}
