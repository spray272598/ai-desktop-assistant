package port

import "github.com/ai-desktop/assistant/internal/domain/desktop/model/valobj"

type IFilePort interface {
	ReadFile(path string) (*valobj.FileResult, error)
	WriteFile(path, content string, mode string) (*valobj.FileResult, error)
	ListFiles(path string) (*valobj.FileResult, error)
	DeleteFile(path string) (*valobj.FileResult, error)
	CreatePath(path string, isDir bool) (*valobj.FileResult, error)
}

type ICommandPort interface {
	Execute(cmd string, args []string, workDir string, timeout int) (*valobj.CommandResult, error)
	ExecuteScript(script string, workDir string, timeout int) (*valobj.CommandResult, error)
}

type IScreenshotPort interface {
	CaptureFullscreen() (*valobj.ScreenshotResult, error)
	CaptureWindow(windowName string) (*valobj.ScreenshotResult, error)
	CaptureRegion(x, y, width, height int) (*valobj.ScreenshotResult, error)
}
