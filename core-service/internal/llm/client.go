package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message is one chat message in the provider wire format.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolCallFunction is the function branch of an OpenAI-compatible tool call.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall is an OpenAI-compatible tool call returned by the model.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ChatRequest mirrors ai-service/app/routers/chat.py's ChatRequest. The
// system prompt is supplied as the first element of Messages by callers; the
// client never injects language directives of its own.
type ChatRequest struct {
	Messages       []Message
	Temp           float64
	MaxTokens      int
	Tools          []map[string]any
	ToolChoice     string
	ResponseFormat map[string]any
}

// ChatResult is the normalized chat round: answer text, display model name and
// any tool calls, plus usage metadata for the /agent-metrics page.
type ChatResult struct {
	Answer    string
	Model     string
	ToolCalls []ToolCall
	Usage     *UsageMetadata
}

// ErrNotConfigured mirrors the Python client's "LLM 配置错误" (HTTP 503).
var ErrNotConfigured = errors.New("llm: API key not configured")

// Client talks to one OpenAI-compatible chat-completions endpoint. It is
// safe for concurrent use.
type Client struct {
	cfg   Config
	httpc *http.Client
}

// NewClient validates the configuration (including the price book) and returns
// a client that will call {BaseURL}/chat/completions.
func NewClient(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Client{cfg: cfg, httpc: &http.Client{Timeout: 120 * time.Second}}, nil
}

// Config exposes the client's resolved configuration.
func (c *Client) Config() Config { return c.cfg }

// Chat performs one non-streaming chat completion round. Every real provider
// round records its outcome into the process-wide health signal (RT1-L3-B);
// ErrNotConfigured is config absence, not provider failure, and records nothing.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (ChatResult, error) {
	result, err := c.chat(ctx, req)
	switch {
	case err == nil:
		recordCallSuccess()
	case errors.Is(err, ErrNotConfigured):
		// config absence: no call was attempted, signal untouched
	default:
		recordCallFailure()
	}
	return result, err
}

func (c *Client) chat(ctx context.Context, req ChatRequest) (ChatResult, error) {
	if c == nil || strings.TrimSpace(c.cfg.APIKey) == "" {
		return ChatResult{}, ErrNotConfigured
	}
	body, err := c.buildBody(req)
	if err != nil {
		return ChatResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.httpc.Do(httpReq)
	if err != nil {
		return ChatResult{}, fmt.Errorf("llm provider unreachable: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChatResult{}, fmt.Errorf("llm provider returned %d: %s", resp.StatusCode, truncate(respBody))
	}
	return ParseChatCompletion(respBody, c.cfg)
}

func (c *Client) buildBody(req ChatRequest) ([]byte, error) {
	payload := map[string]any{
		"model":       c.cfg.Model,
		"messages":    req.Messages,
		"temperature": req.Temp,
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}
	if req.ToolChoice != "" {
		payload["tool_choice"] = req.ToolChoice
	}
	if len(req.ResponseFormat) > 0 {
		payload["response_format"] = req.ResponseFormat
	}
	return json.Marshal(payload)
}

// ParseChatCompletion normalizes a raw chat-completions response body into a
// ChatResult. It is separated from the HTTP transport so parity tests can
// drive it with a recorded fixture.
func ParseChatCompletion(body []byte, cfg Config) (ChatResult, error) {
	var raw struct {
		Choices []struct {
			Message struct {
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
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ChatResult{}, fmt.Errorf("llm: parse chat completion: %w", err)
	}
	if len(raw.Choices) == 0 {
		return ChatResult{}, fmt.Errorf("llm: chat completion returned no choices")
	}
	msg := raw.Choices[0].Message
	result := ChatResult{
		Answer: msg.Content,
		Model:  cfg.displayModelName(),
	}
	for _, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:       tc.ID,
			Type:     orDefault(tc.Type, "function"),
			Function: ToolCallFunction{Name: tc.Function.Name, Arguments: tc.Function.Arguments},
		})
	}
	if len(raw.Usage) > 0 && string(raw.Usage) != "null" {
		var usageVal any
		if err := json.Unmarshal(raw.Usage, &usageVal); err == nil {
			result.Usage = ParseUsage(cfg, usageVal)
		}
	}
	return result, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func truncate(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
