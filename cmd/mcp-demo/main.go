package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// 简易 MCP stdio server（用于联调）：提供 get_time / echo / workspace_info 工具
// 协议：JSON-RPC 2.0 按行读写

type req struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type resp struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcErr     `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	sc := bufio.NewScanner(os.Stdin)
	// 放大 buffer 支持大参数
	buf := make([]byte, 0, 1024*64)
	sc.Buffer(buf, 1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r req
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		// notification 无 id
		if r.ID == nil && strings.HasPrefix(r.Method, "notifications/") {
			continue
		}
		out := handle(r)
		if out == nil {
			continue
		}
		b, _ := json.Marshal(out)
		fmt.Println(string(b))
	}
}

func handle(r req) *resp {
	switch r.Method {
	case "initialize":
		return &resp{
			JSONRPC: "2.0",
			ID:      r.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "desktop-demo-mcp",
					"version": "1.0.0",
				},
			},
		}
	case "tools/list":
		return &resp{
			JSONRPC: "2.0",
			ID:      r.ID,
			Result: map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "get_time",
						"description": "获取当前本地时间",
						"inputSchema": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
					{
						"name":        "echo",
						"description": "回显文本，用于联调 MCP 通道",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"text": map[string]interface{}{"type": "string", "description": "要回显的文本"},
							},
							"required": []string{"text"},
						},
					},
					{
						"name":        "workspace_info",
						"description": "返回当前工作目录与环境信息",
						"inputSchema": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				},
			},
		}
	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		_ = json.Unmarshal(r.Params, &p)
		text, isErr := callTool(p.Name, p.Arguments)
		return &resp{
			JSONRPC: "2.0",
			ID:      r.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": text},
				},
				"isError": isErr,
			},
		}
	case "ping":
		return &resp{JSONRPC: "2.0", ID: r.ID, Result: map[string]interface{}{}}
	default:
		if r.ID == nil {
			return nil
		}
		return &resp{
			JSONRPC: "2.0",
			ID:      r.ID,
			Error:   &rpcErr{Code: -32601, Message: "method not found: " + r.Method},
		}
	}
}

func callTool(name string, args map[string]interface{}) (string, bool) {
	switch name {
	case "get_time":
		return time.Now().Format(time.RFC3339), false
	case "echo":
		text, _ := args["text"].(string)
		if text == "" {
			return "missing text", true
		}
		return text, false
	case "workspace_info":
		wd, _ := os.Getwd()
		return fmt.Sprintf("cwd=%s\npid=%d\ntime=%s", wd, os.Getpid(), time.Now().Format(time.RFC3339)), false
	default:
		return "unknown tool: " + name, true
	}
}
