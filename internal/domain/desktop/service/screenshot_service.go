package service

import (
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kbinani/screenshot"

	"github.com/ai-desktop/assistant/internal/domain/desktop/model/valobj"
)

// ScreenshotService 多后端截图：kbinani/screenshot → 平台原生命令
type ScreenshotService struct {
	saveDir string
}

func NewScreenshotService(saveDir string) *ScreenshotService {
	if saveDir == "" {
		saveDir = "./screenshots"
	}
	abs, _ := filepath.Abs(saveDir)
	if abs != "" {
		saveDir = abs
	}
	_ = os.MkdirAll(saveDir, 0755)
	return &ScreenshotService{saveDir: saveDir}
}

func (s *ScreenshotService) TakeScreenshot(op *valobj.ScreenshotOperation) (*valobj.ScreenshotResult, error) {
	if op == nil {
		op = &valobj.ScreenshotOperation{}
	}
	format := op.Format
	if format == "" {
		format = "png"
	}
	outputPath := filepath.Join(s.saveDir, fmt.Sprintf("screenshot_%d.%s", time.Now().UnixMilli(), format))

	// 1) pure-go 主路径
	if err := s.captureKbinani(outputPath); err == nil {
		return s.readResult(outputPath, format)
	}

	// 2) 平台命令
	var err error
	switch runtime.GOOS {
	case "windows":
		err = s.captureWindowsPS(outputPath)
	case "darwin":
		err = exec.Command("screencapture", "-x", outputPath).Run()
	default:
		err = s.captureLinux(outputPath)
	}
	if err != nil {
		return &valobj.ScreenshotResult{
			Success:  false,
			ErrorMsg: fmt.Sprintf("截图失败 (%s): %v。容器环境通常无法截取宿主机屏幕。", runtime.GOOS, err),
		}, nil
	}
	return s.readResult(outputPath, format)
}

func (s *ScreenshotService) captureKbinani(outputPath string) error {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return fmt.Errorf("no active display")
	}
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return err
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func (s *ScreenshotService) captureWindowsPS(outputPath string) error {
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
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Run()
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
	return fmt.Errorf("no screenshot backend (kbinani/gnome-screenshot/scrot/import)")
}

func (s *ScreenshotService) readResult(path, format string) (*valobj.ScreenshotResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &valobj.ScreenshotResult{Success: false, ErrorMsg: "读取截图失败: " + err.Error()}, nil
	}
	// base64 可选；为控制体积只返回路径 + 大小
	return &valobj.ScreenshotResult{
		Success:   true,
		ImagePath: path,
		Format:    format,
		Width:     0,
		Height:    0,
		ImageData: "", // 大图不塞 base64，避免上下文爆炸
		ErrorMsg:  fmt.Sprintf("bytes=%d", len(data)),
	}, nil
}
