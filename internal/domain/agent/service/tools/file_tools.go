package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ai-desktop/assistant/internal/domain/desktop/model/valobj"
	"github.com/ai-desktop/assistant/internal/domain/desktop/service"
)

type ReadFileTool struct {
	fileService *service.FileService
}

func NewReadFileTool(fileService *service.FileService) *ReadFileTool {
	return &ReadFileTool{fileService: fileService}
}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "读取文件内容，支持指定文件路径" }

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getStringArg(args, "path")
	if path == "" {
		return "错误: 必须提供文件路径 path", nil
	}

	result, err := t.fileService.ExecuteFileOperation(&valobj.FileOperation{
		Type: valobj.FileOpRead,
		Path: path,
	})
	if err != nil {
		return fmt.Sprintf("读取文件失败: %v", err), nil
	}

	if !result.Success {
		return fmt.Sprintf("读取文件失败: %s", result.ErrorMsg), nil
	}

	if len(result.Content) > 5000 {
		return result.Content[:5000] + "\n... (内容已截断)", nil
	}
	return result.Content, nil
}

type WriteFileTool struct {
	fileService *service.FileService
}

func NewWriteFileTool(fileService *service.FileService) *WriteFileTool {
	return &WriteFileTool{fileService: fileService}
}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "写入文件，创建新文件或覆盖现有文件" }

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getStringArg(args, "path")
	content := getStringArg(args, "content")
	mode := getStringArg(args, "mode")

	if path == "" {
		return "错误: 必须提供文件路径 path", nil
	}

	result, err := t.fileService.ExecuteFileOperation(&valobj.FileOperation{
		Type:    valobj.FileOpWrite,
		Path:    path,
		Content: content,
		Mode:    mode,
	})
	if err != nil {
		return fmt.Sprintf("写入文件失败: %v", err), nil
	}

	if result.Success {
		return fmt.Sprintf("文件已成功写入: %s", path), nil
	}
	return fmt.Sprintf("写入文件失败: %s", result.ErrorMsg), nil
}

type ListFilesTool struct {
	fileService *service.FileService
}

func NewListFilesTool(fileService *service.FileService) *ListFilesTool {
	return &ListFilesTool{fileService: fileService}
}

func (t *ListFilesTool) Name() string        { return "list_files" }
func (t *ListFilesTool) Description() string { return "列出目录下的文件和子目录" }

func (t *ListFilesTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getStringArg(args, "path")
	if path == "" {
		path = "."
	}

	result, err := t.fileService.ExecuteFileOperation(&valobj.FileOperation{
		Type: valobj.FileOpList,
		Path: path,
	})
	if err != nil {
		return fmt.Sprintf("列出文件失败: %v", err), nil
	}

	if !result.Success {
		return fmt.Sprintf("列出文件失败: %s", result.ErrorMsg), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("目录: %s\n", path))
	sb.WriteString(strings.Repeat("-", 40) + "\n")
	for _, item := range result.Items {
		typePrefix := "📄"
		if item.IsDir {
			typePrefix = "📁"
		}
		sb.WriteString(fmt.Sprintf("%s %-30s %10d bytes  %s\n",
			typePrefix, item.Name, item.Size, item.Modified))
	}
	return sb.String(), nil
}

type DeleteFileTool struct {
	fileService *service.FileService
}

func NewDeleteFileTool(fileService *service.FileService) *DeleteFileTool {
	return &DeleteFileTool{fileService: fileService}
}

func (t *DeleteFileTool) Name() string        { return "delete_file" }
func (t *DeleteFileTool) Description() string { return "删除指定文件" }

func (t *DeleteFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getStringArg(args, "path")
	if path == "" {
		return "错误: 必须提供文件路径 path", nil
	}

	result, err := t.fileService.ExecuteFileOperation(&valobj.FileOperation{
		Type: valobj.FileOpDelete,
		Path: path,
	})
	if err != nil {
		return fmt.Sprintf("删除文件失败: %v", err), nil
	}

	if result.Success {
		return fmt.Sprintf("文件已删除: %s", path), nil
	}
	return fmt.Sprintf("删除文件失败: %s", result.ErrorMsg), nil
}

func getStringArg(args map[string]interface{}, key string) string {
	val, ok := args[key]
	if !ok {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%v", v)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	val, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
