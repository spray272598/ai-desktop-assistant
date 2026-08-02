package entity

type DesktopCapabilityEntity struct {
	ID          string
	Name        string
	Category    string // file/command/screenshot/browser/system
	Description string
	Enabled     bool
	Config      map[string]interface{}
}

type FileSystemEntity struct {
	Path     string
	Type     string // file/directory
	Size     int64
	Content  string
	Modified string
	Readonly bool
}

type AppEntity struct {
	Name    string
	Path    string
	Version string
	Running bool
	PID     int
}
