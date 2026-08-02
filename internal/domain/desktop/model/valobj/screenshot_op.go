package valobj

type ScreenshotType string

const (
	ScreenshotFullscreen ScreenshotType = "FULLSCREEN"
	ScreenshotWindow     ScreenshotType = "WINDOW"
	ScreenshotRegion     ScreenshotType = "REGION"
)

type ScreenshotOperation struct {
	Type    ScreenshotType
	Window  string
	Region  [4]int // x, y, width, height
	Format  string // png/jpeg
	Quality int    // 1-100
}

type ScreenshotResult struct {
	Success   bool
	ImageData string // base64
	ImagePath string
	Width     int
	Height    int
	Format    string
	ErrorMsg  string
}
