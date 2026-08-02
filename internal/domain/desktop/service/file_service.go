package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/desktop/model/valobj"
)

// FileService 工作区受限的文件服务
type FileService struct {
	baseDir string
}

func NewFileService(baseDir string) *FileService {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		abs = baseDir
	}
	_ = os.MkdirAll(abs, 0755)
	return &FileService{baseDir: abs}
}

func (s *FileService) BaseDir() string {
	return s.baseDir
}

func (s *FileService) ExecuteFileOperation(op *valobj.FileOperation) (*valobj.FileResult, error) {
	absPath, err := s.resolvePath(op.Path)
	if err != nil {
		return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
	}

	switch op.Type {
	case valobj.FileOpRead:
		return s.readFile(absPath)
	case valobj.FileOpWrite:
		if op.Mode == "mkdir" {
			return s.createDir(absPath)
		}
		return s.writeFile(absPath, op.Content, op.Mode)
	case valobj.FileOpList:
		return s.listFiles(absPath)
	case valobj.FileOpDelete:
		return s.deleteFile(absPath)
	case valobj.FileOpCreate:
		return s.createFile(absPath)
	default:
		return &valobj.FileResult{Success: false, ErrorMsg: "unsupported operation"}, nil
	}
}

// resolvePath 将路径解析到 baseDir 内，禁止路径穿越
func (s *FileService) resolvePath(path string) (string, error) {
	if path == "" || path == "." {
		return s.baseDir, nil
	}
	// 清理
	clean := filepath.Clean(path)
	var abs string
	if filepath.IsAbs(clean) {
		abs = clean
	} else {
		abs = filepath.Join(s.baseDir, clean)
	}
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	// 确保在 baseDir 下
	base := s.baseDir
	if !strings.HasPrefix(strings.ToLower(abs), strings.ToLower(base)) {
		// Windows 下再尝试带分隔符比较
		rel, err := filepath.Rel(base, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("路径越界: 仅允许访问工作区 %s", base)
		}
	}
	return abs, nil
}

func (s *FileService) readFile(path string) (*valobj.FileResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
	}
	info, _ := os.Stat(path)
	size := int64(len(data))
	if info != nil {
		size = info.Size()
	}
	return &valobj.FileResult{
		Success: true,
		Path:    path,
		Content: string(data),
		Size:    size,
	}, nil
}

func (s *FileService) writeFile(path, content, mode string) (*valobj.FileResult, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
	}
	m := os.FileMode(0644)
	if mode == "append" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, m)
		if err != nil {
			return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
		}
		defer f.Close()
		if _, err = f.WriteString(content); err != nil {
			return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
		}
	} else {
		if err := os.WriteFile(path, []byte(content), m); err != nil {
			return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
		}
	}
	info, _ := os.Stat(path)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return &valobj.FileResult{Success: true, Path: path, Size: size}, nil
}

func (s *FileService) listFiles(path string) (*valobj.FileResult, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
	}
	var items []valobj.FileInfo
	for _, entry := range entries {
		info, _ := entry.Info()
		modTime := ""
		size := int64(0)
		if info != nil {
			modTime = info.ModTime().Format(time.RFC3339)
			size = info.Size()
		}
		items = append(items, valobj.FileInfo{
			Name:     entry.Name(),
			Path:     filepath.Join(path, entry.Name()),
			IsDir:    entry.IsDir(),
			Size:     size,
			Modified: modTime,
		})
	}
	return &valobj.FileResult{Success: true, Path: path, Items: items}, nil
}

func (s *FileService) deleteFile(path string) (*valobj.FileResult, error) {
	// 禁止删除 baseDir 本身
	if filepath.Clean(path) == filepath.Clean(s.baseDir) {
		return &valobj.FileResult{Success: false, ErrorMsg: "禁止删除工作区根目录"}, nil
	}
	if err := os.RemoveAll(path); err != nil {
		return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
	}
	return &valobj.FileResult{Success: true, Path: path}, nil
}

func (s *FileService) createFile(path string) (*valobj.FileResult, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
	}
	_ = f.Close()
	return &valobj.FileResult{Success: true, Path: path}, nil
}

func (s *FileService) createDir(path string) (*valobj.FileResult, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return &valobj.FileResult{Success: false, ErrorMsg: err.Error()}, nil
	}
	return &valobj.FileResult{Success: true, Path: path}, nil
}
