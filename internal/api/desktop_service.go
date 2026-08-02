package api

import "github.com/ai-desktop/assistant/internal/api/dto"

type IDesktopService interface {
	FileOperation(req dto.FileOperationRequest) (*dto.FileOperationResponse, error)
	ExecuteCommand(req dto.CommandRequest) (*dto.CommandResponse, error)
	TakeScreenshot(req dto.ScreenshotRequest) (*dto.ScreenshotResponse, error)
	BrowserAction(req dto.BrowserRequest) (*dto.BrowserResponse, error)
}
