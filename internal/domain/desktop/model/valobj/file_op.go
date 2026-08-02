package valobj

type FileOperationType string

const (
	FileOpRead   FileOperationType = "READ"
	FileOpWrite  FileOperationType = "WRITE"
	FileOpList   FileOperationType = "LIST"
	FileOpDelete FileOperationType = "DELETE"
	FileOpCreate FileOperationType = "CREATE"
)

type FileOperation struct {
	Type    FileOperationType
	Path    string
	Content string
	Mode    string
}

type FileResult struct {
	Success  bool
	Path     string
	Content  string
	Items    []FileInfo
	Size     int64
	ErrorMsg string
}

type FileInfo struct {
	Name     string
	Path     string
	IsDir    bool
	Size     int64
	Modified string
}
