package service

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/desktop/model/valobj"
)

type ScreenshotService struct {
	saveDir string
}

func NewScreenshotService(saveDir string) *ScreenshotService {
	abs, _ := filepath.Abs(saveDir)
	if abs == "" {
		abs = saveDir
	}
	_ = os.MkdirAll(abs, 0755)
	return &ScreenshotService{saveDir: abs}
}

func (s *ScreenshotService) TakeScreenshot(op *valobj.ScreenshotOperation) (*valobj.ScreenshotResult, error) {
	if op == nil {
		op = &valobj.ScreenshotOperation{}
	}
	format := s.getFormat(op.Format)
	outputPath := filepath.Join(s.saveDir, fmt.Sprintf("screenshot_%d.%s", time.Now().UnixMilli(), format))

	var err error
	switch runtime.GOOS {
	case "windows":
		err = s.captureWindows(outputPath)
	case "darwin":
		err = exec.Command("screencapture", "-x", outputPath).Run()
	default:
		// Linux: 尝试 gnome-screenshot / import / scrot
		err = s.captureLinux(outputPath)
	}

	if err != nil {
		return &valobj.ScreenshotResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("截图失败 (%s): %v。容器环境通常无法截取宿主机屏幕。", runtime.GOOS, err),
		}, nil
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return &valobj.ScreenshotResult{Success: false, ErrorMsg: "读取截图失败: " + err.Error()}, nil
	}

	return &valobj.ScreenshotResult{
		Success:   true,
		ImageData: base64.StdEncoding.EncodeToString(data),
		ImagePath: outputPath,
		Format:    format,
	}, nil
}

func (s *ScreenshotService) captureWindows(outputPath string) error {
	// 使用 PowerShell + .NET
	ps := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$screen = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$bmp = New-Object System.Drawing.Bitmap $screen.Width, $screen.Height
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($screen.Location, [System.Drawing.Point]::Empty, $screen.Size)
$bmp.Save('%s')
$g.Dispose(); $bmp.Dispose()
`, filepath.ToSlash(outputPath))
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	return cmd.Run()
}

func (s *ScreenshotService) captureLinux(outputPath string) error {
	if err := exec.Command("gnome-screenshot", "-f", outputPath).Run(); err == nil {
		return nil
	}
	if err := exec.Command("scrot", outputPath).Run(); err == nil {
		return nil
	}
	if err := exec.Command("import", "-window", "root", outputPath).Run(); err == nil {
		return nil
	}
	return fmt.Errorf("no screenshot tool available (gnome-screenshot/scrot/import)")
}

func (s *ScreenshotService) getFormat(format string) string {
	if format == "" {
		return "png"
	}
	return format
}
