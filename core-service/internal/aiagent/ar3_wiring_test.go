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
// Its canned prompt carries the SAME SHAPE the real Assemble returns —
// history ending with the current user message. The AF1-b review finding was
// exactly that a stub without the current message proved nothing: the shapes
// diverged, so the wiring test could not see the triple-fire.
type stubAssembler struct {
	gotKey   agentcontext.ContextKey
	gotTurn  contextassembler.Turn
	prompt   contextassembler.Prompt
	err      error
	calledN  int
	turnDefs [][]contextassembler.ToolDef // ToolDefs per call, in order
}

func (s *stubAssembler) Assemble(_ context.Context, key agentcontext.ContextKey, turn contextassembler.Turn) (contextassembler.Prompt, error) {
	s.calledN++
	s.gotKey = key
	s.gotTurn = turn
	s.turnDefs = append(s.turnDefs, turn.ToolDefs)
	if s.err != nil {
		return contextassembler.Prompt{}, s.err
	}
	return s.prompt, nil
}

const (
	wiredEntity  = "11111111-2222-4333-8444-555555555555"
	wiredSession = "0b8f6a2e-1111-4111-8111-111111111111"
	currentMsg   = "当前问题"
)

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

// newWiredAgent pins a stub assembler whose prompt mirrors the real shape:
// prior history plus the current user message as the final entry.
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
			{Ref: "m3", Role: "user", Kind: contextassembler.KindText, Text: currentMsg},
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

func wiredRequest() Request {
	return Request{
		SessionID: wiredSession,
		Message:   currentMsg, Language: "zh-CN",
		History: []ChatMessage{
			{Role: "user", Content: "不该被发送的旧消息"}, // must be replaced by the assembled slice
		},
	}
}

// wireOccurrences counts how many times the current message text appears in
// the captured LLM request body — the executable form of the AF1-b invariant.
func wireOccurrences(t *testing.T, body, text string) int {
	t.Helper()
	var parsed struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("parse request body: %v", err)
	}
	n := 0
	for _, m := range parsed.Messages {
		if m.Content == text {
			n++
		}
	}
	return n
}

// ── 验收 1（AF1-b 核心）：真 assembler 跑完，wire 上当前消息恰好一次 ────────

func TestWiredConversationCarriesCurrentMessageExactlyOnce(t *testing.T) {
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

	// REAL assembler over a REAL-shaped stored history: prior rounds plus the
	// trigger message the runtime persisted before execution. No Turn.Messages.
	hist := &storedHistoryFake{rows: []contextassembler.Message{
		{Ref: "m0", Role: "user", Kind: contextassembler.KindText, Text: "上一轮的问题"},
		{Ref: "m1", Role: "assistant", Kind: contextassembler.KindText, Text: "上一轮的回答"},
		{Ref: "m2", Role: "user", Kind: contextassembler.KindText, Text: currentMsg}, // trigger row
	}}
	assembler := contextassembler.NewAssembler(hist)
	if err := contextassembler.RegisterBudget(assembler, "deepseek-v4-flash", contextassembler.BudgetSpec{Window: 100000, ReserveTokens: 4096}); err != nil {
		t.Fatal(err)
	}
	agent.SetContextAssembler(assembler)

	resp, err := agent.executeChatRequest(chatExecContext(), "", wiredRequest(),
		wiredEntity, "user-1", "editor", nil, nil, agent.toolRuntime)
	if err != nil {
		t.Fatalf("executeChatRequest: %v", err)
	}
	if resp.Answer != "好的。" {
		t.Errorf("answer = %q", resp.Answer)
	}

	// Parse the wire and assert the full message sequence: system first,
	// stored rows in order, current message EXACTLY ONCE at the end.
	var parsed struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if len(parsed.Messages) != 4 {
		t.Fatalf("wire messages = %d, want 4 (system + 3 stored): %+v", len(parsed.Messages), parsed.Messages)
	}
	if parsed.Messages[0].Role != "system" || !strings.Contains(parsed.Messages[0].Content, "请用简体中文回答") {
		t.Errorf("first wire message = %#v; want system with language directive", parsed.Messages[0])
	}
	wantContents := []string{"上一轮的问题", "上一轮的回答", currentMsg}
	for i, want := range wantContents {
		got := parsed.Messages[i+1]
		if got.Role != roleOf(want) || got.Content != want {
			t.Errorf("wire[%d] = %q/%q; want %q in conversation order", i+1, got.Role, got.Content, want)
		}
	}
	if n := wireOccurrences(t, body, currentMsg); n != 1 {
		t.Fatalf("current message appears %d times on the wire; want exactly once (review probe reproduced 3)", n)
	}
}

func roleOf(content string) string {
	switch content {
	case "上一轮的回答":
		return "assistant"
	default:
		return "user"
	}
}

// storedHistoryFake stands in for PgHistorySource: rows already in stored
// order, trigger message last.
type storedHistoryFake struct {
	rows []contextassembler.Message
}

func (f *storedHistoryFake) Read(_ context.Context, _ agentcontext.ContextKey) ([]contextassembler.Message, error) {
	out := make([]contextassembler.Message, len(f.rows))
	copy(out, f.rows)
	return out, nil
}

var _ contextassembler.HistorySource = (*storedHistoryFake)(nil)

// ── 不变式防御：装配结果不以当前 user 消息结尾时响亮拒绝 ─────────────────────

func TestAssembleRefusesPromptNotEndingWithCurrentUserMessage(t *testing.T) {
	var body string
	server := newLLMTestServer(t, &body)
	agent, stub := newWiredAgent(t, server.URL)
	stub.prompt.Messages = stub.prompt.Messages[:2] // drops the trailing user row

	_, err := agent.executeChatRequest(chatExecContext(), "", wiredRequest(),
		wiredEntity, "user-1", "editor", nil, nil, agent.toolRuntime)
	if err == nil || !strings.Contains(err.Error(), "does not end with the current user message") {
		t.Fatalf("err = %v; want the trailing-user invariant refusal", err)
	}
	if body != "" {
		t.Errorf("no LLM call may happen after an invariant refusal")
	}
}

// ── 接线等价性：stub 形状下 assembled 历史替换 raw history ──────────────────

func TestExecuteChatRequestUsesAssembledHistoryWhenWired(t *testing.T) {
	var body string
	server := newLLMTestServer(t, &body)
	agent, stub := newWiredAgent(t, server.URL)

	resp, err := agent.executeChatRequest(chatExecContext(), "", wiredRequest(),
		wiredEntity, "user-1", "editor", nil, nil, agent.toolRuntime)
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
	if stub.gotKey.SessionID() != wiredSession ||
		stub.gotKey.UserID() != "user-1" ||
		stub.gotKey.LegalEntityID() != wiredEntity {
		t.Errorf("key dims mismatched: session=%s user=%s entity=%s",
			stub.gotKey.SessionID(), stub.gotKey.UserID(), stub.gotKey.LegalEntityID())
	}
	// Plain chat sends no tool schemas on the wire → ToolDefs stays empty
	// (adjudication: count what the provider counts).
	if len(stub.gotTurn.ToolDefs) != 0 {
		t.Errorf("plain-chat turn carried %d tool defs; want 0", len(stub.gotTurn.ToolDefs))
	}
	if stub.gotTurn.Model != "deepseek-v4-flash" {
		t.Errorf("turn model = %q; budget lookup keys on the wire model name", stub.gotTurn.Model)
	}
	// The raw req.History never reaches the wire under the assembler.
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
	if !strings.Contains(body, "原始历史") || !strings.Contains(body, "当前问题") {
		t.Errorf("flag off must keep the legacy history+current fold; body = %s", body)
	}
	if n := wireOccurrences(t, body, "当前问题"); n != 1 {
		t.Errorf("legacy path current-message occurrences = %d; want 1 (byte-for-byte legacy)", n)
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
	if _, err := agent.executeChatRequest(chatExecContext(), "", wiredRequest(),
		wiredEntity, "user-1", "editor", emit, nil, agent.toolRuntime); err != nil {
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
	_ = body
}

// ── Assemble 失败必须响亮地停下，不得静默退回无界 prompt ────────────────────

func TestExecuteChatRequestFailsLoudlyWhenAssembleFails(t *testing.T) {
	var body string
	server := newLLMTestServer(t, &body)
	agent, stub := newWiredAgent(t, server.URL)
	stub.err = contextassembler.ErrBudgetUnconfigured

	_, err := agent.executeChatRequest(chatExecContext(), "", wiredRequest(),
		wiredEntity, "user-1", "editor", nil, nil, agent.toolRuntime)
	if err == nil {
		t.Fatal("Assemble failure must propagate, not fall back to an unbounded prompt")
	}
	if body != "" {
		t.Errorf("no LLM call may happen after an assembly failure; body = %s", body)
	}
}

// ── 文件分诊轮：ToolDefs 按 wire 实际计入（AF4 口径在接线层的落点） ─────────

func TestFileTriageTurnCountsFileParseToolDefs(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"parse_contract","arguments":"{\"file_id\":\"f1\"}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":1,"total_tokens":11}}`))
	}))
	defer server.Close()
	agent, stub := newWiredAgent(t, server.URL)

	req := wiredRequest()
	req.FileID = "f1"
	req.ObjectName = "contract.pdf"
	req.ContentType = "application/pdf"

	if _, err := agent.executeChatRequest(chatExecContext(), "", req,
		wiredEntity, "user-1", "editor", nil, nil, agent.toolRuntime); err != nil {
		t.Fatalf("executeChatRequest triage: %v", err)
	}
	if stub.calledN == 0 {
		t.Fatal("triage round must go through the assembler when wired")
	}
	// P0-A③ multi-round semantics: EVERY loop round is a wire turn carrying
	// the parse-tool schemas it selects from, and every round goes through the
	// assembler (AF4 unchanged: schemas actually sent are what gets counted).
	// The assembled-history invariant (first turn projects fileParseTools,
	// defs non-empty with name+json) stays pinned.
	if len(stub.turnDefs[0]) != len(fileParseTools) {
		t.Fatalf("first round ToolDefs = %d; want %d (the schemas riding this request)", len(stub.turnDefs[0]), len(fileParseTools))
	}
	for _, defs := range stub.turnDefs {
		if len(defs) == 0 {
			t.Fatalf("a loop round rode no tool schemas: AF4 under-counts budget")
		}
		for _, def := range defs {
			if def.Name == "" || def.JSON == "" {
				t.Fatalf("tool def not projected from fileParseTools: %+v", def)
			}
		}
	}
	_ = body
}

// ── usage 缺失时回填哨兵 0，绝不编造 ───────────────────────────────⼈─────────

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
