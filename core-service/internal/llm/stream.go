package llm

import (
	"context"
	"encoding/json"

	"github.com/lease-management-system/core-service/internal/agentcore"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// StreamOptions tunes a StreamFunc round.
type StreamOptions struct {
	Temperature float64
	MaxTokens   int
}

// StreamFunc adapts the client into the agentcore.StreamFunc shape so that
// agentcore.Loop can drive chat rounds (C wave). Each call builds one
// non-streaming round from the State: system prompt, conversation messages and
// registered tool descriptors are sent to the model; the response is returned
// as a Start + End round with zero or more tool calls.
//
// The current LLM path is non-streaming (Upstreams do not stream), so Updates
// stays empty; the type shape is what matters here — the C wave wires this
// into Loop without a signature change.
func (c *Client) StreamFunc(opts StreamOptions) agentcore.StreamFunc {
	return func(ctx context.Context, s *agentcore.State) (agentcore.StreamResult, error) {
		if c == nil || c.cfg.APIKey == "" {
			return agentcore.StreamResult{}, ErrNotConfigured
		}
		messages := make([]Message, 0, len(s.Messages())+1)
		if sp := s.SystemPrompt(); sp != "" {
			messages = append(messages, Message{Role: "system", Content: sp})
		}
		for _, m := range s.Messages() {
			messages = append(messages, Message{Role: m.Role, Content: m.Content})
		}
		res, err := c.Chat(ctx, ChatRequest{
			Messages:   messages,
			Temp:       opts.Temperature,
			MaxTokens:  opts.MaxTokens,
			Tools:      toolDefinitions(s.Tools()),
			ToolChoice: "auto",
		})
		if err != nil {
			return agentcore.StreamResult{}, err
		}
		out := agentcore.StreamResult{
			Start: true,
			End:   &agentcore.Message{Role: "assistant", Content: res.Answer},
		}
		for _, tc := range res.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, agenttools.ToolCall{
				CallID:    tc.ID,
				ToolName:  tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
		out.Terminate = len(out.ToolCalls) == 0
		return out, nil
	}
}

// toolDefinitions renders registered agentcore tools into the OpenAI function
// wire format. The descriptor's InputSchema is already the "parameters" body.
func toolDefinitions(tools []agentcore.Tool) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		d := t.Descriptor()
		fn := map[string]any{"name": d.Name, "description": d.Description}
		if len(d.InputSchema) > 0 {
			fn["parameters"] = json.RawMessage(d.InputSchema)
		}
		out = append(out, map[string]any{"type": "function", "function": fn})
	}
	return out
}