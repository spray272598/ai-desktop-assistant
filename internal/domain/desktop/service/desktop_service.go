package service

import (
	"github.com/ai-desktop/assistant/internal/domain/desktop/adapter/port"
	"github.com/ai-desktop/assistant/internal/domain/desktop/model/valobj"
)

// DesktopService 聚合桌面能力，实现 Port 接口
type DesktopService struct {
	files       *FileService
	commands    *CommandService
	screenshots *ScreenshotService
}

func NewDesktopService(files *FileService, commands *CommandService, screenshots *ScreenshotService) *DesktopService {
	return &DesktopService{files: files, commands: commands, screenshots: screenshots}
}

func (s *DesktopService) ReadFile(path string) (*valobj.FileResult, error) {
	return s.files.ExecuteFileOperation(&valobj.FileOperation{Type: valobj.FileOpRead, Path: path})
}

func (s *DesktopService) WriteFile(path, content, mode string) (*valobj.FileResult, error) {
	return s.files.ExecuteFileOperation(&valobj.FileOperation{Type: valobj.FileOpWrite, Path: path, Content: content, Mode: mode})
}

func (s *DesktopService) ListFiles(path string) (*valobj.FileResult, error) {
	return s.files.ExecuteFileOperation(&valobj.FileOperation{Type: valobj.FileOpList, Path: path})
}

func (s *DesktopService) DeleteFile(path string) (*valobj.FileResult, error) {
	return s.files.ExecuteFileOperation(&valobj.FileOperation{Type: valobj.FileOpDelete, Path: path})
}

func (s *DesktopService) CreatePath(path string, isDir bool) (*valobj.FileResult, error) {
	if isDir {
		return s.files.ExecuteFileOperation(&valobj.FileOperation{Type: valobj.FileOpWrite, Path: path, Mode: "mkdir"})
	}
	return s.files.ExecuteFileOperation(&valobj.FileOperation{Type: valobj.FileOpCreate, Path: path})
}

func (s *DesktopService) Execute(cmd string, args []string, workDir string, timeout int) (*valobj.CommandResult, error) {
	return s.commands.ExecuteCommand(&valobj.CommandOperation{
		Type: valobj.CmdShell, Command: cmd, Args: args, WorkDir: workDir, Timeout: timeout,
	})
}

func (s *DesktopService) ExecuteScript(script string, workDir string, timeout int) (*valobj.CommandResult, error) {
	return s.commands.ExecuteCommand(&valobj.CommandOperation{
		Type: valobj.CmdScript, Command: script, WorkDir: workDir, Timeout: timeout,
	})
}

func (s *DesktopService) CaptureFullscreen() (*valobj.ScreenshotResult, error) {
	return s.screenshots.TakeScreenshot(&valobj.ScreenshotOperation{Type: valobj.ScreenshotFullscreen})
}

func (s *DesktopService) CaptureWindow(windowName string) (*valobj.ScreenshotResult, error) {
	return s.screenshots.TakeScreenshot(&valobj.ScreenshotOperation{Type: valobj.ScreenshotWindow, Window: windowName})
}

func (s *DesktopService) CaptureRegion(x, y, width, height int) (*valobj.ScreenshotResult, error) {
	return s.screenshots.TakeScreenshot(&valobj.ScreenshotOperation{
		Type:   valobj.ScreenshotRegion,
		Region: [4]int{x, y, width, height},
	})
}

var (
	_ port.IFilePort       = (*DesktopService)(nil)
	_ port.ICommandPort    = (*DesktopService)(nil)
	_ port.IScreenshotPort = (*DesktopService)(nil)
)
