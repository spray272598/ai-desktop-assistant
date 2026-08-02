package tools

import (
	"context"
	"encoding/base64"
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

func (t *ScreenshotTool) Name() string        { return "screenshot" }
func (t *ScreenshotTool) Description() string { return "截取当前屏幕截图，返回截图文件路径和base64编码" }

func (t *ScreenshotTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	format := getStringArg(args, "format")
	if format == "" {
		format = "png"
	}

	op := &valobj.ScreenshotOperation{
		Type:   valobj.ScreenshotFullscreen,
		Format: format,
	}

	result, err := t.screenshotService.TakeScreenshot(op)
	if err != nil {
		return fmt.Sprintf("截图失败: %v", err), nil
	}

	if !result.Success {
		return fmt.Sprintf("截图失败: %s", result.ErrorMsg), nil
	}

	var msg string
	msg += fmt.Sprintf("✅ 截图成功!\n")
	msg += fmt.Sprintf("📁 文件路径: %s\n", result.ImagePath)
	msg += fmt.Sprintf("📐 尺寸: %dx%d\n", result.Width, result.Height)
	msg += fmt.Sprintf("🎨 格式: %s\n", result.Format)

	if result.ImageData != "" {
		compressed := result.ImageData
		if len(compressed) > 100 {
			compressed = result.ImageData[:50] + "..." + result.ImageData[len(result.ImageData)-20:]
		}
		msg += fmt.Sprintf("\n🖼️ Base64预览: %s\n", compressed)
		msg += fmt.Sprintf("📊 数据大小: %d bytes\n", len(base64.StdEncoding.EncodeToString([]byte(result.ImageData))))
	}

	return msg, nil
}
