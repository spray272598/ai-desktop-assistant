package dto

type FileOperationRequest struct {
	Action  string `json:"action"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

type FileOperationResponse struct {
	Success   bool                   `json:"success"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

type CommandRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	WorkDir string   `json:"workDir,omitempty"`
	Timeout int      `json:"timeout,omitempty"`
}

type CommandResponse struct {
	Success   bool   `json:"success"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	ExitCode  int    `json:"exitCode"`
	Duration  int64  `json:"duration"`
	Timestamp int64  `json:"timestamp"`
}

type ScreenshotRequest struct {
	Region string `json:"region,omitempty"`
	Window string `json:"window,omitempty"`
}

type ScreenshotResponse struct {
	Success   bool   `json:"success"`
	ImageData string `json:"imageData,omitempty"`
	ImagePath string `json:"imagePath,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

type BrowserRequest struct {
	Action string                 `json:"action"`
	Url    string                 `json:"url,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
}

type BrowserResponse struct {
	Success   bool                   `json:"success"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}
