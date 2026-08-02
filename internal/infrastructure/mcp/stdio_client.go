package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/mcp/model/entity"
)

// StdioClient MCP stdio 传输客户端
type StdioClient struct {
	name       string
	command    string
	args       []string
	env        map[string]string
	timeout    time.Duration
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	mu         sync.Mutex
	pending    map[int64]chan rpcResponse
	nextID     atomic.Int64
	closed     atomic.Bool
	toolsCache []entity.ToolDef
}

func NewStdioClient(cfg entity.ServerConfig) *StdioClient {
	to := time.Duration(cfg.TimeoutSec) * time.Second
	if to <= 0 {
		to = 60 * time.Second
	}
	return &StdioClient{
		name:    cfg.Name,
		command: cfg.Command,
		args:    cfg.Args,
		env:     cfg.Env,
		timeout: to,
		pending: make(map[int64]chan rpcResponse),
	}
}

func (c *StdioClient) Name() string { return c.name }

func (c *StdioClient) Initialize(ctx context.Context) error {
	if c.command == "" {
		return fmt.Errorf("mcp stdio %s: empty command", c.name)
	}
	// 注意：子进程生命周期与 Initialize 的 ctx 解耦，避免 bootstrap timeout cancel 杀进程
	c.cmd = exec.Command(c.command, c.args...)
	if len(c.env) > 0 {
		env := os.Environ()
		for k, v := range c.env {
			env = append(env, k+"="+v)
		}
		c.cmd.Env = env
	}
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	// stderr 单独管道，避免阻塞子进程
	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start mcp %s: %w", c.name, err)
	}
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			log.Printf("[mcp:%s:stderr] %s\n", c.name, sc.Text())
		}
	}()
	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)
	go c.readLoop()

	// initialize
	params := initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]interface{}{},
		ClientInfo:      clientInfo{Name: "ai-desktop-assistant", Version: "1.0.0"},
	}
	var result json.RawMessage
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		_ = c.Close()
		return fmt.Errorf("mcp initialize %s: %w", c.name, err)
	}
	// notifications/initialized
	_ = c.notify(ctx, "notifications/initialized", map[string]interface{}{})
	log.Printf("[mcp] stdio client %s initialized (pid=%d)\n", c.name, c.cmd.Process.Pid)

	// warm tools
	if tools, err := c.ListTools(ctx); err == nil {
		c.toolsCache = tools
		log.Printf("[mcp] %s tools=%d\n", c.name, len(tools))
	}
	return nil
}

func (c *StdioClient) ListTools(ctx context.Context) ([]entity.ToolDef, error) {
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

func (c *StdioClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
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

func (c *StdioClient) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	return nil
}

func (c *StdioClient) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	if c.closed.Load() {
		return fmt.Errorf("mcp client closed")
	}
	id := c.nextID.Add(1)
	ch := make(chan rpcResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	if err := c.write(req); err != nil {
		return err
	}

	timeout := c.timeout
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d > 0 && d < timeout {
			timeout = d
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("mcp call timeout: %s", method)
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && len(resp.Result) > 0 {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	}
}

func (c *StdioClient) notify(_ context.Context, method string, params interface{}) error {
	req := rpcRequest{JSONRPC: "2.0", Method: method, Params: params}
	return c.write(req)
}

func (c *StdioClient) write(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stdin == nil {
		return fmt.Errorf("stdin closed")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = c.stdin.Write(b)
	return err
}

func (c *StdioClient) readLoop() {
	for {
		if c.closed.Load() {
			return
		}
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			if !c.closed.Load() {
				log.Printf("[mcp] %s read closed: %v\n", c.name, err)
			}
			return
		}
		line = trimSpace(line)
		if len(line) == 0 {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			log.Printf("[mcp] %s bad json: %s\n", c.name, string(line))
			continue
		}
		// notification without id
		if resp.ID == nil {
			continue
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
}

func toInt64(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		i, _ := t.Int64()
		return i
	default:
		var n int64
		b, _ := json.Marshal(v)
		_ = json.Unmarshal(b, &n)
		return n
	}
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t' || b[0] == '\r' || b[0] == '\n') {
		b = b[1:]
	}
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '\t' || b[len(b)-1] == '\r' || b[len(b)-1] == '\n') {
		b = b[:len(b)-1]
	}
	return b
}
