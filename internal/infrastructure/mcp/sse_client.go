package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

// SSEClient MCP SSE 传输（GET /sse 拿 endpoint，POST 消息）
type SSEClient struct {
	name        string
	baseURL     string
	timeout     time.Duration
	httpClient  *http.Client
	messageURL  string
	sessionID   string
	mu          sync.Mutex
	pending     map[int64]chan rpcResponse
	nextID      atomic.Int64
	closed      atomic.Bool
	cancelRead  context.CancelFunc
	toolsCache  []entity.ToolDef
	headers     map[string]string
}

func NewSSEClient(cfg entity.ServerConfig) *SSEClient {
	to := time.Duration(cfg.TimeoutSec) * time.Second
	if to <= 0 {
		to = 60 * time.Second
	}
	return &SSEClient{
		name:    cfg.Name,
		baseURL: strings.TrimRight(cfg.URL, "/"),
		timeout: to,
		httpClient: &http.Client{
			Timeout: 0, // SSE long-lived
		},
		pending: make(map[int64]chan rpcResponse),
		headers: map[string]string{},
	}
}

func (c *SSEClient) Name() string { return c.name }

func (c *SSEClient) Initialize(ctx context.Context) error {
	if c.baseURL == "" {
		return fmt.Errorf("mcp sse %s: empty url", c.name)
	}

	// 尝试 streamable HTTP（POST 直接对话）
	if err := c.tryStreamableHTTP(ctx); err == nil {
		log.Printf("[mcp] sse/http client %s initialized (streamable http)\n", c.name)
		return c.warmTools(ctx)
	}

	// 经典 SSE: GET baseURL 或 baseURL/sse
	sseURL := c.baseURL
	if !strings.Contains(strings.ToLower(sseURL), "sse") {
		sseURL = c.baseURL + "/sse"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mcp sse connect %s: %w", c.name, err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return fmt.Errorf("mcp sse status %d: %s", resp.StatusCode, string(body))
	}

	readCtx, cancel := context.WithCancel(context.Background())
	c.cancelRead = cancel
	go c.readSSE(readCtx, resp.Body)

	// 等待 endpoint 事件
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		ok := c.messageURL != ""
		c.mu.Unlock()
		if ok {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	c.mu.Lock()
	msgURL := c.messageURL
	c.mu.Unlock()
	if msgURL == "" {
		// 某些实现 message 路径就是 base + /message
		c.messageURL = c.baseURL + "/message"
	}

	params := initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]interface{}{},
		ClientInfo:      clientInfo{Name: "ai-desktop-assistant", Version: "1.0.0"},
	}
	var result json.RawMessage
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		_ = c.Close()
		return fmt.Errorf("mcp sse initialize %s: %w", c.name, err)
	}
	_ = c.notify(ctx, "notifications/initialized", map[string]interface{}{})
	log.Printf("[mcp] sse client %s initialized endpoint=%s\n", c.name, c.messageURL)
	return c.warmTools(ctx)
}

func (c *SSEClient) tryStreamableHTTP(ctx context.Context) error {
	// 直接 POST initialize 到 baseURL
	params := initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]interface{}{},
		ClientInfo:      clientInfo{Name: "ai-desktop-assistant", Version: "1.0.0"},
	}
	id := c.nextID.Add(1)
	reqBody := rpcRequest{JSONRPC: "2.0", ID: id, Method: "initialize", Params: params}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	// session id header
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}
	data, _ := io.ReadAll(resp.Body)
	// 可能是 JSON 或 SSE
	if strings.Contains(resp.Header.Get("Content-Type"), "event-stream") {
		// 解析 data: 行
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				var r rpcResponse
				if json.Unmarshal([]byte(payload), &r) == nil && r.Error == nil {
					c.messageURL = c.baseURL // streamable 复用 base
					_ = c.notify(ctx, "notifications/initialized", map[string]interface{}{})
					return nil
				}
			}
		}
		return fmt.Errorf("no result in sse body")
	}
	var r rpcResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	if r.Error != nil {
		return r.Error
	}
	c.messageURL = c.baseURL
	_ = c.notify(ctx, "notifications/initialized", map[string]interface{}{})
	return nil
}

func (c *SSEClient) warmTools(ctx context.Context) error {
	tools, err := c.ListTools(ctx)
	if err != nil {
		log.Printf("[mcp] %s list tools warn: %v\n", c.name, err)
		return nil
	}
	c.toolsCache = tools
	log.Printf("[mcp] %s tools=%d\n", c.name, len(tools))
	return nil
}

func (c *SSEClient) ListTools(ctx context.Context) ([]entity.ToolDef, error) {
	var res toolsListResult
	if err := c.call(ctx, "tools/list", map[string]interface{}{}, &res); err != nil {
		if len(c.toolsCache) > 0 {
			return c.toolsCache, nil
		}
		return nil, err
	}
	out := make([]entity.ToolDef, 0, len(res.Tools))
	for _, t := range res.Tools {
		out = append(out, entity.ToolDef{
			Name: t.Name, Description: t.Description,
			InputSchema: t.InputSchema, ServerName: c.name,
		})
	}
	c.toolsCache = out
	return out, nil
}

func (c *SSEClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	var res toolsCallResult
	if err := c.call(ctx, "tools/call", toolsCallParams{Name: name, Arguments: args}, &res); err != nil {
		return "", err
	}
	text := extractText(&res)
	if res.IsError {
		return text, fmt.Errorf("mcp tool error: %s", text)
	}
	return text, nil
}

func (c *SSEClient) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	if c.cancelRead != nil {
		c.cancelRead()
	}
	return nil
}

func (c *SSEClient) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	id := c.nextID.Add(1)
	// streamable HTTP: 同步 POST 拿响应
	c.mu.Lock()
	msgURL := c.messageURL
	c.mu.Unlock()
	if msgURL == "" {
		msgURL = c.baseURL
	}

	reqBody := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	// 若有 SSE 读循环，也可挂 pending；同时用 HTTP 响应
	ch := make(chan rpcResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msgURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}

	data, _ := io.ReadAll(resp.Body)
	if len(data) > 0 {
		// 直接 JSON
		var r rpcResponse
		if json.Unmarshal(data, &r) == nil && (r.Result != nil || r.Error != nil || r.ID != nil) {
			if r.Error != nil {
				return r.Error
			}
			if result != nil && len(r.Result) > 0 {
				return json.Unmarshal(r.Result, result)
			}
			return nil
		}
		// SSE 片段
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var r rpcResponse
			if json.Unmarshal([]byte(payload), &r) == nil {
				if r.Error != nil {
					return r.Error
				}
				if result != nil && len(r.Result) > 0 {
					return json.Unmarshal(r.Result, result)
				}
				return nil
			}
		}
	}

	// 等待 SSE 推送
	timer := time.NewTimer(c.timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("mcp sse call timeout: %s", method)
	case r := <-ch:
		if r.Error != nil {
			return r.Error
		}
		if result != nil && len(r.Result) > 0 {
			return json.Unmarshal(r.Result, result)
		}
		return nil
	}
}

func (c *SSEClient) notify(ctx context.Context, method string, params interface{}) error {
	c.mu.Lock()
	msgURL := c.messageURL
	c.mu.Unlock()
	if msgURL == "" {
		msgURL = c.baseURL
	}
	reqBody := rpcRequest{JSONRPC: "2.0", Method: method, Params: params}
	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msgURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (c *SSEClient) readSSE(ctx context.Context, body io.ReadCloser) {
	defer body.Close()
	reader := bufio.NewReader(body)
	var eventType string
	var dataBuf strings.Builder
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// dispatch
			if dataBuf.Len() > 0 {
				c.handleSSEEvent(eventType, dataBuf.String())
			}
			eventType = ""
			dataBuf.Reset()
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
}

func (c *SSEClient) handleSSEEvent(eventType, data string) {
	if eventType == "endpoint" || strings.Contains(data, "message") && !strings.HasPrefix(data, "{") {
		// endpoint 路径
		ep := strings.TrimSpace(data)
		if strings.HasPrefix(ep, "http") {
			c.mu.Lock()
			c.messageURL = ep
			c.mu.Unlock()
			return
		}
		// 相对路径
		base := c.baseURL
		if idx := strings.Index(base, "://"); idx >= 0 {
			// 取 host 根
			rest := base[idx+3:]
			if slash := strings.Index(rest, "/"); slash >= 0 {
				base = base[:idx+3+slash]
			}
		}
		if !strings.HasPrefix(ep, "/") {
			ep = "/" + ep
		}
		c.mu.Lock()
		c.messageURL = base + ep
		c.mu.Unlock()
		return
	}
	var resp rpcResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return
	}
	if resp.ID == nil {
		return
	}
	id := toInt64(resp.ID)
	c.mu.Lock()
	ch := c.pending[id]
	c.mu.Unlock()
	if ch != nil {
		select {
		case ch <- resp:
		default:
		}
	}
}
