package aiagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
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
	fixturePath := filepath.Join("..", "..", "..", "contracts", "ai-intake.v1", "payment-schedule.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read shared AI intake fixture: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/parse/payment-schedule" {
			t.Errorf("unexpected AI service path %q", request.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_SERVICE_URL", server.URL)

	agent := &Agent{}
	definition := agent.fileParseDefinitions()[2]
	registry := agenttools.NewRegistry()
	if err := registry.Register(definition); err != nil {
		t.Fatalf("register file tool: %v", err)
	}
	runtime := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	ctx := withAIServiceAuth(context.Background(), "Bearer test-token")
	ctx = agenttools.WithExecutionContext(ctx, agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID:      "user-1",
			Permissions: []string{"ai_chat:use"},
			Scope:       access.Scope{LegalEntityID: "le-001"},
		},
		RunID: "run-1",
	})

	result, err := runtime.Execute(ctx, agenttools.ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name,
		ToolVersion:    definition.Descriptor.Version,
		Arguments:      []byte(`{"file_id":"file-001","object_name":"rent-schedule.xlsx","content_type":"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet","contract_id":"contract-001"}`),
		IdempotencyKey: "lease.file.parse_payment_schedule:file-001:rent-schedule.xlsx:contract-001",
	})
	if err != nil || result.Status != agenttools.StatusNeedsReview {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	parsed, ok := result.Data.(*PaymentScheduleParseResult)
	if !ok || parsed == nil || parsed.Summary == nil || !parsed.Summary.RequiresHumanConfirm {
		t.Fatalf("unexpected parsed result=%#v", result.Data)
	}
	if !result.Review.Required || len(result.Sources) != 1 || result.Sources[0].ID != "file-001" {
		t.Fatalf("review/evidence=%#v", result)
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
	fixturePath := filepath.Join("..", "..", "..", "contracts", "ai-intake.v1", "payment-schedule.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read shared AI intake fixture: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/chat/completions":
			// W4-2: the LLM call is now in-process through internal/llm and
			// hits the provider chat-completions endpoint, not /api/v1/chat.
			_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"parse_payment_schedule","arguments":"{}"}}]}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		case "/api/v1/parse/payment-schedule":
			_, _ = w.Write(fixture)
		default:
			t.Errorf("unexpected AI service path %q", request.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_SERVICE_URL", server.URL)
	t.Setenv("LLM_PROVIDER", "deepseek")
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	t.Setenv("DEEPSEEK_BASE_URL", server.URL)
	t.Setenv("DEEPSEEK_MODEL", "compat-model")

	agent := NewWithOperationalReadersAndGovernanceAndRetail(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "user-1", Role: "editor", Permissions: []string{"ai_chat:use"}},
		RunID:     "run-compat",
	})
	ctx = withAIServiceAuth(ctx, "Bearer test-token")
	response, err := agent.executeChatRequest(ctx, "Bearer test-token", Request{
		Message: "请解析这张租金表", FileID: "file-001", ObjectName: "rent-schedule.xlsx",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}, "le-001", "user-1", "editor", nil, nil, agent.toolRuntime)
	if err != nil {
		t.Fatalf("execute legacy AI Chat file route: %v", err)
	}
	if !response.AgentMode || response.Model != "deepseek/compat-model" || len(response.DraftPaymentSchedules) != 1 {
		t.Fatalf("compat response = %#v", response)
	}
	if response.PaymentScheduleSummary == nil || !response.PaymentScheduleSummary.RequiresHumanConfirm {
		t.Fatalf("compat review summary = %#v", response.PaymentScheduleSummary)
	}
}
