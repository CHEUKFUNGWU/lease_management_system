package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agentcore"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// Compile-time contract: a Client can be used as agentcore.StreamFunc. If the
// shape ever drifts from the loop's contract, this package stops compiling.
var _ agentcore.StreamFunc = (*Client)(nil).StreamFunc(StreamOptions{})

func fakeReadTool() agentcore.Tool {
	def := agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "store_lookup",
			Version:     "v1",
			Description: "查询门店主数据",
			Level:       agenttools.LevelRead,
			ReadOnly:    true,
			Permissions: []agenttools.Permission{{Resource: "store", Action: "read"}},
			InputSchema: json.RawMessage(`{"type":"object","properties":{"store_id":{"type":"string"}},"required":["store_id"]}`),
		},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusCompleted}, nil
		},
	}
	return agentcore.MustTool(def, agentcore.Sequential)
}

func TestStreamFuncNonStreamingRoundPassesStateThrough(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"已生成报告。"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{Provider: "deepseek", APIKey: "sk-test", BaseURL: server.URL, Model: "deepseek-v4-flash", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	state := agentcore.NewState()
	state.SetSystemPrompt("你是经营分析师。")
	state.SetMessages([]agentcore.Message{{Role: "user", Content: "帮我分析 5 月"}})
	state.SetTools([]agentcore.Tool{fakeReadTool()})

	sr, err := client.StreamFunc(StreamOptions{Temperature: 0.1, MaxTokens: 2000})(context.Background(), state)
	if err != nil {
		t.Fatalf("stream round: %v", err)
	}
	if !sr.Start {
		t.Error("round must open with Start")
	}
	if sr.End == nil || sr.End.Content != "已生成报告。" || sr.End.Role != "assistant" {
		t.Errorf("End = %#v", sr.End)
	}
	if len(sr.Updates) != 0 {
		t.Errorf("non-streaming round must send no Updates, got %d", len(sr.Updates))
	}
	if !sr.Terminate {
		t.Error("single assistant turn without tool calls must terminate")
	}
	// The state's system prompt and messages must reach the wire.
	for _, want := range []string{`"role":"system"`, "你是经营分析师", `"role":"user"`, "帮我分析 5 月", `"tools"`, "store_lookup", `"parameters":{`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("wire body missing %s in %s", want, gotBody)
		}
	}
}

func TestStreamFuncReturnsToolCallsFromModel(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_9","type":"function","function":{"name":"store_lookup","arguments":"{\"store_id\":\"s-1\"}"}}]}}]}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{Provider: "deepseek", APIKey: "sk-test", BaseURL: server.URL, Model: "deepseek-v4-flash", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	state := agentcore.NewState()
	state.SetTools([]agentcore.Tool{fakeReadTool()})

	sr, err := client.StreamFunc(StreamOptions{Temperature: 0.1})(context.Background(), state)
	if err != nil {
		t.Fatalf("stream round: %v", err)
	}
	if sr.Terminate {
		t.Error("a round with tool calls must not terminate")
	}
	if len(sr.ToolCalls) != 1 {
		t.Fatalf("tool calls = %#v", sr.ToolCalls)
	}
	tc := sr.ToolCalls[0]
	if tc.ToolName != "store_lookup" || tc.CallID != "call_9" || string(tc.Arguments) != `{"store_id":"s-1"}` {
		t.Errorf("tool call = %#v", tc)
	}
	if !strings.Contains(gotBody, `"tool_choice":"auto"`) {
		t.Errorf("tool_choice must be auto when tools are present: %s", gotBody)
	}
}

func TestStreamFuncFailsClosedWithoutAPIKey(t *testing.T) {
	client, err := NewClient(Config{Provider: "deepseek", BaseURL: "https://example.invalid", Model: "m", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	state := agentcore.NewState()
	if _, err := client.StreamFunc(StreamOptions{})(context.Background(), state); err == nil || err != ErrNotConfigured {
		t.Fatalf("missing key must fail closed with ErrNotConfigured, got %v", err)
	}
}
