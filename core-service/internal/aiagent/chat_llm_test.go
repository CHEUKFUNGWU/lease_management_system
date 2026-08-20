package aiagent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/llm"
)

// W4-2 GUARD-001 evidence: the plain chat path (no file) must round-trip
// through the in-process internal/llm client against the provider-compatible
// /chat/completions endpoint — proving the replacement works, not just that
// the old AI_SERVICE_URL branch is gone.
func TestCallLLMRoundTripsThroughInProcessClient(t *testing.T) {
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"本月营收 120 万。"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`))
	}))
	defer server.Close()

	client, err := llm.NewClient(llm.Config{
		Provider: "deepseek", APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	agent := &Agent{}
	agent.SetLLMClient(client)

	answer, model, err := agent.callLLM(context.Background(),
		"你是经营分析师。", "5月营收如何？",
		[]ChatMessage{{Role: "user", Content: "5月营收如何？"}}, "zh-CN")
	if err != nil {
		t.Fatalf("callLLM: %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("LLM call path = %q, want /chat/completions", gotPath)
	}
	if answer != "本月营收 120 万。" {
		t.Errorf("answer = %q", answer)
	}
	if model != "deepseek/deepseek-v4-flash" {
		t.Errorf("model = %q", model)
	}
	// buildLLMMessages must fold system prompt (with the chat.py language
	// directive) before the history, exactly like the old /chat handler.
	for _, want := range []string{`"role":"system"`, "请用简体中文回答", `"role":"user"`, "5月营收如何？", `"temperature":0.3`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("request body missing %s in %s", want, gotBody)
		}
	}
}

func TestCallLLMAppliesLanguageDirectiveExactlyLikeChatPy(t *testing.T) {
	cases := []struct {
		language string
		want     string
	}{
		{"zh-CN", "请用简体中文回答。"},
		{"", "请用简体中文回答。"},
		{"zh-TW", "請用繁體中文回答。"},
		{"en", "Please answer in English."},
	}
	for _, tc := range cases {
		got := applyLanguageDirective("base", tc.language)
		if !strings.Contains(got, tc.want) {
			t.Errorf("language=%q: directive %q missing in %q", tc.language, tc.want, got)
		}
	}
	// buildSystemPrompt already appends the directive once; the chat path
	// appends it again — preserving the historical doubling.
	prompt := "base"
	prompt = applyLanguageDirective(prompt, "zh-CN")
	prompt = applyLanguageDirective(prompt, "zh-CN")
	if count := strings.Count(prompt, "请用简体中文回答。"); count != 2 {
		t.Errorf("directive doubling lost: count = %d in %q", count, prompt)
	}
}

func TestCallLLMWithToolsRoutesFirstToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_7","type":"function","function":{"name":"parse_contract","arguments":"{\"file_id\":\"f1\"}"}}]}}],"usage":null}`))
	}))
	defer server.Close()

	client, err := llm.NewClient(llm.Config{
		Provider: "deepseek", APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	agent := &Agent{}
	agent.SetLLMClient(client)

	_, model, calls, err := agent.callLLMWithTools(context.Background(),
		"用户上传了文件，请决定调用哪个工具。", "请解析文件", nil, "zh-CN",
		[]map[string]interface{}{{"type": "function", "function": map[string]interface{}{"name": "parse_contract"}}})
	if err != nil {
		t.Fatalf("callLLMWithTools: %v", err)
	}
	if model != "deepseek/deepseek-v4-flash" {
		t.Errorf("model = %q", model)
	}
	if len(calls) != 1 {
		t.Fatalf("tool calls = %#v", calls)
	}
	if calls[0].Tool != "parse_contract" || calls[0].InputSummary != `{"file_id":"f1"}` || calls[0].Skill != "LLM Function Calling" {
		t.Errorf("tool call mapping = %#v", calls[0])
	}
}

func TestCallLLMWithToolsFailsClosedWithoutKey(t *testing.T) {
	client, _ := llm.NewClient(llm.Config{
		Provider: "deepseek", BaseURL: "http://127.0.0.1:1", Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	})
	agent := &Agent{}
	agent.SetLLMClient(client)

	_, _, _, err := agent.callLLMWithTools(context.Background(), "sp", "um", nil, "zh-CN", nil)
	if err == nil {
		t.Fatal("missing API key must fail closed")
	}
}

func TestExecuteChatRequestLLMFailureReturnsFallbackNotSilentSuccess(t *testing.T) {
	// No key configured: the in-process client must fail and the chat handler
	// must surface the fallback answer with an error — never a fabricated
	// answer pretending the LLM answered.
	client, _ := llm.NewClient(llm.Config{
		Provider: "deepseek", BaseURL: "http://127.0.0.1:1", Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	})
	agent := &Agent{}
	agent.SetLLMClient(client)

	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "user-1", Role: "editor", Permissions: []string{"ai_chat:use"}},
		RunID:     "run-1",
	})
	resp, err := agent.executeChatRequest(ctx, "unused", Request{
		Message: "本月经营怎么样", Language: "zh-CN",
	}, "le-001", "user-1", "editor", nil, nil, agent.toolRuntime)
	if err == nil {
		t.Fatal("LLM failure must propagate an error, not a silent success")
	}
	if !strings.Contains(resp.Answer, "AI 服务暂不可用") {
		t.Errorf("fallback answer = %q", resp.Answer)
	}
	if resp.Model != "fallback" {
		t.Errorf("fallback model = %q, want fallback", resp.Model)
	}
}