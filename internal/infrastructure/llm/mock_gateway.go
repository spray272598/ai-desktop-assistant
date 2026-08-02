package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/port"
)

// MockGateway 本地演示用网关（无 API Key 时降级）
type MockGateway struct{}

func NewMockGateway() *MockGateway {
	return &MockGateway{}
}

func (g *MockGateway) Generate(ctx context.Context, req *port.ChatRequest) (*port.ChatResponse, error) {
	_ = ctx
	time.Sleep(30 * time.Millisecond)

	for i := len(req.Messages) - 1; i >= 0; i-- {
		m := req.Messages[i]
		if strings.Contains(m.Content, "[工具") && strings.Contains(m.Content, "执行结果]") {
			preview := m.Content
			if len(preview) > 600 {
				preview = preview[:600] + "\n...(截断)"
			}
			return &port.ChatResponse{
				Content:     fmt.Sprintf("📋 操作结果如下：\n\n%s", preview),
				TotalTokens: len(preview) / 2,
			}, nil
		}
	}

	input := lastUserContent(req.Messages)
	lower := strings.ToLower(input)

	switch {
	case containsAny(lower, "列出", "list", "查看目录", "ls", "目录下", "有哪些文件"):
		return toolResp("list_files", map[string]interface{}{"path": "."}), nil
	case containsAny(lower, "读取", "read", "查看文件", "打开文件", "文件内容"):
		path := extractAfter(input, []string{"读取文件", "查看文件", "打开文件", "读取", "查看", "打开"})
		if path == "" {
			path = "test.txt"
		}
		return toolResp("read_file", map[string]interface{}{"path": path}), nil
	case containsAny(lower, "写入", "write", "创建文件", "保存文件"):
		path := extractAfter(input, []string{"写入文件", "创建文件", "保存到", "写入", "保存"})
		if path == "" {
			path = "output.txt"
		}
		return toolResp("write_file", map[string]interface{}{"path": path, "content": "Hello from AI Desktop Assistant"}), nil
	case containsAny(lower, "删除", "delete", "remove"):
		path := extractAfter(input, []string{"删除文件", "删除", "delete"})
		if path == "" {
			return &port.ChatResponse{Content: "请提供要删除的文件路径。", TotalTokens: 10}, nil
		}
		return toolResp("delete_file", map[string]interface{}{"path": path}), nil
	case containsAny(lower, "执行命令", "运行命令", "执行", "run command", "cmd"):
		cmd := extractAfter(input, []string{"执行命令", "运行命令", "执行", "运行", "run"})
		if cmd == "" {
			cmd = "echo hello"
		}
		return toolResp("run_command", map[string]interface{}{"command": cmd}), nil
	case containsAny(lower, "截图", "screenshot", "截屏"):
		return toolResp("screenshot", map[string]interface{}{}), nil
	default:
		return &port.ChatResponse{
			Content: fmt.Sprintf("我收到了你的请求：%s\n\n（当前为 Mock LLM 模式。设置 LLM_API_KEY 后可使用真实模型。）\n\n你可以尝试：列出文件、读取 test.txt、执行命令 echo hello", extractUserRequest(input)),
			TotalTokens: 50,
		}, nil
	}
}

func (g *MockGateway) ClassifyIntent(_ context.Context, input string, _ string) (string, float64, map[string]string, error) {
	lower := strings.ToLower(input)
	switch {
	case containsAny(lower, "列出", "list", "目录"):
		return "LIST_FILES", 0.9, map[string]string{"path": "."}, nil
	case containsAny(lower, "读取", "read", "查看文件"):
		return "READ_FILE", 0.85, nil, nil
	case containsAny(lower, "写入", "write", "创建文件"):
		return "WRITE_FILE", 0.85, nil, nil
	case containsAny(lower, "删除", "delete"):
		return "DELETE_FILE", 0.85, nil, nil
	case containsAny(lower, "执行", "run", "命令"):
		return "RUN_COMMAND", 0.85, nil, nil
	case containsAny(lower, "截图", "screenshot"):
		return "SCREENSHOT", 0.9, nil, nil
	default:
		return "CHAT", 0.6, nil, nil
	}
}

func toolResp(name string, args map[string]interface{}) *port.ChatResponse {
	payload := map[string]interface{}{"name": name, "args": args}
	b, _ := json.Marshal(payload)
	return &port.ChatResponse{
		Content:     string(b),
		ToolCalls:   []port.ToolCall{{Name: name, Args: args}},
		TotalTokens: 20,
	}
}

func lastUserContent(msgs []port.ChatMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	if len(msgs) > 0 {
		return msgs[len(msgs)-1].Content
	}
	return ""
}

func extractUserRequest(s string) string {
	if idx := strings.Index(s, "## 用户请求"); idx >= 0 {
		return strings.TrimSpace(s[idx+len("## 用户请求"):])
	}
	return s
}

func containsAny(s string, kws ...string) bool {
	for _, kw := range kws {
		if strings.Contains(s, strings.ToLower(kw)) || strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func extractAfter(input string, kws []string) string {
	src := extractUserRequest(input)
	lowerSrc := strings.ToLower(src)
	for _, kw := range kws {
		idx := strings.Index(lowerSrc, strings.ToLower(kw))
		if idx >= 0 {
			rest := strings.TrimSpace(src[idx+len(kw):])
			rest = strings.Trim(rest, "：: \t\"'")
			if rest != "" {
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					return fields[0]
				}
			}
		}
	}
	return ""
}
