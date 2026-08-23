package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lease-management-system/core-service/internal/agentrunner"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// W4-3 vertical slice: the whole agent-runner flow now runs the planner
// in-process through internal/llm. The HTTP gateway mock stays exactly as it
// was; the planner hop (/api/v1/agent/plan) is gone and the model response
// arrives from the LLM chat-completions endpoint instead.
func TestRunCompletesGatewayVerticalSliceWithInProcessPlanner(t *testing.T) {
	var mu sync.Mutex
	var eventTypes []string
	var usageEvent bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/sessions":
			_, _ = writer.Write([]byte(`{"session":{"id":"session-1"}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/runs":
			_, _ = writer.Write([]byte(`{"run":{"id":"run-1","session_id":"session-1","status":"queued"}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/agent/tools":
			_, _ = writer.Write([]byte(`{"tools":[{"name":"lease.contract.get","version":"v1","description":"read contract","level":"read","read_only":true,"permissions":[{"resource":"contracts","action":"read"}]}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/capabilities":
			_, _ = writer.Write([]byte(`{"capability_token":"cap-1"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/chat/completions":
			// The in-process LLM planner response: a JSON-object plan naming
			// only the discovered tool.
			_, _ = writer.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"{\"tool_calls\":[{\"call_id\":\"call-1\",\"tool_name\":\"lease.contract.get\",\"tool_version\":\"v1\",\"arguments\":{\"contract_id\":\"contract-1\"}}]}"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost_usd":0.000003,"cost_currency":"USD"}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/tools/execute":
			_, _ = writer.Write([]byte(`{"call_id":"call-1","status":"completed","review":{"required":false},"data":{"contract_id":"contract-1"}}`))
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/stream"):
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/capabilities/revoke"):
			writer.WriteHeader(http.StatusAccepted)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/events"):
			var body struct {
				EventType string          `json:"type"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode event: %v", err)
			}
			mu.Lock()
			eventTypes = append(eventTypes, body.EventType)
			if body.EventType == "planner_usage" {
				var usage map[string]any
				if json.Unmarshal(body.Payload, &usage) == nil && usage["pricing_version"] == "test-v1" {
					usageEvent = true
				}
			}
			mu.Unlock()
			writer.WriteHeader(http.StatusAccepted)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/checkpoint"):
			writer.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	t.Setenv("LLM_PROVIDER", "deepseek")
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	t.Setenv("DEEPSEEK_BASE_URL", server.URL)
	t.Setenv("DEEPSEEK_MODEL", "deepseek-v4-flash")
	t.Setenv("LLM_PRICING_VERSION", "test-v1")
	t.Setenv("LLM_INPUT_PRICE_USD_PER_MILLION", "0.5")
	t.Setenv("LLM_OUTPUT_PRICE_USD_PER_MILLION", "0.5")

	err := run([]string{
		"--base-url", server.URL,
		"--token", "jwt-token",
		"--message", "读取合同",
		"--skill", "contract_review",
		"--skill-version", "v1",
		"--deadline", "2s",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !usageEvent {
		t.Fatalf("planner usage was not persisted; events=%v", eventTypes)
	}
	if len(eventTypes) == 0 {
		t.Fatal("expected durable run events")
	}
}

// The deterministic --plan path must still work without any API key (CI and
// air-gapped deployments).
func TestRunWithStaticPlanDoesNotRequireAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/sessions":
			_, _ = writer.Write([]byte(`{"session":{"id":"session-1"}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/runs":
			_, _ = writer.Write([]byte(`{"run":{"id":"run-1","session_id":"session-1","status":"queued"}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/agent/tools":
			_, _ = writer.Write([]byte(`{"tools":[{"name":"lease.contract.get","version":"v1","description":"read contract","level":"read","read_only":true,"permissions":[{"resource":"contracts","action":"read"}]}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/capabilities":
			_, _ = writer.Write([]byte(`{"capability_token":"cap-1"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/agent/tools/execute":
			_, _ = writer.Write([]byte(`{"call_id":"call-1","status":"completed","review":{"required":false},"data":{"contract_id":"contract-1"}}`))
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/stream"):
			writer.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/capabilities/revoke"):
			writer.WriteHeader(http.StatusAccepted)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/events"):
			writer.WriteHeader(http.StatusAccepted)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/checkpoint"):
			writer.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	plan, _ := json.Marshal([]map[string]string{{"tool_name": "lease.contract.get", "tool_version": "v1"}})
	planFile := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planFile, plan, 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	err := run([]string{
		"--base-url", server.URL,
		"--token", "jwt-token",
		"--message", "读取合同",
		"--plan", planFile,
		"--deadline", "2s",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestNewPlannerFailsClosedWithoutPlanOrKey(t *testing.T) {
	// With no API key configured and no static plan, the in-process planner
	// must fail at plan time — never silently fall back to an empty plan.
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("DEEPSEEK_API_KEY", "")
	planner, err := newPlanner(nil)
	if err != nil {
		t.Fatalf("construction: %v", err)
	}
	_, err = planner.Plan(t.Context(), agentrunner.PlanRequest{
		RunID: "run-1", Message: "读取合同",
		Tools: []agenttools.ToolDescriptor{{Name: "lease.contract.get", Version: "v1", Description: "read", Level: agenttools.LevelRead, ReadOnly: true}},
	})
	if err == nil {
		t.Fatal("planning without API key must fail closed")
	}
}
