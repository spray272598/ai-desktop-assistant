package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ai-desktop/assistant/internal/domain/agent/adapter/port"
)

// OpenAIGateway OpenAI 兼容 API（SiliconFlow / DeepSeek / OpenAI）
type OpenAIGateway struct {
	apiKey  string
	apiBase string
	model   string
	client  *http.Client
}

func NewOpenAIGateway(apiKey, apiBase, model string) *OpenAIGateway {
	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}
	apiBase = strings.TrimRight(apiBase, "/")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAIGateway{
		apiKey:  apiKey,
		apiBase: apiBase,
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type chatCompletionReq struct {
	Model       string        `json:"model"`
	Messages    []chatMsg     `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatMsg struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type chatCompletionResp struct {
	Choices []struct {
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (g *OpenAIGateway) Generate(ctx context.Context, req *port.ChatRequest) (*port.ChatResponse, error) {
	msgs := make([]chatMsg, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		msgs = append(msgs, chatMsg{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		// 保留 tool 角色（ChatML）；部分网关要求 tool 消息带 tool_call_id
		msg := chatMsg{Role: role, Content: m.Content, Name: m.Name}
		if role == "tool" {
			msg.ToolCallID = m.ToolCallID
			if msg.ToolCallID == "" {
				// 兼容无 id 的历史消息：用 name 派生，避免空 tool_call_id
				msg.ToolCallID = "call_" + m.Name
			}
		}
		msgs = append(msgs, msg)
	}

	temp := req.Temperature
	if temp == 0 {
		temp = 0.3
	}
	body := chatCompletionReq{
		Model:       g.model,
		Messages:    msgs,
		Temperature: temp,
		MaxTokens:   req.MaxTokens,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.apiBase+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed chatCompletionResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse llm response: %w; body=%s", err, truncate(string(data), 300))
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("llm error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("llm empty choices; status=%d body=%s", resp.StatusCode, truncate(string(data), 300))
	}

	msg := parsed.Choices[0].Message
	out := &port.ChatResponse{
		Content:          msg.Content,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		TotalTokens:      parsed.Usage.TotalTokens,
		Raw:              string(data),
	}

	for _, tc := range msg.ToolCalls {
		args := map[string]interface{}{}
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		out.ToolCalls = append(out.ToolCalls, port.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}
	return out, nil
}

func (g *OpenAIGateway) ClassifyIntent(ctx context.Context, input string, contextHint string) (string, float64, map[string]string, error) {
	sys := `你是意图分类器。只输出 JSON，不要 markdown。
格式: {"intent":"INTENT","confidence":0.0,"entities":{}}
可选 intent: READ_FILE,WRITE_FILE,LIST_FILES,DELETE_FILE,CREATE_DIR,RUN_COMMAND,RUN_SCRIPT,START_APP,SCREENSHOT,OPEN_URL,SYSTEM_INFO,TASK_PLAN,CHAT,UNKNOWN
entities 可含 path/command/app/scriptPath。`

	user := "用户消息: " + input
	if contextHint != "" {
		user += "\n上下文: " + contextHint
	}

	resp, err := g.Generate(ctx, &port.ChatRequest{
		SystemPrompt: sys,
		Messages:     []port.ChatMessage{{Role: "user", Content: user}},
		Temperature:  0,
		MaxTokens:    256,
	})
	if err != nil {
		return "", 0, nil, err
	}

	content := strings.TrimSpace(resp.Content)
	// 提取 JSON
	if i := strings.Index(content, "{"); i >= 0 {
		if j := strings.LastIndex(content, "}"); j > i {
			content = content[i : j+1]
		}
	}
	var parsed struct {
		Intent     string            `json:"intent"`
		Confidence float64           `json:"confidence"`
		Entities   map[string]string `json:"entities"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return "CHAT", 0.5, nil, nil
	}
	return parsed.Intent, parsed.Confidence, parsed.Entities, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
