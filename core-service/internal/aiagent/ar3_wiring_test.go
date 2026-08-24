package aiagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agentcontext"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/contextassembler"
	"github.com/lease-management-system/core-service/internal/llm"
)

// ── fakes ────────────────────────────────────────────────────────────────────

// stubAssembler records the Turn it received and returns a canned Prompt.
type stubAssembler struct {
	gotKey  agentcontext.ContextKey
	gotTurn contextassembler.Turn
	prompt  contextassembler.Prompt
	err     error
	calledN int
}

func (s *stubAssembler) Assemble(_ context.Context, key agentcontext.ContextKey, turn contextassembler.Turn) (contextassembler.Prompt, error) {
	s.calledN++
	s.gotKey = key
	s.gotTurn = turn
	if s.err != nil {
		return contextassembler.Prompt{}, s.err
	}
	return s.prompt, nil
}

func newLLMTestServer(t *testing.T, capturedBody *string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*capturedBody = string(b)
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"好的。"}}],"usage":{"prompt_tokens":42,"completion_tokens":5,"total_tokens":47}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func newWiredAgent(t *testing.T, serverURL string) (*Agent, *stubAssembler) {
	t.Helper()
	client, err := llm.NewClient(llm.Config{
		Provider: "deepseek", APIKey: "test-key", BaseURL: serverURL, Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	agent := &Agent{}
	agent.SetLLMClient(client)
	stub := &stubAssembler{prompt: contextassembler.Prompt{
		Messages: []contextassembler.Message{
			{Ref: "m1", Role: "user", Kind: contextassembler.KindText, Text: "早前的问题"},
			{Ref: "m2", Role: "assistant", Kind: contextassembler.KindText, Text: "早前的回答"},
		},
		Budget: 995904,
	}}
	agent.SetContextAssembler(stub)
	return agent, stub
}

func chatExecContext() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "user-1", Role: "editor", Permissions: []string{"ai_chat:use"}},
		RunID:     "run-1",
	})
}

// ── 验收 1：接线只换 history 来源，行为等价 ────────────────────────────────

func TestExecuteChatRequestUsesAssembledHistoryWhenWired(t *testing.T) {
	var body string
	server := newLLMTestServer(t, &body)
	agent, stub := newWiredAgent(t, server.URL)

	resp, err := agent.executeChatRequest(chatExecContext(), "", Request{
		SessionID: "0b8f6a2e-1111-4111-8111-111111111111",
		Message:   "当前问题", Language: "zh-CN",
		History: []ChatMessage{
			{Role: "user", Content: "不该被发送的旧消息"}, // must be replaced by the assembled slice
		},
	}, "11111111-2222-4333-8444-555555555555", "user-1", "editor", nil, nil, agent.toolRuntime)
	if err != nil {
		t.Fatalf("executeChatRequest: %v", err)
	}
	if resp.Answer != "好的。" {
		t.Errorf("answer = %q", resp.Answer)
	}
	if stub.calledN != 1 {
		t.Fatalf("Assemble called %d times, want exactly 1", stub.calledN)
	}
	// The key carries the five isolation dimensions from the request identity.
	if stub.gotKey.SessionID() != "0b8f6a2e-1111-4111-8111-111111111111" ||
		stub.gotKey.UserID() != "user-1" ||
		stub.gotKey.LegalEntityID() != "11111111-2222-4333-8444-555555555555" {
		t.Errorf("key dims mismatched: session=%s user=%s entity=%s",
			stub.gotKey.SessionID(), stub.gotKey.UserID(), stub.gotKey.LegalEntityID())
	}
	// The current message rides in Turn.Messages; systemPrompt never does.
	if len(stub.gotTurn.Messages) != 1 || stub.gotTurn.Messages[0].Text != "当前问题" {
		t.Fatalf("turn messages = %#v", stub.gotTurn.Messages)
	}
	if stub.gotTurn.Model != "deepseek-v4-flash" {
		t.Errorf("turn model = %q; budget lookup keys on the wire model name", stub.gotTurn.Model)
	}
	// The LLM request carries the ASSEMBLED history plus the current message,
	// not the raw req.History.
	if !strings.Contains(body, "早前的问题") || !strings.Contains(body, "当前问题") {
		t.Errorf("request body missing assembled history/current message: %s", body)
	}
	if strings.Contains(body, "不该被发送的旧消息") {
		t.Errorf("raw req.History leaked past the assembler: %s", body)
	}
	// GUARD-001: prove measured truth landed on the response, not just that
	// the old discard is gone.
	if resp.MeasuredTokens != 42 {
		t.Errorf("resp.MeasuredTokens = %d, want 42 (provider prompt_tokens)", resp.MeasuredTokens)
	}
}

func TestExecuteChatRequestWithoutAssemblerKeepsLegacyHistoryVerbatim(t *testing.T) {
	var body string
	server := newLLMTestServer(t, &body)
	client, err := llm.NewClient(llm.Config{
		Provider: "deepseek", APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	agent := &Agent{}
	agent.SetLLMClient(client)

	if _, err := agent.executeChatRequest(chatExecContext(), "", Request{
		Message: "当前问题", Language: "zh-CN",
		History: []ChatMessage{{Role: "user", Content: "原始历史"}},
	}, "le-001", "user-1", "editor", nil, nil, agent.toolRuntime); err != nil {
		t.Fatalf("executeChatRequest: %v", err)
	}
	if !strings.Contains(body, "原始历史") {
		t.Errorf("flag off must keep req.History verbatim; body = %s", body)
	}
}

func TestExecuteChatRequestWithoutSessionIDSkipsAssembler(t *testing.T) {
	var body string
	server := newLLMTestServer(t, &body)
	agent, stub := newWiredAgent(t, server.URL)

	if _, err := agent.executeChatRequest(chatExecContext(), "", Request{
		Message: "无会话", Language: "zh-CN",
		History: []ChatMessage{{Role: "user", Content: "原始历史"}},
	}, "le-001", "user-1", "editor", nil, nil, agent.toolRuntime); err != nil {
		t.Fatalf("executeChatRequest: %v", err)
	}
	if stub.calledN != 0 {
		t.Fatalf("Assemble must not run without a session locator (called %d)", stub.calledN)
	}
	if !strings.Contains(body, "原始历史") {
		t.Errorf("legacy history lost: %s", body)
	}
}

// ── 验收 2（事件侧）：压缩发生时 run_events 拿到可解析的 Dropped 证据 ───────

func TestExecuteChatRequestEmitsCompactionEventWithResolvableDroppedRefs(t *testing.T) {
	var body string
	server := newLLMTestServer(t, &body)
	agent, _ := newWiredAgent(t, server.URL)
	agent.ctxAssembler.(*stubAssembler).prompt.Compacted = true
	agent.ctxAssembler.(*stubAssembler).prompt.Dropped = []contextassembler.MessageRef{
		{Ref: "msg-old-1", Kind: contextassembler.KindText},
	}
	agent.ctxAssembler.(*stubAssembler).prompt.Tokens = 990000
	agent.ctxAssembler.(*stubAssembler).prompt.EstimatedTokens = 12

	var events []map[string]any
	emit := func(_ context.Context, eventType string, payload any) error {
		events = append(events, map[string]any{"type": eventType, "payload": payload})
		return nil
	}
	if _, err := agent.executeChatRequest(chatExecContext(), "", Request{
		SessionID: "0b8f6a2e-1111-4111-8111-111111111111",
		Message:   "继续问", Language: "zh-CN",
	}, "le-001", "user-1", "editor", emit, nil, agent.toolRuntime); err != nil {
		t.Fatalf("executeChatRequest: %v", err)
	}

	var compacted map[string]any
	for _, e := range events {
		if e["type"] == "context_compacted" {
			compacted = e["payload"].(map[string]any)
		}
	}
	if compacted == nil {
		t.Fatal("no context_compacted event emitted for a compacted prompt")
	}
	dropped, ok := compacted["dropped"].([]contextassembler.MessageRef)
	if !ok || len(dropped) != 1 || dropped[0].Ref != "msg-old-1" {
		t.Fatalf("dropped refs not resolvable in event payload: %#v", compacted["dropped"])
	}
	raw, _ := json.Marshal(compacted)
	if !strings.Contains(string(raw), "budget") || !strings.Contains(string(raw), "tokens") {
		t.Errorf("event payload missing observability fields: %s", raw)
	}
	// And the dropped content itself left the prompt.
	if strings.Contains(body, "不该出现") {
		t.Errorf("dropped content leaked into the prompt")
	}
}

// ── Assemble 失败必须响亮地停下，不得静默退回无界 prompt ────────────────────

func TestExecuteChatRequestFailsLoudlyWhenAssembleFails(t *testing.T) {
	var body string
	server := newLLMTestServer(t, &body)
	agent, stub := newWiredAgent(t, server.URL)
	stub.err = contextassembler.ErrBudgetUnconfigured

	_, err := agent.executeChatRequest(chatExecContext(), "", Request{
		SessionID: "0b8f6a2e-1111-4111-8111-111111111111",
		Message:   "继续问", Language: "zh-CN",
	}, "le-001", "user-1", "editor", nil, nil, agent.toolRuntime)
	if err == nil {
		t.Fatal("Assemble failure must propagate, not fall back to an unbounded prompt")
	}
	if body != "" {
		t.Errorf("no LLM call may happen after an assembly failure; body = %s", body)
	}
}

// ── usage 缺失时回填哨兵 0，绝不编造 ────────────────────────────────────────

func TestMeasuredInputTokensNilSafety(t *testing.T) {
	if got := measuredInputTokens(nil); got != 0 {
		t.Errorf("nil usage → %d, want 0", got)
	}
	if got := measuredInputTokens(&llm.UsageMetadata{}); got != 0 {
		t.Errorf("usage without counts → %d, want 0", got)
	}
	n := 7
	if got := measuredInputTokens(&llm.UsageMetadata{InputTokens: &n}); got != 7 {
		t.Errorf("measured usage → %d, want 7", got)
	}
}

// compile-time guard: the stub satisfies the port.
var _ contextassembler.Assembler = (*stubAssembler)(nil)
