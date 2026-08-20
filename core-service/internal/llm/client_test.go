package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func chatTestServer(t *testing.T, handler func(http.ResponseWriter, *http.Request), body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
		_, _ = io.WriteString(w, body)
	}))
}

func TestChatHitsChatCompletionsWithBearerAuth(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	server := chatTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
	}, `{"choices":[{"index":0,"message":{"role":"assistant","content":"你好"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	defer server.Close()

	client, err := NewClient(Config{Provider: "deepseek", APIKey: "sk-test", BaseURL: server.URL, Model: "deepseek-v4-flash", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	result, err := client.Chat(context.Background(), ChatRequest{
		Messages:   []Message{{Role: "user", Content: "hi"}},
		Temp:       0.3,
		MaxTokens:  2000,
		ToolChoice: "auto",
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("auth = %q", gotAuth)
	}
	for _, want := range []string{`"model":"deepseek-v4-flash"`, `"temperature":0.3`, `"max_tokens":2000`, `"tool_choice":"auto"`, `"role":"user"`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("request body missing %s in %s", want, gotBody)
		}
	}
	if result.Answer != "你好" || result.Model != "deepseek/deepseek-v4-flash" {
		t.Errorf("result = %#v", result)
	}
	if result.Usage == nil || result.Usage.TotalTokens == nil || *result.Usage.TotalTokens != 2 {
		t.Errorf("usage not parsed: %#v", result.Usage)
	}
}

func TestChatSendsToolsWhenProvided(t *testing.T) {
	var gotBody string
	server := chatTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
	}, `{"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"parse_contract","arguments":"{}"}}]}}]}`)
	defer server.Close()

	client, err := NewClient(Config{Provider: "deepseek", APIKey: "sk-test", BaseURL: server.URL, Model: "m", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	result, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "解析文件"}},
		Temp:     0.1,
		Tools:    []map[string]any{{"type": "function", "function": map[string]any{"name": "parse_contract", "description": "解析合同"}}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if !strings.Contains(gotBody, `"tools"`) || !strings.Contains(gotBody, "parse_contract") {
		t.Errorf("request body must carry tools: %s", gotBody)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool calls = %#v", result.ToolCalls)
	}
	tc := result.ToolCalls[0]
	if tc.ID != "call_1" || tc.Type != "function" || tc.Function.Name != "parse_contract" || tc.Function.Arguments != "{}" {
		t.Errorf("tool call = %#v", tc)
	}
}

func TestChatNon2xxFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":"boom"}`)
	}))
	defer server.Close()
	client, err := NewClient(Config{Provider: "deepseek", APIKey: "sk-test", BaseURL: server.URL, Model: "m", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil || !strings.Contains(err.Error(), "returned 500") {
		t.Fatalf("non-2xx must fail closed, got %v", err)
	}
}

func TestChatEmptyChoicesFailsClosed(t *testing.T) {
	server := chatTestServer(t, func(w http.ResponseWriter, r *http.Request) {}, `{"choices":[]}`)
	defer server.Close()
	client, err := NewClient(Config{Provider: "deepseek", APIKey: "sk-test", BaseURL: server.URL, Model: "m", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("empty choices must fail closed, got %v", err)
	}
}

func TestChatContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := NewClient(Config{Provider: "deepseek", APIKey: "sk-test", BaseURL: "http://127.0.0.1:1", Model: "m", PricingVersion: "unconfigured"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.Chat(ctx, ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}}); err == nil {
		t.Fatal("cancelled context must fail")
	}
}