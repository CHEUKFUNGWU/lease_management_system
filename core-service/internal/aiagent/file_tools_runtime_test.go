package aiagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/llm"
)

func TestFileParseToolDescriptorsRequireDraftReview(t *testing.T) {
	agent := &Agent{}
	definitions := agent.fileParseDefinitions()
	if len(definitions) != 4 {
		t.Fatalf("file parse tool count = %d, want 4", len(definitions))
	}
	for _, definition := range definitions {
		if err := definition.Descriptor.Validate(); err != nil {
			t.Fatalf("%s descriptor invalid: %v", definition.Descriptor.Name, err)
		}
		if definition.Descriptor.Level != agenttools.LevelDraft || !definition.Descriptor.Review.Required {
			t.Fatalf("%s must be a reviewable draft tool: %#v", definition.Descriptor.Name, definition.Descriptor)
		}
		if !definition.Descriptor.SupportsIdempotency {
			t.Fatalf("%s must declare idempotency", definition.Descriptor.Name)
		}
	}
}

func TestPaymentScheduleFileToolRunsThroughRuntimeAsReviewableDraft(t *testing.T) {
	text, _, llmR, _ := loadCorr2(t, "payment-full")
	agent := newIntakeAgent(t, llmR, map[string][]byte{"rent-schedule.pdf": []byte(text)})

	definition := agent.fileParseDefinitions()[2]
	registry := agenttools.NewRegistry()
	if err := registry.Register(definition); err != nil {
		t.Fatalf("register file tool: %v", err)
	}
	runtime := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", Permissions: []string{"ai_chat:use"},
			Scope: access.Scope{LegalEntityID: "le-001"},
		},
		RunID: "run-1",
	})

	result, err := runtime.Execute(ctx, agenttools.ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name,
		ToolVersion:    definition.Descriptor.Version,
		Arguments:      []byte(`{"file_id":"file-001","object_name":"rent-schedule.pdf","content_type":"application/pdf","contract_id":"contract-001"}`),
		IdempotencyKey: "lease.file.parse_payment_schedule:file-001:rent-schedule.pdf:contract-001",
	})
	if err != nil || result.Status != agenttools.StatusNeedsReview {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	parsed, ok := result.Data.(*PaymentScheduleParseResult)
	if !ok || parsed == nil || parsed.Summary == nil || !parsed.Summary.RequiresHumanConfirm {
		t.Fatalf("unexpected parsed result=%#v", result.Data)
	}
	if !result.Review.Required {
		t.Fatalf("review gate must be required: %#v", result.Review)
	}
}
func TestFileParseToolRejectsURLObjectNameBeforeCallingAIService(t *testing.T) {
	agent := &Agent{}
	definition := agent.fileParseDefinitions()[0]
	result, err := definition.Handler(context.Background(), agenttools.ToolCall{
		CallID: "call-1", Arguments: []byte(`{"file_id":"file-001","object_name":"https://evil.example/file.pdf","content_type":"application/pdf"}`),
	})
	if err != nil || result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorInvalidArguments {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestLegacyAIChatFileRouteKeepsDraftResponseAfterRuntimeMigration(t *testing.T) {
	text, ct, llmR, _ := loadCorr2(t, "payment-full")
	// The multi-round file pipeline (P0-A③) makes its own LLM calls: the
	// first round(s) carry the parse tool schemas and must select
	// lease.file.parse_payment_schedule; any later round that also carries
	// schemas is treated as terminal-answer because the scripted client keeps
	// answering with tool calls otherwise (the 4-round budget would stall).
	// The final main chat round carries no tools and returns the recorded
	// payment response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(body), `"tools"`) {
			var req struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			_ = json.Unmarshal(body, &req)
			sawToolResult := false
			for _, m := range req.Messages {
				if m.Role == "tool" {
					sawToolResult = true
				}
			}
			if sawToolResult {
				_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"租金表解析完成"}}],"usage":{"prompt_tokens":10,"completion_tokens":1,"total_tokens":11}}`))
				return
			}
			_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lease.file.parse_payment_schedule","arguments":"{\"file_id\":\"file-001\",\"object_name\":\"rent-schedule.pdf\",\"content_type\":\"application/pdf\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":` + mustJSONString(llmR) + `}}]}`))
	}))
	t.Cleanup(server.Close)
	client, err := llm.NewClient(llm.Config{
		Provider: "deepseek", APIKey: "test-key", BaseURL: server.URL, Model: "deepseek-v4-flash",
		PricingVersion: "unconfigured",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	agent := NewAgent(AgentPorts{})
	agent.SetLLMClient(client)
	agent.SetDocumentParser(textDocParser{})
	agent.SetFileBytesReader(func(_ context.Context, objectName string) ([]byte, error) {
		if objectName == "rent-schedule.pdf" {
			return []byte(text), nil
		}
		return nil, os.ErrNotExist
	})

	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "user-1", Role: "editor", Permissions: []string{"ai_chat:use"}},
		RunID:     "run-compat",
	})
	ctx = withAIServiceAuth(ctx, "Bearer test-token")
	response, err := agent.executeChatRequest(ctx, "Bearer test-token", Request{
		Message: "请解析这张租金表", FileID: "file-001", ObjectName: "rent-schedule.pdf",
		ContentType: ct,
	}, "le-001", "user-1", "editor", nil, nil, agent.toolRuntime)
	if err != nil {
		t.Fatalf("execute legacy AI Chat file route: %v", err)
	}
	// P0-A③: the multi-round file pipeline owns tool selection now; the
	// response Model names the loop ("lease-agent") instead of the last
	// single-round provider name. Everything the contract cares about —
	// AgentMode, two draft schedules, review summary — must be unchanged.
	if !response.AgentMode || response.Model != "lease-agent" || len(response.DraftPaymentSchedules) != 2 {
		t.Fatalf("compat response model=%q agentMode=%v schedules=%d", response.Model, response.AgentMode, len(response.DraftPaymentSchedules))
	}
	if response.PaymentScheduleSummary == nil || !response.PaymentScheduleSummary.RequiresHumanConfirm {
		t.Fatalf("compat review summary = %#v", response.PaymentScheduleSummary)
	}
}
