package tools

import (
	"context"
	"fmt"

	"github.com/ai-desktop/assistant/internal/domain/desktop/model/valobj"
	"github.com/ai-desktop/assistant/internal/domain/desktop/service"
)

type ScreenshotTool struct {
	screenshotService *service.ScreenshotService
}

func NewScreenshotTool(screenshotService *service.ScreenshotService) *ScreenshotTool {
	return &ScreenshotTool{screenshotService: screenshotService}
}

func (t *ScreenshotTool) Name() string { return "screenshot" }
func (t *ScreenshotTool) Description() string {
	return "截取当前屏幕，返回保存路径（使用 kbinani/screenshot 或系统原生命令）"
}

func (t *ScreenshotTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	format := getStringArg(args, "format")
	if format == "" {
		format = "png"
	}
	op := &valobj.ScreenshotOperation{Type: valobj.ScreenshotFullscreen, Format: format}
	result, err := t.screenshotService.TakeScreenshot(op)
	if err != nil {
		return fmt.Sprintf("截图失败: %v", err), nil
	}
	if !result.Success {
		return fmt.Sprintf("截图失败: %s", result.ErrorMsg), nil
	}
	return fmt.Sprintf("✅ 截图成功\n路径: %s\n格式: %s\n%s", result.ImagePath, result.Format, result.ErrorMsg), nil
}
